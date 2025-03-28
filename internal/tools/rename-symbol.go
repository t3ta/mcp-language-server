package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath" // Import filepath for Abs
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
	// "github.com/isaacphi/mcp-language-server/internal/utilities" // Removed unused import
)

// RenameSymbolTool defines the MCP tool for renaming symbols using LSP.
type RenameSymbolTool struct {
	Client *lsp.Client // Assuming Client is accessible or passed appropriately
}

// RenameSymbolArgs defines the arguments for the rename_symbol tool.
type RenameSymbolArgs struct {
	FilePath  string `json:"filePath"`  // Required: Path to the file containing the symbol.
	Line      int    `json:"line"`      // Required: 0-based line number of the symbol.
	Character int    `json:"character"` // Required: 0-based character offset of the symbol.
	NewName   string `json:"newName"`   // Required: The new name for the symbol.
}

// RenameSymbolResult defines the result structure (delegating to apply_text_edit format).
type RenameSymbolResult struct {
	Changes map[string][]protocol.TextEdit `json:"changes"`
}


// Name returns the name of the tool.
func (t *RenameSymbolTool) Name() string {
	return "rename_symbol"
}

// Description returns the description of the tool.
func (t *RenameSymbolTool) Description() string {
	return "Renames a symbol across the workspace using the Language Server Protocol."
}

// Schema returns the JSON schema for the tool's arguments and result.
func (t *RenameSymbolTool) Schema() string {
	argsSchema := `{
		"type": "object",
		"properties": {
			"filePath": {"type": "string", "description": "Path to the file containing the symbol."},
			"line": {"type": "integer", "description": "0-based line number of the symbol."},
			"character": {"type": "integer", "description": "0-based character offset of the symbol."},
			"newName": {"type": "string", "description": "The new name for the symbol."}
		},
		"required": ["filePath", "line", "character", "newName"]
	}`
	resultSchema := `{
		"type": "object",
		"properties": {
			"changes": {
				"type": "object",
				"additionalProperties": {
					"type": "array",
					"items": {
						"type": "object",
						"properties": {
							"range": {
								"type": "object",
								"properties": {
									"start": {"$ref": "#/definitions/position"},
									"end": {"$ref": "#/definitions/position"}
								},
								"required": ["start", "end"]
							},
							"newText": {"type": "string"}
						},
						"required": ["range", "newText"]
					}
				}
			}
		},
		"definitions": {
			"position": {
				"type": "object",
				"properties": {
					"line": {"type": "integer"},
					"character": {"type": "integer"}
				},
				"required": ["line", "character"]
			}
		}
	}`
	return fmt.Sprintf(`{"name": "%s", "description": "%s", "input_schema": %s, "output_schema": %s}`, t.Name(), t.Description(), argsSchema, resultSchema)
}

// Execute performs the rename operation.
func (t *RenameSymbolTool) Execute(ctx context.Context, argsJSON json.RawMessage) (json.RawMessage, error) {
	var args RenameSymbolArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Corrected: Use filepath.Abs to ensure absolute path
	absPath, err := filepath.Abs(args.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for %s: %w", args.FilePath, err)
	}

	// Ensure the file is open in the LSP client
	if !t.Client.IsFileOpen(absPath) {
		if err := t.Client.OpenFile(ctx, absPath); err != nil {
			// Log warning but continue, server might handle it
			fmt.Fprintf(os.Stderr, "Warning: failed to open file %s before rename: %v\n", absPath, err)
		}
	}


	params := protocol.RenameParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: protocol.DocumentUri("file://" + absPath),
		},
		Position: protocol.Position{
			Line:      uint32(args.Line),
			Character: uint32(args.Character),
		},
		NewName: args.NewName,
	}

	workspaceEdit, err := t.Client.RequestRename(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("LSP rename request failed: %w", err)
	}

	if workspaceEdit == nil {
		result := RenameSymbolResult{Changes: make(map[string][]protocol.TextEdit)}
		return json.Marshal(result)
	}

	// Convert WorkspaceEdit.Changes (map[protocol.DocumentUri][]protocol.TextEdit)
	convertedChanges := make(map[string][]protocol.TextEdit)
	if workspaceEdit.Changes != nil {
		for uri, edits := range workspaceEdit.Changes {
			// Use TrimPrefix to convert file URI to path for the map key
			filePathKey := strings.TrimPrefix(string(uri), "file://")
			convertedChanges[filePathKey] = edits
		}
	}


	// Handle DocumentChanges as well
	if len(workspaceEdit.DocumentChanges) > 0 {
		for _, docChange := range workspaceEdit.DocumentChanges {
			// Corrected: Check if TextDocumentEdit field is not nil
			if docChange.TextDocumentEdit != nil {
				textDocEdit := docChange.TextDocumentEdit
				// Corrected: Cast protocol.DocumentUri to string and remove "file://" prefix
				filePath := strings.TrimPrefix(string(textDocEdit.TextDocument.URI), "file://")

				// Convert edits from Or_TextDocumentEdit_edits_Elem to protocol.TextEdit
				actualEdits := make([]protocol.TextEdit, 0, len(textDocEdit.Edits))
				for _, editUnion := range textDocEdit.Edits {
					// Corrected: Use AsTextEdit method from the union type
					if te, err := editUnion.AsTextEdit(); err == nil {
						actualEdits = append(actualEdits, te)
					} else {
						// Handle AnnotatedTextEdit or SnippetTextEdit if necessary, or log warning
						fmt.Fprintf(os.Stderr, "Warning: Skipping non-plain TextEdit in rename result: %v\n", err)
					}
				}

				// Append edits, potentially overwriting if the same file exists in Changes map
				convertedChanges[filePath] = append(convertedChanges[filePath], actualEdits...)
			} else {
				// Handle other types like CreateFile, RenameFile, DeleteFile if necessary
				// For rename, we only care about TextDocumentEdit
				// Use specific fields like CreateFile, RenameFile, DeleteFile to check type
				kind := ""
				if docChange.CreateFile != nil { kind = "CreateFile" }
				if docChange.RenameFile != nil { kind = "RenameFile" }
				if docChange.DeleteFile != nil { kind = "DeleteFile" }
				if kind != "" {
					fmt.Fprintf(os.Stderr, "Warning: Unsupported document change type '%s' received in rename result.\n", kind)
				} else {
					fmt.Fprintf(os.Stderr, "Warning: Unknown document change type received in rename result.\n")
				}
			}
		}
	}

	result := RenameSymbolResult{
		Changes: convertedChanges,
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal rename result: %w", err)
	}

	return resultJSON, nil
}

// Removed the compile-time check as Tool interface might not be explicitly defined/needed in this structure
// var _ Tool = (*RenameSymbolTool)(nil)
