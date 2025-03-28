package main

import (
	"encoding/json" // Import encoding/json
	"fmt"
	"path/filepath" // For extension checking

	"github.com/isaacphi/mcp-language-server/internal/lsp"    // For lsp.Client type
	internalTools "github.com/isaacphi/mcp-language-server/internal/tools" // Alias internal/tools to avoid name clash
	"github.com/metoro-io/mcp-golang"
)

// Helper function to get the appropriate LSP client based on file extension
func (s *server) getClientForFile(filePath string) (*lsp.Client, error) {
	ext := filepath.Ext(filePath)
	language, ok := s.extensionToLanguage[ext]
	if !ok {
		return nil, fmt.Errorf("language not supported for file extension: %s (file: %s)", ext, filePath)
	}

	client, ok := s.lspClients[language]
	if !ok {
		// This should ideally not happen if initialization succeeded for this language
		return nil, fmt.Errorf("LSP client for language '%s' not found or not initialized", language)
	}
	return client, nil
}

type ReadDefinitionArgs struct {
	SymbolName      string `json:"symbolName" jsonschema:"required,description=The name of the symbol whose definition you want to find (e.g. 'mypackage.MyFunction', 'MyType.MyMethod')"`
	ShowLineNumbers bool   `json:"showLineNumbers" jsonschema:"required,default=true,description=Include line numbers in the returned source code"`
	Language        string `json:"language" jsonschema:"required,description=The programming language of the symbol (e.g., 'typescript', 'go')"` // Added language argument
}

type FindReferencesArgs struct {
	SymbolName      string `json:"symbolName" jsonschema:"required,description=The name of the symbol to search for (e.g. 'mypackage.MyFunction', 'MyType')"`
	ShowLineNumbers bool   `json:"showLineNumbers" jsonschema:"required,default=true,description=Include line numbers when showing where the symbol is used"`
	Language        string `json:"language" jsonschema:"required,description=The programming language of the symbol (e.g., 'typescript', 'go')"` // Added language argument
}

type ApplyTextEditArgs struct {
	FilePath string                `json:"filePath" jsonschema:"required,description=The path to the file to apply edits to."` // Added description
	Edits    []internalTools.TextEdit `json:"edits" jsonschema:"required,description=An array of text edit objects defining the changes to apply.",items={
		"type": "object",
		"properties": {
			"type": {
				"type": "string",
				"enum": ["replace", "insert", "delete"],
				"description": "Type of edit operation (replace, insert, delete)"
			},
			"startLine": {
				"type": "integer",
				"description": "Start line of the range, inclusive"
			},
			"endLine": {
				"type": "integer",
				"description": "End line of the range, inclusive"
			},
			"newText": {
				"type": "string",
				"description": "Replacement text for non-regex replace/insert. Leave blank for delete."
			},
			"isRegex": {
				"type": "boolean",
				"description": "Whether to treat pattern as regex"
			},
			"regexPattern": {
				"type": "string",
				"description": "Regex pattern to search for within the range (if isRegex is true)"
			},
			"regexReplace": {
				"type": "string",
				"description": "Replacement string, supporting capture groups like $1 (if isRegex is true)"
			},
			"preserveBrackets": {
				"type": "boolean",
				"description": "If true, check and prevent edits that break bracket pairs"
			},
			"bracketTypes": {
				"type": "array",
				"items": { "type": "string" },
				"description": "Types of brackets to check (e.g., '()', '{}', '[]'). Defaults if empty."
			}
		},
		"required": ["type", "startLine", "endLine"]
	}` // Use internalTools alias, Inlined TextEdit schema description
}

type GetDiagnosticsArgs struct {
	FilePath        string `json:"filePath" jsonschema:"required,description=The path to the file to get diagnostics for"`
	IncludeContext  bool   `json:"includeContext" jsonschema:"default=false,description=Include additional context for each diagnostic. Prefer false."`
	ShowLineNumbers bool   `json:"showLineNumbers" jsonschema:"required,default=true,description=If true, adds line numbers to the output"`
}

type GetCodeLensArgs struct {
	FilePath string `json:"filePath" jsonschema:"required,description=The path to the file to get code lens information for"`
}

type ExecuteCodeLensArgs struct {
	FilePath string `json:"filePath" jsonschema:"required,description=The path to the file containing the code lens to execute"`
	Index    int    `json:"index" jsonschema:"required,description=The index of the code lens to execute (from get_codelens output), 1 indexed"`
}

