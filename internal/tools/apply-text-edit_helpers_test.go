package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require" // Only require is needed here
)

// Helper function to create a temporary file with content
func createTempFile(t *testing.T, content string) string {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "apply_edit_test_*.txt")
	require.NoError(t, err)
	// Write content only if it's not empty, otherwise CreateTemp already creates an empty file
	if content != "" {
		_, err = tmpFile.WriteString(content)
		require.NoError(t, err)
	}
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
// Note: This mock might need to be adjusted if other tests require different mock behavior.
type mockLSPClient struct{}

// Implement OpenFile for testing purposes
func (m *mockLSPClient) OpenFile(ctx context.Context, filePath string) error {
	// Mock implementation - just check if file exists for simplicity
	// In a real scenario, you might want more sophisticated mocking
	if _, err := os.Stat(filePath); err != nil {
		// Return an error that simulates file not found for the non-existent file test
		return fmt.Errorf("mock OpenFile failed for '%s': %w", filePath, os.ErrNotExist) // Use os.ErrNotExist for clarity
	}
	// fmt.Printf("Mock OpenFile called for: %s\n", filePath) // Debug print
	return nil
}

// Define the TextEdit and EditType types here as well, as they are used by tests
// (Assuming they are defined elsewhere, otherwise they need to be added or imported)
// If they are in the same 'tools' package but different file, they are accessible.
// Let's assume they are accessible within the package.
