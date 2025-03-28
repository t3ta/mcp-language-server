package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// FindSymbolsTool defines the MCP tool for finding symbols using LSP.
type FindSymbolsTool struct {
	Client *lsp.Client // Assuming Client is accessible or passed appropriately
}

// FindSymbolsArgs defines the arguments for the find_symbols tool.
type FindSymbolsArgs struct {
	Query           string `json:"query"`                      // Required: Search query string.
	Scope           string `json:"scope"`                      // Required: "workspace" or "document".
	FilePath        string `json:"filePath,omitempty"`         // Optional: Required if scope is "document".
	ShowLineNumbers bool   `json:"showLineNumbers,omitempty"` // Optional: Default true.
}

// FindSymbolsResult defines the result structure.
// Returning a simple string list for now. Could be JSON later.
type FindSymbolsResult struct {
	Symbols []string `json:"symbols"`
}

// Name returns the name of the tool.
func (t *FindSymbolsTool) Name() string {
	return "find_symbols"
}

// Description returns the description of the tool.
func (t *FindSymbolsTool) Description() string {
	return "Finds symbols in the workspace or a specific document using the Language Server Protocol."
}

// Schema returns the JSON schema for the tool's arguments and result.
func (t *FindSymbolsTool) Schema() string {
	argsSchema := `{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "Search query string."},
			"scope": {"type": "string", "enum": ["workspace", "document"], "description": "Search scope ('workspace' or 'document')."},
			"filePath": {"type": "string", "description": "Path to the file (required if scope is 'document')."},
			"showLineNumbers": {"type": "boolean", "default": true, "description": "Include line numbers in the result."}
		},
		"required": ["query", "scope"]
	}`
	resultSchema := `{
		"type": "object",
		"properties": {
			"symbols": {
				"type": "array",
				"items": {"type": "string"},
				"description": "List of found symbols with their locations."
			}
		}
	}`
	return fmt.Sprintf(`{"name": "%s", "description": "%s", "input_schema": %s, "output_schema": %s}`, t.Name(), t.Description(), argsSchema, resultSchema)
}

// Execute performs the symbol search operation.
func (t *FindSymbolsTool) Execute(ctx context.Context, argsJSON json.RawMessage) (json.RawMessage, error) {
	var args FindSymbolsArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Set default for showLineNumbers if not provided
	if args.ShowLineNumbers == false {
		// Check if it was explicitly set to false or just omitted (zero value)
		// A more robust way might involve using a pointer, but this works for now.
		var tempArgs map[string]any // Use any instead of interface{}
		if json.Unmarshal(argsJSON, &tempArgs) == nil {
			if _, ok := tempArgs["showLineNumbers"]; !ok {
				args.ShowLineNumbers = true // Default to true if omitted
			}
		} else {
			args.ShowLineNumbers = true // Default to true on unmarshal error too? Or handle error?
		}
	}


	var symbolsResult any // Use any instead of interface{}
	var err error

	switch args.Scope {
	case "workspace":
		// Workspace scope search
		params := protocol.WorkspaceSymbolParams{
			Query: args.Query,
		}
		// Note: RequestWorkspaceSymbols returns []protocol.SymbolInformation
		symbolsResult, err = t.Client.RequestWorkspaceSymbols(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("LSP workspace/symbol request failed: %w", err)
		}

	case "document":
		// Document scope search
		if args.FilePath == "" {
			return nil, fmt.Errorf("filePath is required for document scope search")
		}
		absPath, pathErr := filepath.Abs(args.FilePath)
		if pathErr != nil {
			return nil, fmt.Errorf("failed to get absolute path for %s: %w", args.FilePath, pathErr)
		}

		// Ensure file is open
		if !t.Client.IsFileOpen(absPath) {
			if openErr := t.Client.OpenFile(ctx, absPath); openErr != nil {
				// Log or handle error, maybe proceed cautiously
				fmt.Printf("Warning: failed to open file %s before document symbol search: %v\n", absPath, openErr)
			}
		}

		params := protocol.DocumentSymbolParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: protocol.DocumentUri("file://" + absPath),
			},
		}
		// Note: RequestDocumentSymbols returns interface{} which can be []protocol.DocumentSymbol or []protocol.SymbolInformation
		symbolsResult, err = t.Client.RequestDocumentSymbols(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("LSP textDocument/documentSymbol request failed: %w", err)
		}

	default:
		return nil, fmt.Errorf("invalid scope: %s. Must be 'workspace' or 'document'", args.Scope)
	}

	// Format the result
	formattedSymbols := formatSymbols(symbolsResult, args.ShowLineNumbers)

	result := FindSymbolsResult{Symbols: formattedSymbols}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal find_symbols result: %w", err)
	}

	return resultJSON, nil
}

