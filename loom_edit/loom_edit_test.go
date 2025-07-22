package loom_edit

import (
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoomEditCases(t *testing.T) {
	testCases := []struct {
		name        string
		editFile    string
		expectedFile string
	}{
		{
			name:        "case1_replace",
			editFile:    "example/case1/edit.txt",
			expectedFile: "example/case1/final.md",
		},
		{
			name:        "case2_insert_after",
			editFile:    "example/case2/edit.txt", 
			expectedFile: "example/case2/final.md",
		},
		{
			name:        "case3_delete",
			editFile:    "example/case3/edit.txt",
			expectedFile: "example/case3/final.txt",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Read the base file
			baseContent, err := ioutil.ReadFile("example/base.md")
			if err != nil {
				t.Fatalf("Failed to read base.md: %v", err)
			}

			// Read the edit command
			editContent, err := ioutil.ReadFile(tc.editFile)
			if err != nil {
				t.Fatalf("Failed to read edit file %s: %v", tc.editFile, err)
			}

			// Read expected result
			expectedContent, err := ioutil.ReadFile(tc.expectedFile)
			if err != nil {
				t.Fatalf("Failed to read expected file %s: %v", tc.expectedFile, err)
			}

			// Create a temporary file with base content
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "test.md")
			err = ioutil.WriteFile(tmpFile, baseContent, 0644)
			if err != nil {
				t.Fatalf("Failed to write temp file: %v", err)
			}

			// Parse the edit command
			editCmd, err := ParseEditCommand(string(editContent))
			if err != nil {
				t.Fatalf("Failed to parse edit command: %v", err)
			}

			// Apply the edit to the temp file
			err = ApplyEdit(tmpFile, editCmd)
			if err != nil {
				t.Fatalf("Failed to apply edit: %v", err)
			}

			// Read the result
			resultContent, err := ioutil.ReadFile(tmpFile)
			if err != nil {
				t.Fatalf("Failed to read result file: %v", err)
			}

			// Compare with expected
			if string(resultContent) != string(expectedContent) {
				t.Errorf("Result doesn't match expected.\nGot:\n%s\nExpected:\n%s", 
					string(resultContent), string(expectedContent))
			}
		})
	}
}

func TestParseEditCommand(t *testing.T) {
	input := `>>LOOM_EDIT file=docs/CHANGELOG.md v=2eee673e114363992ac6afd6769ddca5986d1645 REPLACE 4-5
#OLD_HASH:cdca25044369b5e81f8073f02fc3e412a6a86360
- First stable release
- Integrated API layer
<<LOOM_EDIT`

	cmd, err := ParseEditCommand(input)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if cmd.File != "docs/CHANGELOG.md" {
		t.Errorf("Expected file 'docs/CHANGELOG.md', got '%s'", cmd.File)
	}
	if cmd.FileSHA != "2eee673e114363992ac6afd6769ddca5986d1645" {
		t.Errorf("Expected FileSHA '2eee673e114363992ac6afd6769ddca5986d1645', got '%s'", cmd.FileSHA)
	}
	if cmd.Action != "REPLACE" {
		t.Errorf("Expected action 'REPLACE', got '%s'", cmd.Action)
	}
	if cmd.Start != 4 {
		t.Errorf("Expected start 4, got %d", cmd.Start)
	}
	if cmd.End != 5 {
		t.Errorf("Expected end 5, got %d", cmd.End)
	}
	if cmd.OldHash != "cdca25044369b5e81f8073f02fc3e412a6a86360" {
		t.Errorf("Expected OldHash 'cdca25044369b5e81f8073f02fc3e412a6a86360', got '%s'", cmd.OldHash)
	}
	expectedNewText := "- First stable release\n- Integrated API layer"
	if cmd.NewText != expectedNewText {
		t.Errorf("Expected NewText '%s', got '%s'", expectedNewText, cmd.NewText)
	}
}

