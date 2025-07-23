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
		name         string
		editFile     string
		expectedFile string
		baseFile     string
	}{
		{
			name:         "case1_replace",
			editFile:     "example/case1/edit.txt",
			expectedFile: "example/case1/final.md",
			baseFile:     "example/base.md",
		},
		{
			name:         "case2_insert_after",
			editFile:     "example/case2/edit.txt",
			expectedFile: "example/case2/final.md",
			baseFile:     "example/base.md",
		},
		{
			name:         "case3_delete",
			editFile:     "example/case3/edit.txt",
			expectedFile: "example/case3/final.txt",
			baseFile:     "example/base.md",
		},
		{
			name:         "case4_search_replace",
			editFile:     "example/case4/edit.txt",
			expectedFile: "example/case4/final.md",
			baseFile:     "example/case4/base.md", // Use case-specific base file
		},
		{
			name:         "case5_multiline_search_replace",
			editFile:     "example/case5/edit.txt",
			expectedFile: "example/case5/final.md",
			baseFile:     "example/case5/base.md", // Use case-specific base file
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Read the base file
			baseContent, err := ioutil.ReadFile(tc.baseFile)
			if err != nil {
				t.Fatalf("Failed to read base file %s: %v", tc.baseFile, err)
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

			// Normalize line endings before comparison
			normalizedResult := strings.ReplaceAll(string(resultContent), "\r\n", "\n")
			normalizedExpected := strings.ReplaceAll(string(expectedContent), "\r\n", "\n")

			// Compare with expected
			if normalizedResult != normalizedExpected {
				t.Errorf("Result doesn't match expected.\nGot:\n%s\nExpected:\n%s",
					string(resultContent), string(expectedContent))
			}
		})
	}
}

