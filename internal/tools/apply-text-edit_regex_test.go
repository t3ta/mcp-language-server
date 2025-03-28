package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test Regex Functionality ---

func TestApplyTextEdits_RegexReplace_Simple(t *testing.T) {
	ctx := context.Background()
	client := &mockLSPClient{} // Uses mock from helpers file
	initialContent := "Hello World\nThis is a test\nWorld again"
	filePath := createTempFile(t, initialContent) // Uses helper from helpers file

	edits := []TextEdit{
		{
			Type:         Replace,
			StartLine:    1,
			EndLine:      3, // Apply regex across all lines
			IsRegex:      true,
			RegexPattern: `World`,
			RegexReplace: `Universe`,
		},
	}

	_, err := ApplyTextEdits(ctx, client, filePath, edits)
	require.NoError(t, err)

	expectedContent := "Hello Universe\nThis is a test\nUniverse again"
	actualContent := readFileContent(t, filePath) // Uses helper from helpers file
	assert.Equal(t, expectedContent, actualContent)
}

func TestApplyTextEdits_RegexReplace_Multiline(t *testing.T) {
	ctx := context.Background()
	client := &mockLSPClient{}
	initialContent := "Start\nLine 1\nLine 2\nEnd"
	filePath := createTempFile(t, initialContent)

	edits := []TextEdit{
		{
			Type:         Replace,
			StartLine:    2,
			EndLine:      3,
			IsRegex:      true,
			RegexPattern: `(?s)Line 1\nLine 2`,
			RegexReplace: `Replaced Block`,
		},
	}

	_, err := ApplyTextEdits(ctx, client, filePath, edits)
	require.NoError(t, err)

	expectedContent := "Start\nReplaced Block\nEnd"
	actualContent := readFileContent(t, filePath)
	assert.Equal(t, expectedContent, actualContent)
}

func TestApplyTextEdits_RegexReplace_CaptureGroup(t *testing.T) {
	ctx := context.Background()
	client := &mockLSPClient{}
	initialContent := "Name: Alice\nName: Bob"
	filePath := createTempFile(t, initialContent)

	edits := []TextEdit{
		{
			Type:         Replace,
			StartLine:    1,
			EndLine:      2,
			IsRegex:      true,
			RegexPattern: `Name: (\w+)`,
			RegexReplace: `User: $1`,
		},
	}

	_, err := ApplyTextEdits(ctx, client, filePath, edits)
	require.NoError(t, err)

	expectedContent := "User: Alice\nUser: Bob"
	actualContent := readFileContent(t, filePath)
	assert.Equal(t, expectedContent, actualContent)
}

func TestApplyTextEdits_RegexReplace_NoMatch(t *testing.T) {
	ctx := context.Background()
	client := &mockLSPClient{}
	initialContent := "Hello World"
	filePath := createTempFile(t, initialContent)

	edits := []TextEdit{
		{
			Type:         Replace,
			StartLine:    1,
			EndLine:      1,
			IsRegex:      true,
			RegexPattern: `NotFound`,
			RegexReplace: `Replaced`,
		},
	}

	_, err := ApplyTextEdits(ctx, client, filePath, edits)
	require.NoError(t, err)

	expectedContent := "Hello World"
	actualContent := readFileContent(t, filePath)
	assert.Equal(t, expectedContent, actualContent)
}

func TestApplyTextEdits_InvalidRegexPattern(t *testing.T) {
	ctx := context.Background()
	client := &mockLSPClient{}
	initialContent := "Some content"
	filePath := createTempFile(t, initialContent)

	edits := []TextEdit{
		{
			Type:         Replace,
			StartLine:    1,
			EndLine:      1,
			IsRegex:      true,
			RegexPattern: `[`,
			RegexReplace: `X`,
		},
	}

	_, err := ApplyTextEdits(ctx, client, filePath, edits)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid regex pattern")
}

func TestApplyTextEdits_RegexAndNewTextConflict(t *testing.T) {
	ctx := context.Background()
	client := &mockLSPClient{}
	initialContent := "Hello World"
	filePath := createTempFile(t, initialContent)

	edits := []TextEdit{
		{
			Type:         Replace,
			StartLine:    1,
			EndLine:      1,
			IsRegex:      true,          // Regex is true
			RegexPattern: `World`,       // Valid regex pattern
			RegexReplace: `Universe`,    // Valid regex replace
			NewText:      `Should be ignored or cause error`, // ALSO provide NewText
		},
	}

	// We expect an error because both regex fields and NewText are provided when IsRegex is true.
	// Or, if the implementation prioritizes regex, we'd expect no error and regex to be applied.
	// Let's assume an error is the desired behavior for clarity.
	_, err := ApplyTextEdits(ctx, client, filePath, edits)
	require.Error(t, err, "Expected an error due to conflicting parameters (IsRegex=true and NewText provided)")
	// Optionally, check for a specific error type or message if the implementation provides one.
	// For now, just ensuring any error occurs is a good start.
	// assert.Contains(t, err.Error(), "conflicting parameters", "Error message should indicate conflict")

	// Also verify the file content wasn't changed unexpectedly if an error occurred
	actualContent := readFileContent(t, filePath)
	assert.Equal(t, initialContent, actualContent, "File content should not change on error")
}
