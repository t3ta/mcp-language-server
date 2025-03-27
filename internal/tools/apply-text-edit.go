package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"regexp" // Added for regex support
	"sort"
	"strings"
	"path/filepath" // Added for absolute path conversion

	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/isaacphi/mcp-language-server/internal/utilities"
)

// FileOpener defines the interface required for opening files, typically implemented by lsp.Client.
type FileOpener interface {
	OpenFile(ctx context.Context, filePath string) error
}

type TextEditType string

const (
	Replace TextEditType = "replace"
	Insert  TextEditType = "insert"
	Delete  TextEditType = "delete"
)

type TextEdit struct {
	Type         TextEditType `json:"type" jsonschema:"required,enum=replace|insert|delete,description=Type of edit operation (replace, insert, delete)"`
	StartLine    int          `json:"startLine" jsonschema:"required,description=Start line of the range, inclusive"`
	EndLine      int          `json:"endLine" jsonschema:"required,description=End line of the range, inclusive"`
	NewText      string       `json:"newText,omitempty" jsonschema:"description=Replacement text for non-regex replace/insert. Leave blank for delete."`
	IsRegex      bool         `json:"isRegex,omitempty" jsonschema:"description=Whether to treat pattern as regex"`
	RegexPattern string       `json:"regexPattern,omitempty" jsonschema:"description=Regex pattern to search for within the range (if isRegex is true)"`
	RegexReplace string       `json:"regexReplace,omitempty" jsonschema:"description=Replacement string, supporting capture groups like $1 (if isRegex is true)"`
}

// ApplyTextEdits applies a series of text edits to a file.
// It now accepts a FileOpener interface instead of a concrete *lsp.Client.
func ApplyTextEdits(ctx context.Context, opener FileOpener, filePath string, edits []TextEdit) (string, error) {
	// Ensure filePath is absolute
	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("could not get absolute path for '%s': %w", filePath, err)
	}
	filePath = absFilePath // Use absolute path from now on

	err = opener.OpenFile(ctx, filePath) // Use the opener interface
	if err != nil {
		return "", fmt.Errorf("could not open file '%s': %w", filePath, err)
	}

	// Sort edits by line number in descending order to process from bottom to top
	// This way line numbers don't shift under us as we make edits
	sort.Slice(edits, func(i, j int) bool {
		return edits[i].StartLine > edits[j].StartLine
	})

	// Convert from input format to protocol.TextEdit
	var textEdits []protocol.TextEdit
	for _, edit := range edits {
		rng, err := getRange(edit.StartLine, edit.EndLine, filePath)
		if err != nil {
			return "", fmt.Errorf("invalid position: %v", err)
		}

		// Handle Regex Replace first
		if edit.IsRegex && edit.Type == Replace {
			if edit.RegexPattern == "" {
				return "", fmt.Errorf("regex pattern cannot be empty when isRegex is true for edit starting at line %d", edit.StartLine)
			}

			// Read file content to get the text within the range
			contentBytes, err := os.ReadFile(filePath)
			if err != nil {
				// Note: This reads the file potentially multiple times in a loop if many regex edits exist.
				// Could be optimized by reading once before the loop if performance becomes an issue.
				return "", fmt.Errorf("failed to read file for regex replace: %w", err)
			}
			contentStr := string(contentBytes)

			// Determine line endings (could also be optimized)
			lineEnding := "\n"
			if strings.Contains(contentStr, "\r\n") {
				lineEnding = "\r\n"
			}
			lines := strings.Split(contentStr, lineEnding)

			// Get 0-based indices, clamping to valid range
			startIdx := edit.StartLine - 1
			endIdx := edit.EndLine - 1
			if startIdx < 0 { startIdx = 0 } // Ensure start is not negative
			if endIdx >= len(lines) { endIdx = len(lines) - 1 } // Ensure end does not exceed lines available
			if startIdx > endIdx {
				// If start is beyond end after clamping, it implies an invalid input range or targeting EOF in a weird way.
				// For regex replace, we need a valid content range.
				return "", fmt.Errorf("invalid range for regex replace: start line %d > end line %d (after bounds check)", edit.StartLine, edit.EndLine)
			}

			// Extract the content within the specified lines
			contentInRange := strings.Join(lines[startIdx:endIdx+1], lineEnding)

			// Compile the regex
			re, err := regexp.Compile(edit.RegexPattern)
			if err != nil {
				return "", fmt.Errorf("invalid regex pattern '%s' for edit starting at line %d: %w", edit.RegexPattern, edit.StartLine, err)
			}

			// Perform the replacement within the extracted content
			replacedContent := re.ReplaceAllString(contentInRange, edit.RegexReplace)

			// Create a single edit replacing the original range with the new content
			textEdits = append(textEdits, protocol.TextEdit{
				Range:   rng, // Use the range covering the original lines
				NewText: replacedContent,
			})
			continue // Skip the normal switch statement below
		}

		// Handle non-regex edits (Insert, Delete, simple Replace)
		var currentEdit protocol.TextEdit
		switch edit.Type {
		case Insert:
			rng.End = rng.Start // Make it a zero-width range at the start position
			currentEdit = protocol.TextEdit{
				Range:   rng,
				NewText: edit.NewText,
			}
		case Delete:
			currentEdit = protocol.TextEdit{
				Range:   rng,
				NewText: "", // Ensure NewText is empty for delete
			}
		case Replace: // Non-regex Replace
			currentEdit = protocol.TextEdit{
				Range:   rng,
				NewText: edit.NewText, // Use the full range and NewText as-is
			}
		default:
			// Should not happen if JSON schema validation works, but good to have
			return "", fmt.Errorf("unknown edit type '%s' for edit starting at line %d", edit.Type, edit.StartLine)
		}
		textEdits = append(textEdits, currentEdit)
	}

	edit := protocol.WorkspaceEdit{
		Changes: map[protocol.DocumentUri][]protocol.TextEdit{
			protocol.DocumentUri(filePath): textEdits,
		},
	}

	if err := utilities.ApplyWorkspaceEdit(edit); err != nil {
		return "", fmt.Errorf("failed to apply text edits: %v", err)
	}

	return "Successfully applied text edits.\nWARNING: line numbers may have changed. Re-read code before applying additional edits.", nil
}

