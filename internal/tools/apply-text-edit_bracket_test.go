package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test Bracket Guard Functionality ---

func TestApplyTextEdits_BracketGuard_CrossingPair(t *testing.T) {
	ctx := context.Background()
	client := &mockLSPClient{} // Uses mock from helpers file
	initialContent := "func main() {\n  fmt.Println(\"Hello\")\n}"
	filePath := createTempFile(t, initialContent) // Uses helper from helpers file

	// Edit starts inside {} but ends outside
	edits := []TextEdit{
		{
			Type:             Replace,
			StartLine:        2, // Inside {}
			EndLine:          3, // Outside {}
			NewText:          " // Replaced",
			PreserveBrackets: true,
			BracketTypes:     []string{"{}"}, // Check only curly braces
		},
	}

	_, err := ApplyTextEdits(ctx, client, filePath, edits)
	require.Error(t, err) // Expect an error
	guardErr, ok := err.(*BracketGuardError)
	require.True(t, ok, "Expected BracketGuardError, got %T", err)
	assert.Equal(t, "CrossingPairEnd", guardErr.ViolationType)
	assert.Contains(t, guardErr.Message, "includes closing bracket '}' at line 3 but not its opening bracket at line 1")

	// Check that content was NOT modified
	actualContent := readFileContent(t, filePath) // Uses helper from helpers file
	assert.Equal(t, initialContent, actualContent)
}

func TestApplyTextEdits_BracketGuard_PartialPairStart(t *testing.T) {
	ctx := context.Background()
	client := &mockLSPClient{}
	initialContent := "(\n  value\n)"
	filePath := createTempFile(t, initialContent)

	// Edit includes opening ( but not closing )
	edits := []TextEdit{
		{
			Type:             Delete,
			StartLine:        1, // Includes (
			EndLine:          2, // Does not include )
			PreserveBrackets: true,
			BracketTypes:     []string{"()"},
		},
	}

	_, err := ApplyTextEdits(ctx, client, filePath, edits)
	require.Error(t, err)
	guardErr, ok := err.(*BracketGuardError)
	require.True(t, ok, "Expected BracketGuardError, got %T", err)
	assert.Equal(t, "CrossingPairStart", guardErr.ViolationType)
	assert.Contains(t, guardErr.Message, "includes opening bracket '(' at line 1 but not its closing bracket at line 3")

	actualContent := readFileContent(t, filePath)
	assert.Equal(t, initialContent, actualContent)
}

func TestApplyTextEdits_BracketGuard_SafeEditInsidePair(t *testing.T) {
	ctx := context.Background()
	client := &mockLSPClient{}
	initialContent := "{\n  \"key\": \"value\"\n}"
	filePath := createTempFile(t, initialContent)

	// Edit is entirely inside {}
	edits := []TextEdit{
		{
			Type:             Replace,
			StartLine:        2,
			EndLine:          2,
			NewText:          "  \"key\": \"new_value\"",
			PreserveBrackets: true,
			BracketTypes:     []string{"{}"},
		},
	}

	_, err := ApplyTextEdits(ctx, client, filePath, edits)
	require.NoError(t, err) // Expect no error

	expectedContent := "{\n  \"key\": \"new_value\"\n}"
	actualContent := readFileContent(t, filePath)
	assert.Equal(t, expectedContent, actualContent)
}

func TestApplyTextEdits_BracketGuard_SafeEditOutsidePair(t *testing.T) {
	ctx := context.Background()
	client := &mockLSPClient{}
	initialContent := "// Comment\n[\n  1, 2\n]\n// Another comment"
	filePath := createTempFile(t, initialContent)

	// Edit is entirely outside []
	edits := []TextEdit{
		{
			Type:             Replace,
			StartLine:        1,
			EndLine:          1,
			NewText:          "// New Comment",
			PreserveBrackets: true,
			BracketTypes:     []string{"[]"},
		},
	}

	_, err := ApplyTextEdits(ctx, client, filePath, edits)
	require.NoError(t, err) // Expect no error

	expectedContent := "// New Comment\n[\n  1, 2\n]\n// Another comment"
	actualContent := readFileContent(t, filePath)
	assert.Equal(t, expectedContent, actualContent)
}

func TestApplyTextEdits_BracketGuard_Disabled(t *testing.T) {
	ctx := context.Background()
	client := &mockLSPClient{}
	initialContent := "func main() {\n  fmt.Println(\"Hello\")\n}"
	filePath := createTempFile(t, initialContent)

	// Edit starts inside {} but ends outside, BUT guard is disabled
	edits := []TextEdit{
		{
			Type:             Replace,
			StartLine:        2, // Inside {}
			EndLine:          3, // Outside {}
			NewText:          " // Replaced",
			PreserveBrackets: false, // Guard disabled
		},
	}

	_, err := ApplyTextEdits(ctx, client, filePath, edits)
	require.NoError(t, err) // Expect no error because guard is off

	// Check that content WAS modified (even though it breaks brackets)
	expectedContent := "func main() {\n // Replaced"
	actualContent := readFileContent(t, filePath)
	assert.Equal(t, expectedContent, actualContent)
}
