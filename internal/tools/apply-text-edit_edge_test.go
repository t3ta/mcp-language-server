package tools

import (
	"context"
	// "errors" // errors パッケージは assert.ErrorIs で暗黙的に使われるので不要
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test Edge Cases ---

// Regex tests are moved to apply-text-edit_regex_test.go
// Basic tests (SimpleReplace, Insert, Delete) are moved to apply-text-edit_basic_test.go
// Multiple edits test is moved to apply-text-edit_multiple_test.go
// Bracket guard tests are moved to apply-text-edit_bracket_test.go

func TestApplyTextEdits_EmptyFile_Insert(t *testing.T) {
	ctx := context.Background()
	client := &mockLSPClient{} // Uses mock from helpers file
	filePath := createTempFile(t, "") // Uses helper from helpers file

	edits := []TextEdit{
		{Type: Insert, StartLine: 1, EndLine: 1, NewText: "Hello Empty World!\n"},
	}

	_, err := ApplyTextEdits(ctx, client, filePath, edits)
	require.NoError(t, err)

	expectedContent := "Hello Empty World!\n"
	actualContent := readFileContent(t, filePath) // Uses helper from helpers file
	assert.Equal(t, expectedContent, actualContent)
}

func TestApplyTextEdits_EmptyFile_Replace(t *testing.T) {
	ctx := context.Background()
	client := &mockLSPClient{}
	filePath := createTempFile(t, "")

	edits := []TextEdit{
		{Type: Replace, StartLine: 1, EndLine: 1, NewText: "Replaced Empty World!\n"},
	}

	_, err := ApplyTextEdits(ctx, client, filePath, edits)
	require.NoError(t, err)

	expectedContent := "Replaced Empty World!\n"
	actualContent := readFileContent(t, filePath)
	assert.Equal(t, expectedContent, actualContent)
}

func TestApplyTextEdits_NonExistentFile(t *testing.T) {
	ctx := context.Background()
	client := &mockLSPClient{}
	nonExistentFilePath := filepath.Join(t.TempDir(), "non_existent_file.txt")

	edits := []TextEdit{
		{Type: Insert, StartLine: 1, EndLine: 1, NewText: "Some text"},
	}

	_, err := ApplyTextEdits(ctx, client, nonExistentFilePath, edits)
	require.Error(t, err) // まずエラーが発生したことを確認
	assert.ErrorIs(t, err, os.ErrNotExist, "Expected error chain to include os.ErrNotExist") // errors.Is を使ってチェック！
}

func TestApplyTextEdits_SpecialChars_Emoji(t *testing.T) {
	ctx := context.Background()
	client := &mockLSPClient{}
	initialContent := "Line 1 ✨\nLine 2 🍣🚀\nLine 3"
	filePath := createTempFile(t, initialContent)

	edits := []TextEdit{
		{Type: Replace, StartLine: 1, EndLine: 1, NewText: "Line 1 REPLACED ✨"},
		{Type: Insert, StartLine: 3, EndLine: 3, NewText: "Line 2.5 INSERTED 💖\n"},
		{Type: Delete, StartLine: 2, EndLine: 2},
	}

	_, err := ApplyTextEdits(ctx, client, filePath, edits)
	require.NoError(t, err)

	expectedContent := "Line 1 REPLACED ✨\nLine 2.5 INSERTED 💖\nLine 3"
	actualContent := readFileContent(t, filePath)
	assert.Equal(t, expectedContent, actualContent)
}
