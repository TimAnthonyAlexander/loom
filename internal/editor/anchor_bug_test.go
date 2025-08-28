package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAnchorReplaceDuplicationBug reproduces the bug where including anchor_before
// content in the replacement content causes duplication.
func TestAnchorReplaceDuplicationBug(t *testing.T) {
	// Original JSON content
	originalContent := `{
    "test": "in fact",
    "true?": false,
    "number": 123
}`

	// Create temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// This simulates the exact LLM call that caused the duplication bug
	req := AdvancedEditRequest{
		FilePath:     testFile,
		Action:       ActionAnchorReplace,
		AnchorBefore: "    \"number\": 123",
		AnchorAfter:  "}",
		Content:      "    \"number\": 123,\n    \"story\": \"Once upon a time, in a quiet village, a curious cat discovered a hidden garden full of magical flowers.\"",
	}

	// Create the edit plan
	result, err := ProposeAdvancedEdit(tmpDir, req)
	if err != nil {
		t.Fatalf("ProposeAdvancedEdit failed: %v", err)
	}

	t.Logf("Original content:\n%s", originalContent)
	t.Logf("New content:\n%s", result.NewContent)

	// Check for duplication - the number line should not appear twice
	numberLineCount := strings.Count(result.NewContent, `"number": 123`)
	if numberLineCount > 1 {
		t.Errorf("BUG REPRODUCED: Found %d occurrences of '\"number\": 123' in result, expected 1", numberLineCount)
		t.Logf("This demonstrates the duplication bug where anchor_before content is included in replacement")
	}

	// Check that the story was added
	if !strings.Contains(result.NewContent, `"story"`) {
		t.Errorf("Story field was not added to the JSON")
	}

	// The result should be valid JSON-like structure (though we won't parse it here)
	expectedStructure := []string{
		`"test": "in fact"`,
		`"true?": false`,
		`"number": 123`,
		`"story"`,
	}

	for _, expected := range expectedStructure {
		if !strings.Contains(result.NewContent, expected) {
			t.Errorf("Expected structure element '%s' not found in result", expected)
		}
	}
}

// TestAnchorReplaceCorrectUsage shows how ANCHOR_REPLACE should be used correctly
func TestAnchorReplaceCorrectUsage(t *testing.T) {
	// Original JSON content
	originalContent := `{
    "test": "in fact",
    "true?": false,
    "number": 123
}`

	// Create temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Correct approach: Don't include anchor_before content in the replacement
	req := AdvancedEditRequest{
		FilePath:     testFile,
		Action:       ActionAnchorReplace,
		AnchorBefore: "    \"number\": 123",
		AnchorAfter:  "}",
		Content:      "    \"number\": 123,\n    \"story\": \"Once upon a time, in a quiet village, a curious cat discovered a hidden garden full of magical flowers.\"\n}", // Include the closing brace
	}

	result, err := ProposeAdvancedEdit(tmpDir, req)
	if err != nil {
		t.Fatalf("ProposeAdvancedEdit failed: %v", err)
	}

	t.Logf("Correct usage result:\n%s", result.NewContent)

	// Should have exactly one occurrence of the number line
	numberLineCount := strings.Count(result.NewContent, `"number": 123`)
	if numberLineCount != 1 {
		t.Errorf("Expected exactly 1 occurrence of '\"number\": 123', got %d", numberLineCount)
	}
}
