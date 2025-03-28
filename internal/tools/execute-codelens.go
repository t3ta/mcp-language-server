package tools

import (
	"context"
	"fmt"
	"time"
	"path/filepath" // Added for absolute path conversion

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// ExecuteCodeLens executes a specific code lens command from a file.
func ExecuteCodeLens(ctx context.Context, client *lsp.Client, filePath string, index int) (string, error) {
	// Ensure filePath is absolute
	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("could not get absolute path for '%s': %w", filePath, err)
	}
	filePath = absFilePath // Use absolute path from now on

	// Open the file
	err = client.OpenFile(ctx, filePath) // Use absolute path
	if err != nil {
		return "", fmt.Errorf("could not open file '%s': %w", filePath, err)
	}
	// TODO: find a more appropriate way to wait
	time.Sleep(time.Second)

	// Get code lenses
	docIdentifier := protocol.TextDocumentIdentifier{
		URI: protocol.DocumentUri("file://" + filePath),
	}

	params := protocol.CodeLensParams{
		TextDocument: docIdentifier,
	}
	codeLenses, err := client.CodeLens(ctx, params)
	if err != nil {
		return "", fmt.Errorf("Failed to get code lenses: %v", err)
	}

	if len(codeLenses) == 0 {
		return "", fmt.Errorf("No code lenses found in file")
	}

	if index < 1 || index > len(codeLenses) {
		return "", fmt.Errorf("Invalid code lens index: %d. Available range: 1-%d", index, len(codeLenses))
	}

	lens := codeLenses[index-1]

	// Resolve the code lens if it doesn't have a command
	if lens.Command == nil {
		resolvedLens, err := client.ResolveCodeLens(ctx, lens)
		if err != nil {
			return "", fmt.Errorf("Failed to resolve code lens: %v", err)
		}
		lens = resolvedLens
	}

	if lens.Command == nil {
		return "", fmt.Errorf("Code lens has no command after resolution")
	}

	// Execute the command
	_, err = client.ExecuteCommand(ctx, protocol.ExecuteCommandParams{
		Command:   lens.Command.Command,
		Arguments: lens.Command.Arguments,
	})
	if err != nil {
		return "", fmt.Errorf("Failed to execute code lens command: %v", err)
	}

	return fmt.Sprintf("Successfully executed code lens command: %s", lens.Command.Title), nil
}
