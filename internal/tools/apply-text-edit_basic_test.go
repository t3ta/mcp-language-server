package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test Basic Functionality (Non-Regex) ---

func TestApplyTextEdits_SimpleReplace(t *testing.T) {
	ctx := context.Background()
	client := &mockLSPClient{} // Uses mock from helpers file
	initialContent := "Line 1\nLine 2\nLine 3"
	filePath := createTempFile(t, initialContent) // Uses helper from helpers file

	edits := []TextEdit{
		{Type: Replace, StartLine: 2, EndLine: 2, NewText: "Replaced Line 2"},
	}

	_, err := ApplyTextEdits(ctx, client, filePath, edits)
	require.NoError(t, err)

	expectedContent := "Line 1\nReplaced Line 2\nLine 3"
	actualContent := readFileContent(t, filePath) // Uses helper from helpers file
	assert.Equal(t, expectedContent, actualContent)
}

func TestApplyTextEdits_Insert(t *testing.T) {
	ctx := context.Background()
	client := &mockLSPClient{}
	initialContent := "Line 1\nLine 3"
	filePath := createTempFile(t, initialContent)

	edits := []TextEdit{
		{Type: Insert, StartLine: 2, EndLine: 2, NewText: "Inserted Line 2\n"}, // Insert before Line 3 (which is line 2 now)
	}

	_, err := ApplyTextEdits(ctx, client, filePath, edits)
	require.NoError(t, err)

	// Assuming insertion happens at the start of line index 1 (line 2).
	expectedContent := "Line 1\nInserted Line 2\nLine 3"
	actualContent := readFileContent(t, filePath)
	assert.Equal(t, expectedContent, actualContent, "Insertion behavior might differ based on getRange implementation")
}

func TestApplyTextEdits_Delete(t *testing.T) {
	ctx := context.Background()
	client := &mockLSPClient{}
	initialContent := "Line 1\nLine 2 to delete\nLine 3"
	filePath := createTempFile(t, initialContent)

	edits := []TextEdit{
		{Type: Delete, StartLine: 2, EndLine: 2},
	}

	_, err := ApplyTextEdits(ctx, client, filePath, edits)
	require.NoError(t, err)

	// Assuming getRange covers the line content up to the newline.
	expectedContent := "Line 1\nLine 3"
	actualContent := readFileContent(t, filePath)
	assert.Equal(t, expectedContent, actualContent, "Deletion behavior depends on getRange newline handling")
}
