package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a temporary file with content
func createTempFile(t *testing.T, content string) string {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "apply_edit_test_*.txt")
	require.NoError(t, err)
	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)
	err = tmpFile.Close()
	require.NoError(t, err)
	absPath, err := filepath.Abs(tmpFile.Name())
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(absPath) }) // Ensure cleanup
	return absPath
}

// Helper function to read file content
func readFileContent(t *testing.T, filePath string) string {
	t.Helper()
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	return string(content)
}

// Mock LSP Client implementing the FileOpener interface
type mockLSPClient struct{}

// Implement OpenFile for testing purposes
func (m *mockLSPClient) OpenFile(ctx context.Context, filePath string) error {
	// Mock implementation - just check if file exists for simplicity
	// In a real scenario, you might want more sophisticated mocking
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("mock OpenFile failed for '%s': %w", filePath, err)
	}
	// fmt.Printf("Mock OpenFile called for: %s\n", filePath) // Debug print
	return nil
}

// --- Test Cases ---

func TestApplyTextEdits_RegexReplace_Simple(t *testing.T) {
	ctx := context.Background()
	// Instantiate the mock client properly
	// We don't need a fully functional real client for these tests,
	// so embedding a nil or zero client might be okay if OpenFile is the only method used.
	// However, let's create a minimal real client instance just in case.
	// If lsp.NewClient exists and is simple, use that. Otherwise, a zero struct.
	// Assuming a zero struct is sufficient for embedding here as we override OpenFile.
	client := &mockLSPClient{} // Use the simple mock client
	initialContent := "Hello World\nThis is a test\nWorld again"
	filePath := createTempFile(t, initialContent)

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
	actualContent := readFileContent(t, filePath)
	assert.Equal(t, expectedContent, actualContent)
}

func TestApplyTextEdits_RegexReplace_Multiline(t *testing.T) {
	ctx := context.Background()
	client := &mockLSPClient{} // Use the simple mock client
	initialContent := "Start\nLine 1\nLine 2\nEnd"
	filePath := createTempFile(t, initialContent)

	edits := []TextEdit{
		{
			Type:         Replace,
			StartLine:    2, // Line 1
			EndLine:      3, // Line 2
			IsRegex:      true,
			RegexPattern: `(?s)Line 1\nLine 2`, // (?s) for dotall mode
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
	client := &mockLSPClient{} // Use the simple mock client
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
	client := &mockLSPClient{} // Use the simple mock client
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

	// Content should remain unchanged
	expectedContent := "Hello World"
	actualContent := readFileContent(t, filePath)
	assert.Equal(t, expectedContent, actualContent)
}

func TestApplyTextEdits_InvalidRegexPattern(t *testing.T) {
	ctx := context.Background()
	client := &mockLSPClient{} // Use the simple mock client
	initialContent := "Some content"
	filePath := createTempFile(t, initialContent)

	edits := []TextEdit{
		{
			Type:         Replace,
			StartLine:    1,
			EndLine:      1,
			IsRegex:      true,
			RegexPattern: `[`, // Invalid regex
			RegexReplace: `X`,
		},
	}

	_, err := ApplyTextEdits(ctx, client, filePath, edits)
	require.Error(t, err) // Expect an error due to invalid regex
	assert.Contains(t, err.Error(), "invalid regex pattern")
}


// --- Test Existing Functionality (Non-Regex) ---

func TestApplyTextEdits_SimpleReplace(t *testing.T) {
	ctx := context.Background()
	client := &mockLSPClient{} // Use the simple mock client
	initialContent := "Line 1\nLine 2\nLine 3"
	filePath := createTempFile(t, initialContent)

	edits := []TextEdit{
		{Type: Replace, StartLine: 2, EndLine: 2, NewText: "Replaced Line 2"},
	}

	_, err := ApplyTextEdits(ctx, client, filePath, edits)
	require.NoError(t, err)

	expectedContent := "Line 1\nReplaced Line 2\nLine 3"
	actualContent := readFileContent(t, filePath)
	assert.Equal(t, expectedContent, actualContent)
}

func TestApplyTextEdits_Insert(t *testing.T) {
	ctx := context.Background()
	client := &mockLSPClient{} // Use the simple mock client
	initialContent := "Line 1\nLine 3"
	filePath := createTempFile(t, initialContent)

	edits := []TextEdit{
		{Type: Insert, StartLine: 2, EndLine: 2, NewText: "Inserted Line 2\n"}, // Insert before Line 3 (which is line 2 now)
	}

	_, err := ApplyTextEdits(ctx, client, filePath, edits)
	require.NoError(t, err)

	// Note: The getRange logic might need adjustment for precise insertion points.
	// This test assumes insertion happens at the beginning of the StartLine.
	// Depending on exact getRange behavior, expected might differ slightly.
	// Let's assume it inserts at the start of line index 1 (line 2).
	expectedContent := "Line 1\nInserted Line 2\nLine 3" // Adjust if getRange behaves differently
	actualContent := readFileContent(t, filePath)
	assert.Equal(t, expectedContent, actualContent, "Insertion behavior might differ based on getRange implementation")
}


func TestApplyTextEdits_Delete(t *testing.T) {
	ctx := context.Background()
	client := &mockLSPClient{} // Use the simple mock client
	initialContent := "Line 1\nLine 2 to delete\nLine 3"
	filePath := createTempFile(t, initialContent)

	edits := []TextEdit{
		{Type: Delete, StartLine: 2, EndLine: 2},
	}

	_, err := ApplyTextEdits(ctx, client, filePath, edits)
	require.NoError(t, err)

	// Expecting the line and its newline to be removed if getRange includes it.
	// If getRange only covers content, the newline might remain.
	// Assuming getRange covers the line content up to the newline.
	expectedContent := "Line 1\nLine 3" // Adjust based on getRange newline handling
	actualContent := readFileContent(t, filePath)
	assert.Equal(t, expectedContent, actualContent, "Deletion behavior depends on getRange newline handling")
}

// TODO: Add more tests:
// - Multiple edits (mix of regex and non-regex)
// - Edits at the beginning/end of the file
// - Edge cases with line endings (\r\n)
// - More complex capture group scenarios
// - Performance test for large files (might require a real LSP client mock)