// Define args struct for rename_symbol tool
type RenameSymbolArgs struct {
	FilePath  string `json:"filePath" jsonschema:"required,description=Path to the file containing the symbol."`
	Line      int    `json:"line" jsonschema:"required,description=0-based line number of the symbol."`
	Character int    `json:"character" jsonschema:"required,description=0-based character offset of the symbol."`
	NewName   string `json:"newName" jsonschema:"required,description=The new name for the symbol."`
}

// Define args struct for find_symbols tool
type FindSymbolsArgs struct {
	Query           string `json:"query" jsonschema:"required,description=Search query string."`
	Scope           string `json:"scope" jsonschema:"required,enum=[\"workspace\", \"document\"],description=Search scope ('workspace' or 'document')."`
	FilePath        string `json:"filePath,omitempty" jsonschema:"description=Path to the file (required if scope is 'document')."`
	ShowLineNumbers bool   `json:"showLineNumbers,omitempty" jsonschema:"default=true,description=Include line numbers in the result."`
}


func (s *server) registerTools() error {

	// Register apply_text_edit tool
	err := s.mcpServer.RegisterTool(
		"apply_text_edit",
		"Apply multiple text edits to a file specified by `filePath`. Each edit in the `edits` array defines the operation (`replace`, `insert`, `delete`), range (`startLine`, `endLine`), and optionally `newText` or regex patterns for advanced replacements.", // Even more detailed description! ✨
		func(args ApplyTextEditArgs) (*mcp_golang.ToolResponse, error) {
			// Get LSP client based on file extension
			client, err := s.getClientForFile(args.FilePath)
			if err != nil {
				return nil, err // Error includes context like "language not supported"
			}

			// Call the actual tool implementation with the selected client
			response, err := internalTools.ApplyTextEdits(s.ctx, client, args.FilePath, args.Edits) // Use internalTools alias
			if err != nil {
				return nil, fmt.Errorf("failed to apply edits: %v", err)
			}
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(response)), nil
		})
	if err != nil {
		return fmt.Errorf("failed to register tool: %v", err)
	}

	// Register read_definition tool
	err = s.mcpServer.RegisterTool(
		"read_definition",
		"Read the source code definition of a symbol (function, type, constant, etc.) specified by `symbolName` and `language`. Returns the complete implementation code where the symbol is defined.", // Updated description
		func(args ReadDefinitionArgs) (*mcp_golang.ToolResponse, error) {
			// Get LSP client based on language argument
			client, ok := s.lspClients[args.Language]
			if !ok {
				return nil, fmt.Errorf("LSP client for language '%s' not found or not initialized", args.Language)
			}

			// Call the actual tool implementation with the selected client
			text, err := internalTools.ReadDefinition(s.ctx, client, args.SymbolName, args.ShowLineNumbers) // Use internalTools alias
			if err != nil {
				return nil, fmt.Errorf("failed to get definition: %v", err)
			}
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(text)), nil
		})
	if err != nil {
		return fmt.Errorf("failed to register tool: %v", err)
	}

	// Register find_references tool
	err = s.mcpServer.RegisterTool(
		"find_references",
		"Find all usages and references of a symbol specified by `symbolName` and `language` throughout the codebase. Returns a list of all files and locations where the symbol appears.", // Updated description
		func(args FindReferencesArgs) (*mcp_golang.ToolResponse, error) {
			// Get LSP client based on language argument
			client, ok := s.lspClients[args.Language]
			if !ok {
				return nil, fmt.Errorf("LSP client for language '%s' not found or not initialized", args.Language)
			}

			// Call the actual tool implementation with the selected client
			text, err := internalTools.FindReferences(s.ctx, client, args.SymbolName, args.ShowLineNumbers) // Use internalTools alias
			if err != nil {
				return nil, fmt.Errorf("failed to find references: %v", err)
			}
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(text)), nil
		})
	if err != nil {
		return fmt.Errorf("failed to register tool: %v", err)
	}

	// Register get_diagnostics tool
	err = s.mcpServer.RegisterTool(
		"get_diagnostics",
		"Get diagnostic information (errors, warnings) for a specific file specified by `filePath` from the language server.", // Updated description
		func(args GetDiagnosticsArgs) (*mcp_golang.ToolResponse, error) {
			// Get LSP client based on file extension
			client, err := s.getClientForFile(args.FilePath)
			if err != nil {
				return nil, err
			}

			// Call the actual tool implementation with the selected client
			text, err := internalTools.GetDiagnosticsForFile(s.ctx, client, args.FilePath, args.IncludeContext, args.ShowLineNumbers) // Use internalTools alias
			if err != nil {
				return nil, fmt.Errorf("failed to get diagnostics: %v", err)
			}
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(text)), nil
		},
	)
	if err != nil {
		return fmt.Errorf("failed to register tool: %v", err)
	}

	// Register get_codelens tool
	err = s.mcpServer.RegisterTool(
		"get_codelens",
		"Get code lens hints (e.g., run test, references) for a given file specified by `filePath` from the language server.", // Updated description
		func(args GetCodeLensArgs) (*mcp_golang.ToolResponse, error) {
			// Get LSP client based on file extension
			client, err := s.getClientForFile(args.FilePath)
			if err != nil {
				return nil, err
			}

			// Call the actual tool implementation with the selected client
			text, err := internalTools.GetCodeLens(s.ctx, client, args.FilePath) // Use internalTools alias
			if err != nil {
				return nil, fmt.Errorf("failed to get code lens: %v", err)
			}
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(text)), nil
		},
	)
	if err != nil {
		return fmt.Errorf("failed to register tool: %v", err)
	}

	// Register execute_codelens tool
	err = s.mcpServer.RegisterTool(
		"execute_codelens",
		"Execute a code lens command (obtained from `get_codelens`) for a given file specified by `filePath` and the lens `index`.", // Updated description
		func(args ExecuteCodeLensArgs) (*mcp_golang.ToolResponse, error) {
			// Get LSP client based on file extension
			client, err := s.getClientForFile(args.FilePath)
			if err != nil {
				return nil, err
			}

			// Call the actual tool implementation with the selected client
			text, err := internalTools.ExecuteCodeLens(s.ctx, client, args.FilePath, args.Index) // Use internalTools alias
			if err != nil {
				return nil, fmt.Errorf("failed to execute code lens: %v", err)
			}
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(text)), nil
		},
	)
	if err != nil {
		return fmt.Errorf("failed to register tool: %v", err)
	}

	// Register rename_symbol tool
	err = s.mcpServer.RegisterTool(
		"rename_symbol",
		"Renames a symbol across the workspace using the Language Server Protocol.",
		func(args RenameSymbolArgs) (*mcp_golang.ToolResponse, error) {
			// Get LSP client based on file extension
			client, err := s.getClientForFile(args.FilePath)
			if err != nil {
				return nil, err
			}

			// Instantiate the tool struct (assuming it needs the client)
			renameTool := internalTools.RenameSymbolTool{Client: client}

			// Corrected: Marshal args to json.RawMessage before passing to Execute
			argsJSON, err := json.Marshal(args)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal rename args: %v", err)
			}


			// Execute the tool's logic
			resultJSON, err := renameTool.Execute(s.ctx, argsJSON) // Pass marshaled JSON
			if err != nil {
				return nil, fmt.Errorf("failed to execute rename symbol: %v", err)
			}

			// Corrected: Return result as text (string representation of the JSON)
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(string(resultJSON))), nil
		},
	)
	if err != nil {
		return fmt.Errorf("failed to register rename_symbol tool: %v", err)
	}

	// Register find_symbols tool
	err = s.mcpServer.RegisterTool(
		"find_symbols",
		"Finds symbols in the workspace or a specific document using the Language Server Protocol.",
		func(args FindSymbolsArgs) (*mcp_golang.ToolResponse, error) {
			var client *lsp.Client
			var err error

			// Determine client based on scope
			if args.Scope == "document" {
				if args.FilePath == "" {
					return nil, fmt.Errorf("filePath is required for document scope search")
				}
				client, err = s.getClientForFile(args.FilePath)
				if err != nil {
					return nil, err
				}
			} else if args.Scope == "workspace" {
				// For workspace scope, we might need a default client or iterate through all?
				// Let's assume a primary client exists or pick the first one for now.
				// This logic might need refinement based on how workspace symbols should work across languages.
				if len(s.lspClients) == 0 {
					return nil, fmt.Errorf("no LSP clients available for workspace symbol search")
				}
				for _, c := range s.lspClients { // Just pick the first one
					client = c
					break
				}
			} else {
				return nil, fmt.Errorf("invalid scope: %s. Must be 'workspace' or 'document'", args.Scope)
			}

			// Instantiate the tool struct
			findTool := internalTools.FindSymbolsTool{Client: client}

			// Marshal args to json.RawMessage
			argsJSON, err := json.Marshal(args)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal find_symbols args: %v", err)
			}

			// Execute the tool's logic
			resultJSON, err := findTool.Execute(s.ctx, argsJSON)
			if err != nil {
				return nil, fmt.Errorf("failed to execute find symbols: %v", err)
			}

			// Return result as text (string representation of the JSON containing the list)
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(string(resultJSON))), nil
		},
	)
	if err != nil {
		return fmt.Errorf("failed to register find_symbols tool: %v", err)
	}


	return nil
}