// getRange now handles EOF insertions and is more precise about character positions
func getRange(startLine, endLine int, filePath string) (protocol.Range, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return protocol.Range{}, fmt.Errorf("failed to read file: %w", err)
	}

	// Detect line ending style
	var lineEnding string
	if bytes.Contains(content, []byte("\r\n")) {
		lineEnding = "\r\n"
	} else {
		lineEnding = "\n"
	}

	// Split lines without the line endings
	lines := strings.Split(string(content), lineEnding)

	// Handle start line positioning
	if startLine < 1 {
		return protocol.Range{}, fmt.Errorf("start line must be >= 1, got %d", startLine)
	}

	// Convert to 0-based line numbers
	startIdx := startLine - 1
	endIdx := endLine - 1

	// Handle EOF positioning
	if startIdx >= len(lines) {
		// For EOF, we want to point to the end of the last content-bearing line
		lastContentLineIdx := len(lines) - 1
		if lastContentLineIdx >= 0 && lines[lastContentLineIdx] == "" {
			lastContentLineIdx--
		}

		if lastContentLineIdx < 0 {
			lastContentLineIdx = 0
		}

		pos := protocol.Position{
			Line:      uint32(lastContentLineIdx),
			Character: uint32(len(lines[lastContentLineIdx])),
		}

		return protocol.Range{
			Start: pos,
			End:   pos,
		}, nil
	}

	// Normal range handling
	if endIdx >= len(lines) {
		endIdx = len(lines) - 1
	}

	return protocol.Range{
		Start: protocol.Position{
			Line:      uint32(startIdx),
			Character: 0,
		},
		End: protocol.Position{
			Line:      uint32(endIdx),
			Character: uint32(len(lines[endIdx])),
		},
	}, nil
}
