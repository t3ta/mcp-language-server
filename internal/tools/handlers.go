package tools

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

func GetDefinition(ctx context.Context, client *lsp.Client, symbolName string) (string, error) {
	symbolResult, err := client.Symbol(ctx, protocol.WorkspaceSymbolParams{
		Query: symbolName,
	})
	if err != nil {
		return "", fmt.Errorf("Failed to fetch symbol: %v", err)
	}

	results, err := symbolResult.Results()
	if err != nil {
		return "", fmt.Errorf("Failed to parse results: %v", err)
	}

	var definitions []string
	for _, symbol := range results {
		if symbol.GetName() != symbolName {
			continue
		}

		log.Printf("Symbol: %s\n", symbol.GetName())
		loc := symbol.GetLocation()

		banner := strings.Repeat("=", 80) + "\n"
		definition, loc, err := GetFullDefinition(ctx, client, loc)
		locationInfo := fmt.Sprintf(
			"Symbol: %s\n"+
				"File: %s\n"+
				"Start Position: Line %d, Column %d\n"+
				"End Position: Line %d, Column %d\n"+
				"%s\n",
			symbolName,
			strings.TrimPrefix(string(loc.URI), "file://"),
			loc.Range.Start.Line+1,
			loc.Range.Start.Character+1,
			loc.Range.End.Line+1,
			loc.Range.End.Character+1,
			strings.Repeat("=", 80))

		if err != nil {
			log.Printf("Error getting definition: %v\n", err)
			continue
		}

		definitions = append(definitions, banner+locationInfo+definition+"\n")
	}

	if len(definitions) == 0 {
		return fmt.Sprintf("%s not found", symbolName), nil
	}

	return strings.Join(definitions, "\n"), nil
}