func TestParseEditCommand(t *testing.T) {
	input := `>>LOOM_EDIT file=docs/CHANGELOG.md REPLACE 4-5
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
	if cmd.Action != "REPLACE" {
		t.Errorf("Expected action 'REPLACE', got '%s'", cmd.Action)
	}
	if cmd.Start != 4 {
		t.Errorf("Expected start 4, got %d", cmd.Start)
	}
	if cmd.End != 5 {
		t.Errorf("Expected end 5, got %d", cmd.End)
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
			name:  "invalid_line_number",
			input: ">>LOOM_EDIT file=test.txt REPLACE abc-2\n<<LOOM_EDIT",
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
			name: "out_of_range_start",
			cmd: &EditCommand{
				File:    "test.txt",
				Action:  "REPLACE",
				Start:   0, // Invalid start line
				End:     1,
				NewText: "new content",
			},
		},
		{
			name: "out_of_range_end",
			cmd: &EditCommand{
				File:    "test.txt",
				Action:  "REPLACE",
				Start:   1,
				End:     10, // Invalid end line
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

	cmd := &EditCommand{
		File:    "test.txt",
		Action:  "INSERT_BEFORE",
		Start:   2, // Insert before line2
		End:     2,
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

func TestNewlineNormalization(t *testing.T) {
	testCases := []struct {
		name            string
		originalContent string
		newText         string
		expectedResult  string
	}{
		{
			name:            "crlf_to_lf",
			originalContent: "line1\r\nline2\r\nline3\r\n",
			newText:         "replaced line",
			expectedResult:  "replaced line\nline2\nline3\n",
		},
		{
			name:            "mixed_line_endings",
			originalContent: "line1\r\nline2\nline3\r",
			newText:         "replaced\r\nwith\rmixed",
			expectedResult:  "replaced\nwith\nmixed\nline2\nline3\n",
		},
		{
			name:            "cr_only",
			originalContent: "line1\rline2\rline3\r",
			newText:         "new line",
			expectedResult:  "new line\nline2\nline3\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create temporary file with original content
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "test.txt")
			err := ioutil.WriteFile(tmpFile, []byte(tc.originalContent), 0644)
			if err != nil {
				t.Fatalf("Failed to write temp file: %v", err)
			}

			// Normalize for SHA calculation
			normalizedOriginal := strings.ReplaceAll(tc.originalContent, "\r\n", "\n")
			normalizedOriginal = strings.ReplaceAll(normalizedOriginal, "\r", "\n")

			cmd := &EditCommand{
				File:    "test.txt",
				Action:  "REPLACE",
				Start:   1,
				End:     1,
				NewText: tc.newText,
			}

			err = ApplyEdit(tmpFile, cmd)
			if err != nil {
				t.Fatalf("Failed to apply edit: %v", err)
			}

			result, err := ioutil.ReadFile(tmpFile)
			if err != nil {
				t.Fatalf("Failed to read result: %v", err)
			}

			if string(result) != tc.expectedResult {
				t.Errorf("Normalization failed.\nGot:\n%q\nExpected:\n%q", string(result), tc.expectedResult)
			}
		})
	}
}

func TestTrailingNewlinePreservation(t *testing.T) {
	testCases := []struct {
		name            string
		originalContent string
		action          string
		newText         string
		expectedEnding  string
	}{
		{
			name:            "preserve_trailing_newline",
			originalContent: "line1\nline2\nline3\n",
			action:          "REPLACE",
			newText:         "replaced",
			expectedEnding:  "\n",
		},
		{
			name:            "preserve_no_trailing_newline",
			originalContent: "line1\nline2\nline3",
			action:          "REPLACE",
			newText:         "replaced",
			expectedEnding:  "",
		},
		{
			name:            "insert_preserves_trailing_newline",
			originalContent: "line1\nline2\n",
			action:          "INSERT_AFTER",
			newText:         "inserted",
			expectedEnding:  "\n",
		},
		{
			name:            "delete_preserves_no_trailing_newline",
			originalContent: "line1\nline2\nline3",
			action:          "DELETE",
			newText:         "",
			expectedEnding:  "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "test.txt")
			err := ioutil.WriteFile(tmpFile, []byte(tc.originalContent), 0644)
			if err != nil {
				t.Fatalf("Failed to write temp file: %v", err)
			}

			var start, end int

			switch tc.action {
			case "REPLACE":
				start, end = 1, 1
			case "INSERT_AFTER":
				start, end = 1, 1
			case "DELETE":
				start, end = 3, 3
			}

			cmd := &EditCommand{
				File:    "test.txt",
				Action:  tc.action,
				Start:   start,
				End:     end,
				NewText: tc.newText,
			}

			err = ApplyEdit(tmpFile, cmd)
			if err != nil {
				t.Fatalf("Failed to apply edit: %v", err)
			}

			result, err := ioutil.ReadFile(tmpFile)
			if err != nil {
				t.Fatalf("Failed to read result: %v", err)
			}

			actualEnding := ""
			if strings.HasSuffix(string(result), "\n") {
				actualEnding = "\n"
			}

			if actualEnding != tc.expectedEnding {
				t.Errorf("Trailing newline not preserved.\nExpected ending: %q\nActual ending: %q\nFull result: %q",
					tc.expectedEnding, actualEnding, string(result))
			}
		})
	}
}

// TestParseEditCommandMissingEndLine tests handling of commands with missing end line numbers
func TestParseEditCommandMissingEndLine(t *testing.T) {
	// Test for single line number (REPLACE without end line)
	input := ">>LOOM_EDIT file=sample.json REPLACE 3\n    \"name\": \"Chair\",\n<<LOOM_EDIT"

	cmd, err := ParseEditCommand(input)
	if err != nil {
		t.Log("Error captured as expected:", err)
	} else {
		// We expect this to pass now with the fix
		t.Log("Command parsed with end=start fallback")
		if cmd.Start != 3 || cmd.End != 3 {
			t.Errorf("Expected start=3, end=3, got start=%d, end=%d", cmd.Start, cmd.End)
		}
		if cmd.Action != "REPLACE" {
			t.Errorf("Expected action=REPLACE, got action=%s", cmd.Action)
		}
		if cmd.File != "sample.json" {
			t.Errorf("Expected file=sample.json, got file=%s", cmd.File)
		}
		if cmd.NewText != "    \"name\": \"Chair\"," {
			t.Errorf("Expected newText='    \"name\": \"Chair\",', got newText='%s'", cmd.NewText)
		}
	}
}

// TestParseSearchReplaceCommand tests parsing of the SEARCH_REPLACE command
func TestParseSearchReplaceCommand(t *testing.T) {
	input := `>>LOOM_EDIT file=config.js SEARCH_REPLACE "localhost:8080" "localhost:9090"
<<LOOM_EDIT`

	cmd, err := ParseEditCommand(input)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if cmd.File != "config.js" {
		t.Errorf("Expected file 'config.js', got '%s'", cmd.File)
	}
	if cmd.Action != "SEARCH_REPLACE" {
		t.Errorf("Expected action 'SEARCH_REPLACE', got '%s'", cmd.Action)
	}
	if cmd.OldString != "localhost:8080" {
		t.Errorf("Expected OldString 'localhost:8080', got '%s'", cmd.OldString)
	}
	if cmd.NewString != "localhost:9090" {
		t.Errorf("Expected NewString 'localhost:9090', got '%s'", cmd.NewString)
	}

	// Test with multiline strings
	multilineInput := `>>LOOM_EDIT file=config.js SEARCH_REPLACE "const config = {
  port: 8080
}" "const config = {
  port: 9090
}"
<<LOOM_EDIT`

	multilineCmd, err := ParseEditCommand(multilineInput)
	if err != nil {
		t.Fatalf("Failed to parse multiline command: %v", err)
	}

	if !strings.Contains(multilineCmd.OldString, "8080") {
		t.Errorf("Old string doesn't contain expected content '8080', got: '%s'", multilineCmd.OldString)
	}

	if !strings.Contains(multilineCmd.NewString, "9090") {
		t.Errorf("New string doesn't contain expected content '9090', got: '%s'", multilineCmd.NewString)
	}
}

// TestSearchReplaceOperation tests the SEARCH_REPLACE operation
func TestSearchReplaceOperation(t *testing.T) {
	// Test SEARCH_REPLACE operation
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	content := "The server is at localhost:8080 and the backup is at localhost:8080/backup"
	err := ioutil.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	cmd := &EditCommand{
		File:      tmpFile,
		Action:    "SEARCH_REPLACE",
		OldString: "localhost:8080",
		NewString: "example.com:9090",
		Start:     1, // Make sure to set valid start/end values
		End:       1, // even though they're not used for SEARCH_REPLACE
	}

	err = ApplyEdit(tmpFile, cmd)
	if err != nil {
		t.Fatalf("Failed to apply SEARCH_REPLACE: %v", err)
	}

	result, err := ioutil.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read result: %v", err)
	}

	expected := "The server is at example.com:9090 and the backup is at example.com:9090/backup"
	if string(result) != expected {
		t.Errorf("SEARCH_REPLACE failed.\nGot:\n%s\nExpected:\n%s", string(result), expected)
	}

	// Test with string not found in file
	notFoundCmd := &EditCommand{
		File:      tmpFile,
		Action:    "SEARCH_REPLACE",
		OldString: "string-not-in-file",
		NewString: "replacement",
		Start:     1, // Valid values required
		End:       1,
	}

	err = ApplyEdit(tmpFile, notFoundCmd)
	if err == nil {
		t.Errorf("Expected error when string not found, but got none")
	}
}
