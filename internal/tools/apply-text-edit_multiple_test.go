package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test Multiple Edits ---

func TestApplyTextEdits_MultipleEditsSingleCall(t *testing.T) {
	ctx := context.Background()
	client := &mockLSPClient{} // Uses mock from helpers file
	initialContent := "Line 1: Original\nLine 2: Original\nLine 3: Original\nLine 4: Original\nLine 5: Original"
	filePath := createTempFile(t, initialContent) // Uses helper from helpers file

	// Test applying multiple non-overlapping edits in a single call
	edits := []TextEdit{
		{Type: Replace, StartLine: 1, EndLine: 1, NewText: "Line 1: Replaced"},
		{Type: Insert, StartLine: 3, EndLine: 3, NewText: "Line 2.5: Inserted\n"}, // Insert before original line 3
		{Type: Delete, StartLine: 5, EndLine: 5},                                  // Delete original line 5
		{Type: Replace, StartLine: 4, EndLine: 4, NewText: "Line 4: Replaced Non-Regex"}, // Replace original line 4
	}

	_, err := ApplyTextEdits(ctx, client, filePath, edits)
	require.NoError(t, err)

	// Expected content after applying non-regex edits based on original lines:
	expectedContent := "Line 1: Replaced\nLine 2: Original\nLine 2.5: Inserted\nLine 3: Original\nLine 4: Replaced Non-Regex" // Line 5 was deleted
	actualContent := readFileContent(t, filePath) // Uses helper from helpers file
	assert.Equal(t, expectedContent, actualContent)
}

// TODO: Add more complex multiple edit scenarios, including overlapping edits (if supported/expected to error)
// and combinations with regex.