func TestHashValidation(t *testing.T) {
	// Test file hashing
	content := "test content\n"
	expected := fmt.Sprintf("%x", sha1.Sum([]byte(content)))
	actual := HashContent(content)
	if actual != expected {
		t.Errorf("Expected hash '%s', got '%s'", expected, actual)
	}

	// Test slice hashing
	lines := []string{"line1", "line2", "line3"}
	slice := lines[1:3] // "line2", "line3"
	sliceContent := strings.Join(slice, "\n")
	expectedSlice := fmt.Sprintf("%x", sha1.Sum([]byte(sliceContent)))
	actualSlice := HashContent(sliceContent)
	if actualSlice != expectedSlice {
		t.Errorf("Expected slice hash '%s', got '%s'", expectedSlice, actualSlice)
	}
}

func TestParseEditCommandErrors(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{
			name:  "invalid_header",
			input: "invalid header format",
		},
		{
			name:  "missing_old_hash",
			input: ">>LOOM_EDIT file=test.txt v=abc123 REPLACE 1-2\nsome content\n<<LOOM_EDIT",
		},
		{
			name:  "invalid_line_number",
			input: ">>LOOM_EDIT file=test.txt v=abc123 REPLACE abc-2\n#OLD_HASH:def456\n<<LOOM_EDIT",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseEditCommand(tc.input)
			if err == nil {
				t.Errorf("Expected error for case %s, but got none", tc.name)
			}
		})
	}
}

func TestApplyEditErrors(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2\nline3\n"
	err := ioutil.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	testCases := []struct {
		name string
		cmd  *EditCommand
	}{
		{
			name: "wrong_file_sha",
			cmd: &EditCommand{
				File:    "test.txt",
				FileSHA: "wrongsha",
				Action:  "REPLACE",
				Start:   1,
				End:     1,
				OldHash: "somehash",
				NewText: "new content",
			},
		},
		{
			name: "out_of_range_start",
			cmd: &EditCommand{
				File:    "test.txt",
				FileSHA: HashContent(content),
				Action:  "REPLACE",
				Start:   0, // Invalid start line
				End:     1,
				OldHash: "somehash",
				NewText: "new content",
			},
		},
		{
			name: "out_of_range_end",
			cmd: &EditCommand{
				File:    "test.txt",
				FileSHA: HashContent(content),
				Action:  "REPLACE",
				Start:   1,
				End:     10, // Invalid end line
				OldHash: "somehash",
				NewText: "new content",
			},
		},
		{
			name: "wrong_old_hash",
			cmd: &EditCommand{
				File:    "test.txt",
				FileSHA: HashContent(content),
				Action:  "REPLACE",
				Start:   1,
				End:     1,
				OldHash: "wronghash",
				NewText: "new content",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ApplyEdit(tmpFile, tc.cmd)
			if err == nil {
				t.Errorf("Expected error for case %s, but got none", tc.name)
			}
		})
	}
}

func TestInsertBeforeOperation(t *testing.T) {
	// Test INSERT_BEFORE operation
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2\nline3\n"
	err := ioutil.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	lines := strings.Split(content, "\n")
	oldSlice := lines[1:2] // line2
	oldHash := HashContent(strings.Join(oldSlice, "\n"))

	cmd := &EditCommand{
		File:    "test.txt",
		FileSHA: HashContent(content),
		Action:  "INSERT_BEFORE",
		Start:   2, // Insert before line2
		End:     2,
		OldHash: oldHash,
		NewText: "inserted line",
	}

	err = ApplyEdit(tmpFile, cmd)
	if err != nil {
		t.Fatalf("Failed to apply INSERT_BEFORE: %v", err)
	}

	result, err := ioutil.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read result: %v", err)
	}

	expected := "line1\ninserted line\nline2\nline3\n"
	if string(result) != expected {
		t.Errorf("INSERT_BEFORE failed.\nGot:\n%s\nExpected:\n%s", string(result), expected)
	}
} 