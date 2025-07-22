package task

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoomEditProcessor_ParseLoomEdits(t *testing.T) {
	processor := NewLoomEditProcessor("/test/workspace")

	tests := []struct {
		name        string
		message     string
		expectCount int
		expectPaths []string
	}{
		{
			name:        "no edit blocks",
			message:     "This is just a regular message without any edits.",
			expectCount: 0,
			expectPaths: []string{},
		},
		{
			name: "single edit block",
			message: "Here's an edit:\n\n```LOOM_EDIT\n--- a/test.txt\n+++ b/test.txt\n@@ -1,1 +1,2 @@\n hello\n+world\n```\n\nThat was the edit.",
			expectCount: 1,
			expectPaths: []string{"test.txt"},
		},
		{
			name: "multiple edit blocks",
			message: "First edit:\n\n```LOOM_EDIT\n--- a/file1.txt\n+++ b/file1.txt\n@@\n hello\n+world\n```\n\nSecond edit:\n\n```LOOM_EDIT\n--- a/file2.txt\n+++ b/file2.txt\n@@\n foo\n+bar\n```",
			expectCount: 2,
			expectPaths: []string{"file1.txt", "file2.txt"},
		},
		{
			name: "edit block with explanation",
			message: "I need to fix this bug by adding a line.\n\n```LOOM_EDIT\n--- a/main.go\n+++ b/main.go\n@@\n package main\n \n+import \"fmt\"\n func main() {\n```\n\nThis import is necessary for the fmt.Println call.",
			expectCount: 1,
			expectPaths: []string{"main.go"},
		},
		{
			name: "plain LOOM_EDIT format without markdown",
			message: `LOOM_EDIT\n--- a/sample.json\n+++ b/sample.json\n@@ -3,7 +3,7 @@\n     "name": "Sample Item",\n-    "price": 19.99,\n+    "price": 18.99,\n     "inStock": true,\n     "tags": [\nOBJECTIVE_COMPLETE: Updated the price in sample.json from 19.99 to 18.99.`,
			expectCount: 1,
			expectPaths: []string{"sample.json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks, err := processor.ParseLoomEdits(tt.message)
			if err != nil {
				t.Fatalf("ParseLoomEdits() error = %v", err)
			}

			if len(blocks) != tt.expectCount {
				t.Errorf("ParseLoomEdits() got %d blocks, want %d", len(blocks), tt.expectCount)
			}

			if len(blocks) != len(tt.expectPaths) {
				t.Errorf("ParseLoomEdits() got %d blocks, want %d paths", len(blocks), len(tt.expectPaths))
				return
			}

			for i, block := range blocks {
				if block.FilePath != tt.expectPaths[i] {
					t.Errorf("ParseLoomEdits() block %d got path %s, want %s", i, block.FilePath, tt.expectPaths[i])
				}
			}
		})
	}
}

