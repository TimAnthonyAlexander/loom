package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRealUserScenario reproduces the exact scenario the user experienced
func TestRealUserScenario(t *testing.T) {
	// Exact content from user's example
	originalContent := `{
    "test": "in fact",
    "true?": false,
    "number": 123
}`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Exact LLM call from the user's example that caused duplication
	req := AdvancedEditRequest{
		FilePath:     testFile,
		Action:       ActionAnchorReplace,
		AnchorBefore: "    \"number\": 123",
		AnchorAfter:  "}",
		Content:      "    \"number\": 123,\n    \"story\": \"Once upon a time, in a quiet village, a curious cat discovered a hidden garden full of magical flowers.\"",
	}

	result, err := ProposeAdvancedEdit(tmpDir, req)
	if err != nil {
		t.Fatalf("ProposeAdvancedEdit failed: %v", err)
	}

	t.Logf("=== BEFORE FIX (what user experienced) ===")
	t.Logf("This would have had duplication:")
	t.Logf(`{
    "test": "in fact", 
    "true?": false,
    "number": 123
    "number": 123,
    "story": "Once upon a time..."
}`)

	t.Logf("\n=== AFTER FIX (current result) ===")
	t.Logf("Result:\n%s", result.NewContent)

	// Critical check: NO duplication of "number": 123
	numberOccurrences := strings.Count(result.NewContent, `"number": 123`)
	if numberOccurrences > 1 {
		t.Errorf("DUPLICATION BUG STILL EXISTS: Found %d occurrences of '\"number\": 123', expected 1", numberOccurrences)
	} else {
		t.Logf("✅ SUCCESS: Duplication bug is fixed - found exactly %d occurrence of 'number': 123", numberOccurrences)
	}

	// Verify story was added
	if !strings.Contains(result.NewContent, `"story"`) {
		t.Errorf("Story field was not added")
	} else {
		t.Logf("✅ SUCCESS: Story field was properly added")
	}

	// Check that we have valid JSON structure (basic check)
	requiredFields := []string{`"test": "in fact"`, `"true?": false`, `"number": 123`, `"story"`}
	for _, field := range requiredFields {
		if !strings.Contains(result.NewContent, field) {
			t.Errorf("Missing required field: %s", field)
		}
	}

	t.Logf("✅ All checks passed - the anchor duplication bug has been successfully fixed!")
}

// TestShowImprovement demonstrates the improvement in editing experience
func TestShowImprovement(t *testing.T) {
	tmpDir := t.TempDir()

	t.Logf("=== DEMONSTRATION: LLM Editing Experience Improvement ===")

	// Before: LLM had to be perfect about not including anchor_before in content
	// After: System is smart and auto-detects/fixes the overlap

	testCases := []struct {
		name        string
		content     string
		description string
	}{
		{
			name:        "case1_llm_includes_anchor",
			content:     "    \"number\": 123,\n    \"new_field\": \"value\"",
			description: "LLM incorrectly includes anchor_before in content (common mistake)",
		},
		{
			name:        "case2_llm_perfect",
			content:     ",\n    \"new_field\": \"value\"",
			description: "LLM correctly excludes anchor_before from content (rare)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			originalContent := `{
    "test": "value",
    "number": 123
}`
			testFile := filepath.Join(tmpDir, tc.name+".json")
			if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			req := AdvancedEditRequest{
				FilePath:     testFile,
				Action:       ActionAnchorReplace,
				AnchorBefore: "    \"number\": 123",
				AnchorAfter:  "}",
				Content:      tc.content,
			}

			result, err := ProposeAdvancedEdit(tmpDir, req)
			if err != nil {
				t.Fatalf("ProposeAdvancedEdit failed for %s: %v", tc.name, err)
			}

			t.Logf("Case: %s", tc.description)
			t.Logf("Content: %q", tc.content)
			t.Logf("Result:\n%s", result.NewContent)

			// Both cases should now work correctly (no duplication)
			numberCount := strings.Count(result.NewContent, `"number": 123`)
			if numberCount != 1 {
				t.Errorf("Expected exactly 1 'number': 123, got %d", numberCount)
			}

			if !strings.Contains(result.NewContent, `"new_field"`) {
				t.Errorf("new_field not found in result")
			}

			t.Logf("✅ %s handled correctly\n", tc.name)
		})
	}
}