// formatSymbols converts the LSP symbol result (either []DocumentSymbol or []SymbolInformation) into a string slice.
func formatSymbols(result any, showLineNumbers bool) []string { // Use any instead of interface{}
	var formatted []string

	switch symbols := result.(type) {
	case []protocol.DocumentSymbol:
		for _, symbol := range symbols {
			formatted = append(formatted, formatDocumentSymbol(symbol, "", showLineNumbers)...) // Start with empty prefix for top-level
		}
	case []protocol.SymbolInformation:
		for _, symbol := range symbols {
			formatted = append(formatted, formatSymbolInformation(symbol, showLineNumbers))
		}
	default:
		// Should not happen if LSP client returns correctly
		formatted = append(formatted, fmt.Sprintf("Error: Unexpected symbol result type %T", result))
	}

	return formatted
}

// formatDocumentSymbol recursively formats DocumentSymbol and its children.
func formatDocumentSymbol(symbol protocol.DocumentSymbol, prefix string, showLineNumbers bool) []string {
	var results []string
	line := ""
	if showLineNumbers {
		// Add 1 to line numbers because LSP uses 0-based lines
		line = fmt.Sprintf(" (L%d)", symbol.SelectionRange.Start.Line+1)
	}
	symbolKindStr := symbolKindToString(symbol.Kind) // Helper function needed
	results = append(results, fmt.Sprintf("%s%s: %s%s", prefix, symbolKindStr, symbol.Name, line))

	// Recursively format children with indentation
	childPrefix := prefix + "  "
	for _, child := range symbol.Children {
		results = append(results, formatDocumentSymbol(child, childPrefix, showLineNumbers)...)
	}
	return results
}

// formatSymbolInformation formats SymbolInformation.
func formatSymbolInformation(symbol protocol.SymbolInformation, showLineNumbers bool) string {
	line := ""
	filePath := strings.TrimPrefix(string(symbol.Location.URI), "file://")
	if showLineNumbers {
		// Add 1 to line numbers because LSP uses 0-based lines
		line = fmt.Sprintf(" (L%d)", symbol.Location.Range.Start.Line+1)
	}
	symbolKindStr := symbolKindToString(symbol.Kind) // Helper function needed
	container := ""
	if symbol.ContainerName != "" {
		container = fmt.Sprintf(" in %s", symbol.ContainerName)
	}
	return fmt.Sprintf("%s: %s%s%s - %s", symbolKindStr, symbol.Name, container, line, filePath)
}

// symbolKindToString converts protocol.SymbolKind to a readable string.
// This needs to be implemented based on the SymbolKind constants.
func symbolKindToString(kind protocol.SymbolKind) string {
	switch kind {
	case protocol.File: return "File"
	case protocol.Module: return "Module"
	case protocol.Namespace: return "Namespace"
	case protocol.Package: return "Package"
	case protocol.Class: return "Class"
	case protocol.Method: return "Method"
	case protocol.Property: return "Property"
	case protocol.Field: return "Field"
	case protocol.Constructor: return "Constructor"
	case protocol.Enum: return "Enum"
	case protocol.Interface: return "Interface"
	case protocol.Function: return "Function"
	case protocol.Variable: return "Variable"
	case protocol.Constant: return "Constant"
	case protocol.String: return "String"
	case protocol.Number: return "Number"
	case protocol.Boolean: return "Boolean"
	case protocol.Array: return "Array"
	case protocol.Object: return "Object"
	case protocol.Key: return "Key"
	case protocol.Null: return "Null"
	case protocol.EnumMember: return "EnumMember"
	case protocol.Struct: return "Struct"
	case protocol.Event: return "Event"
	case protocol.Operator: return "Operator"
	case protocol.TypeParameter: return "TypeParameter"
	default: return fmt.Sprintf("UnknownKind(%d)", kind)
	}
}