func TestLoomEditProcessor_ValidateDiffFormat(t *testing.T) {
	processor := NewLoomEditProcessor("/test/workspace")

	tests := []struct {
		name      string
		diff      string
		expectErr bool
	}{
		{
			name: "valid diff",
			diff: "--- a/test.txt\n+++ b/test.txt\n@@\n hello\n+world\n@@",
			expectErr: false,
		},
		{
			name: "missing minus header",
			diff: "+++ b/test.txt\n@@\n hello\n+world\n@@",
			expectErr: true,
		},
		{
			name: "missing plus header",
			diff: "--- a/test.txt\n@@\n hello\n+world\n@@",
			expectErr: true,
		},
		{
			name: "missing hunk header",
			diff: "--- a/test.txt\n+++ b/test.txt\n hello\n+world",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.ValidateDiffFormat(tt.diff)
			if (err != nil) != tt.expectErr {
				t.Errorf("ValidateDiffFormat() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

// TestLoomEditWithFixtures tests the LOOM_EDIT system using the real test fixtures
func TestLoomEditWithFixtures(t *testing.T) {
	// Get the workspace root (go up from task/ directory)
	workspaceRoot, err := filepath.Abs("../")
	if err != nil {
		t.Fatalf("Failed to get workspace root: %v", err)
	}

	// Verify we're in a git repository
	if _, err := os.Stat(filepath.Join(workspaceRoot, ".git")); os.IsNotExist(err) {
		t.Skipf("Skipping test: not in a git repository")
	}

	testCases := []struct {
		name         string
		caseDir      string
		expectedFile string
	}{
		{
			name:         "case1 - insert lines",
			caseDir:      "case1",
			expectedFile: "output.txt",
		},
		{
			name:         "case2 - replace line",
			caseDir:      "case2",
			expectedFile: "output.txt",
		},
		{
			name:         "case3 - add bullet point",
			caseDir:      "case3",
			expectedFile: "output.txt",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Read the command (raw diff) - use the clean version
			commandFile := filepath.Join(workspaceRoot, "editing_tests", tc.caseDir, "command_clean.txt")
			diffContent, err := os.ReadFile(commandFile)
			if err != nil {
				t.Fatalf("Failed to read command file: %v", err)
			}

			// The patch operates on thefile.md directly - make a backup first
			baselineFile := filepath.Join(workspaceRoot, "editing_tests", "thefile.md")
			backupFile := filepath.Join(workspaceRoot, "editing_tests", "thefile_backup.md")
			
			// Backup original file
			baselineContent, err := os.ReadFile(baselineFile)
			if err != nil {
				t.Fatalf("Failed to read baseline file: %v", err)
			}
			
			if err := os.WriteFile(backupFile, baselineContent, 0644); err != nil {
				t.Fatalf("Failed to create backup file: %v", err)
			}
			
			// Restore original file when done
			defer func() {
				os.WriteFile(baselineFile, baselineContent, 0644)
				os.Remove(backupFile)
			}()

			// Wrap the diff in LOOM_EDIT block - ensure proper newline before closing
			message := "Here's the edit:\n\n```LOOM_EDIT\n" + string(diffContent) + "\n```\n\nDone!"

			// Apply the edit
			processor := NewLoomEditProcessor(workspaceRoot)
			result, err := processor.ProcessMessage(message)
			if err != nil {
				t.Fatalf("ProcessMessage() failed: %v", err)
			}

			if result.BlocksFound != 1 {
				t.Errorf("Expected 1 block, got %d", result.BlocksFound)
			}

			if len(result.FilesEdited) != 1 || result.FilesEdited[0] != "editing_tests/thefile.md" {
				t.Errorf("Expected to edit editing_tests/thefile.md, got %v", result.FilesEdited)
			}

			// Read the result
			actualContent, err := os.ReadFile(baselineFile)
			if err != nil {
				t.Fatalf("Failed to read result file: %v", err)
			}

			// Read expected output
			expectedFile := filepath.Join(workspaceRoot, "editing_tests", tc.caseDir, tc.expectedFile)
			expectedContent, err := os.ReadFile(expectedFile)
			if err != nil {
				t.Fatalf("Failed to read expected file: %v", err)
			}

			// Compare
			if string(actualContent) != string(expectedContent) {
				t.Errorf("Content mismatch.\nExpected:\n%s\nActual:\n%s", string(expectedContent), string(actualContent))
			}
		})
	}
}

func TestLoomEditProcessor_extractFilePathFromDiff(t *testing.T) {
	processor := NewLoomEditProcessor("/test/workspace")

	tests := []struct {
		name     string
		diff     string
		expected string
		wantErr  bool
	}{
		{
			name:     "simple path",
			diff:     "--- a/test.txt\n+++ b/test.txt\n@@\n hello\n+world\n@@",
			expected: "test.txt",
			wantErr:  false,
		},
		{
			name:     "nested path",
			diff:     "--- a/src/main.go\n+++ b/src/main.go\n@@\n package main\n+import \"fmt\"\n@@",
			expected: "src/main.go",
			wantErr:  false,
		},
		{
			name:     "complex path",
			diff:     "--- a/editing_tests/thefile.md\n+++ b/editing_tests/thefile.md\n@@\n # Title\n+New line\n@@",
			expected: "editing_tests/thefile.md",
			wantErr:  false,
		},
		{
			name:    "missing plus header",
			diff:    "--- a/test.txt\n@@\n hello\n+world\n@@",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := processor.extractFilePathFromDiff(tt.diff)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractFilePathFromDiff() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("extractFilePathFromDiff() got = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestComplexLoomEditScenarios tests more complex scenarios
func TestComplexLoomEditScenarios(t *testing.T) {
	processor := NewLoomEditProcessor("/test/workspace")

	t.Run("multiple hunks in single file", func(t *testing.T) {
		message := `Here are multiple changes to the same file:

` + "```LOOM_EDIT" + `
--- a/example.txt
+++ b/example.txt
@@
 line 1
+inserted line after 1
 line 2
@@
 line 5
-line 6 to remove
 line 7
@@
` + "```" + `

These changes add a line and remove another.`

		blocks, err := processor.ParseLoomEdits(message)
		if err != nil {
			t.Fatalf("ParseLoomEdits() error = %v", err)
		}

		if len(blocks) != 1 {
			t.Errorf("Expected 1 block, got %d", len(blocks))
		}

		if blocks[0].FilePath != "example.txt" {
			t.Errorf("Expected path example.txt, got %s", blocks[0].FilePath)
		}

		// Validate the diff format
		if err := processor.ValidateDiffFormat(blocks[0].DiffContent); err != nil {
			t.Errorf("ValidateDiffFormat() failed: %v", err)
		}
	})

	t.Run("edit blocks with extra whitespace", func(t *testing.T) {
		message := `

` + "```LOOM_EDIT" + `

--- a/test.txt
+++ b/test.txt
@@
 hello
+world
@@

` + "```" + `

`

		blocks, err := processor.ParseLoomEdits(message)
		if err != nil {
			t.Fatalf("ParseLoomEdits() error = %v", err)
		}

		if len(blocks) != 1 {
			t.Errorf("Expected 1 block, got %d", len(blocks))
		}
	})
} 