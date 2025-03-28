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

// BracketGuardError represents an error when an edit violates bracket balancing rules.
type BracketGuardError struct {
	ViolationType string    `json:"violationType"` // e.g., "CrossingPair", "PartialPairStart", "PartialPairEnd"
	Message       string    `json:"message"`
	Suggestion    *TextEdit `json:"suggestion,omitempty"` // Optional suggestion for a safe edit
}

func (e *BracketGuardError) Error() string {
	return fmt.Sprintf("Bracket balance violation (%s): %s", e.ViolationType, e.Message)
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
	PreserveBrackets bool     `json:"preserveBrackets,omitempty" jsonschema:"description=If true, check and prevent edits that break bracket pairs"`
	BracketTypes []string     `json:"bracketTypes,omitempty" jsonschema:"description=Types of brackets to check (e.g., '()', '{}', '[]'). Defaults if empty."`
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
		// --- Parameter Conflict Check ---
		if edit.IsRegex && edit.NewText != "" {
			return "", fmt.Errorf("invalid edit parameters for line %d: cannot provide both IsRegex=true and non-empty NewText", edit.StartLine)
		}
		// --- End Parameter Conflict Check ---

		rng, err := getRange(edit.StartLine, edit.EndLine, filePath)
		if err != nil {
			return "", fmt.Errorf("invalid position: %v", err)
		}

		// --- Bracket Guard Check ---
		if edit.PreserveBrackets {
			// Read file content just before the check (could be optimized)
			contentBytes, readErr := os.ReadFile(filePath)
			if readErr != nil {
				// Log or handle error? For now, maybe return error as it's critical for the check.
				return "", fmt.Errorf("failed to read file for bracket check: %w", readErr)
			}
			if guardErr := checkBracketBalance(ctx, filePath, edit, contentBytes); guardErr != nil {
				// If the check fails, return the specific bracket guard error
				return "", guardErr
			}
		}
		// --- End Bracket Guard Check ---

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

// checkBracketBalance checks if the proposed edit range (defined by edit)
// would break bracket pairs in the given file content.
// TODO: Implement the actual bracket checking logic.
func checkBracketBalance(ctx context.Context, filePath string, edit TextEdit, content []byte) *BracketGuardError {
	// Default bracket pairs to check if not specified
	bracketPairs := map[rune]rune{
		'(': ')',
		'[': ']',
		'{': '}',
	}
	if len(edit.BracketTypes) > 0 {
		bracketPairs = make(map[rune]rune)
		for _, pair := range edit.BracketTypes {
			if len(pair) == 2 {
				runes := []rune(pair)
				bracketPairs[runes[0]] = runes[1]
			} else {
				// Optionally log a warning about invalid pair format
			}
		}
	}
	if len(bracketPairs) == 0 {
		return nil // No brackets to check
	}

	// --- Parsing and Checking Logic ---

	// Define reverse mapping for closing brackets
	closingBrackets := make(map[rune]rune)
	for open, close := range bracketPairs {
		closingBrackets[close] = open
	}

	// Convert content to lines
	contentStr := string(content)
	lineEnding := "\n"
	if strings.Contains(contentStr, "\r\n") {
		lineEnding = "\r\n"
	}
	lines := strings.Split(contentStr, lineEnding)

	// Get 0-based line indices for the edit range
	editStartLineIdx := edit.StartLine - 1
	editEndLineIdx := edit.EndLine - 1

	// Basic validation for line indices (should ideally be caught earlier)
	if editStartLineIdx < 0 { editStartLineIdx = 0 }
	if editEndLineIdx >= len(lines) { editEndLineIdx = len(lines) - 1 }
	if editStartLineIdx > editEndLineIdx {
		// Invalid range, but not a balance issue itself.
		return nil
	}

	type bracketInfo struct {
		char rune
		line int // 0-based index
		col  int // 0-based index
	}
	var stack []bracketInfo

	// Iterate through the lines to find bracket pairs and check against the edit range
	for lineIdx, line := range lines {
		for colIdx, char := range line {
			// Check for opening brackets
			if _, isOpen := bracketPairs[char]; isOpen {
				stack = append(stack, bracketInfo{char: char, line: lineIdx, col: colIdx})
			}

			// Check for closing brackets
			if openChar, isClose := closingBrackets[char]; isClose {
				if len(stack) > 0 && stack[len(stack)-1].char == openChar {
					// Found a matching pair
					openBracket := stack[len(stack)-1]
					closeBracket := bracketInfo{char: char, line: lineIdx, col: colIdx}
					stack = stack[:len(stack)-1] // Pop from stack

					// --- Check for violations based on LINE numbers ---
					// Note: Column checks are omitted for simplicity for now.

					isOpenInRange := openBracket.line >= editStartLineIdx && openBracket.line <= editEndLineIdx
					isCloseInRange := closeBracket.line >= editStartLineIdx && closeBracket.line <= editEndLineIdx

					// Case 1 & 2: Edit crosses one boundary of the pair
					if isOpenInRange != isCloseInRange {
						violationType := "CrossingPairStart"
						message := fmt.Sprintf("Edit range includes opening bracket '%c' at line %d but not its closing bracket at line %d", openBracket.char, openBracket.line+1, closeBracket.line+1)
						if !isOpenInRange && isCloseInRange {
							violationType = "CrossingPairEnd"
							message = fmt.Sprintf("Edit range includes closing bracket '%c' at line %d but not its opening bracket at line %d", closeBracket.char, closeBracket.line+1, openBracket.line+1)
						}
						return &BracketGuardError{
							ViolationType: violationType,
							Message:       message,
							// TODO: Add Suggestion (e.g., expand range to include both brackets)
						}
					}

					// Case 3: Edit is strictly inside the pair but doesn't contain either bracket line (only relevant for multi-line pairs)
					// This might be allowed depending on desired strictness. Let's allow it for now.

					// Case 4: Edit contains the pair entirely (this is generally safe)
					// if isOpenInRange && isCloseInRange { continue } // Safe

				} else {
					// Mismatched closing bracket - ignore for now.
				}
			}
		}
	}

	// If stack is not empty, there are unclosed brackets - ignore for now.

	// If no violations were found
	return nil
}
