package main

import (
	"fmt"
	"path/filepath" // For extension checking

	"github.com/isaacphi/mcp-language-server/internal/lsp"                 // For lsp.Client type
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
	FilePath string                   `json:"filePath"`
	Edits    []internalTools.TextEdit `json:"edits"` // Use internalTools alias
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

func (s *server) registerTools() error {

	// Register apply_text_edit tool
	err := s.mcpServer.RegisterTool(
		"apply_text_edit",
		"Apply multiple text edits to a specified file. Takes `filePath` (string) and `edits` (array of TextEdit objects with `type`, `startLine`, `endLine`, `newText`). Returns a confirmation message or error.",
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
		"Read the source code definition of a symbol. Takes `symbolName` (string), `language` (string), and optional `showLineNumbers` (bool). Returns the definition's source code with location info.",
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
		"Find all usages and references of a symbol. Takes `symbolName` (string), `language` (string), and optional `showLineNumbers` (bool). Returns a list of locations and code snippets.",
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
		"Get diagnostic information (errors, warnings) for a specific file. Takes `filePath` (string), optional `includeContext` (bool), and optional `showLineNumbers` (bool). Returns formatted diagnostics.",
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
		"Get code lens hints (e.g., run test, references) for a file. Takes `filePath` (string). Returns a list of code lenses with their locations and associated commands.",
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
		"Execute a specific code lens command identified by its index. Takes `filePath` (string) and `index` (int, 1-based from get_codelens). Returns a confirmation message or error.",
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

	return nil
}
