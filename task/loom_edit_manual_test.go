package task

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestManualPatchCreation tests patch creation and application manually
func TestManualPatchCreation(t *testing.T) {
	// Get the workspace root
	workspaceRoot, err := filepath.Abs("../")
	if err != nil {
		t.Fatalf("Failed to get workspace root: %v", err)
	}

	// Create a simple test file
	testContent := `# This is a file to be tested against with applying editing diffs

This is a line beneath the headline.

## Whats up bro

- Bullet
- Points
- Are
- Necessary

----

Tim Anthony Alexander

`

	testFile := filepath.Join(workspaceRoot, "editing_tests", "manual_test.md")
	
	// Write initial content
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(testFile)

	// Read one of our properly generated patches
	patchFile := filepath.Join(workspaceRoot, "editing_tests", "case1", "command_fixed.txt")
	patchBytes, err := os.ReadFile(patchFile)
	if err != nil {
		t.Fatalf("Failed to read patch file: %v", err)
	}
	
	// Adjust path for our test file
	patch := strings.ReplaceAll(string(patchBytes), "editing_tests/thefile.md", "editing_tests/manual_test.md")

	// Test our processor
	processor := NewLoomEditProcessor(workspaceRoot)
	
	// Wrap in LOOM_EDIT block
	message := "```LOOM_EDIT\n" + patch + "```"
	
	// Debug: print the exact message being processed
	t.Logf("Processing message:\n%s", message)
	
	// Parse the edit
	blocks, err := processor.ParseLoomEdits(message)
	if err != nil {
		t.Fatalf("ParseLoomEdits failed: %v", err)
	}
	
	if len(blocks) != 1 {
		t.Fatalf("Expected 1 block, got %d", len(blocks))
	}
	
	t.Logf("Parsed block:\nFilePath: %s\nDiffContent:\n%s", blocks[0].FilePath, blocks[0].DiffContent)
	
	// Debug: show exact content with line numbers
	lines := strings.Split(blocks[0].DiffContent, "\n")
	for i, line := range lines {
		t.Logf("Line %d: %q", i+1, line)
	}
	
	// Try to apply manually first to debug
	tmpFile := filepath.Join(os.TempDir(), "debug.patch")
	if err := os.WriteFile(tmpFile, []byte(blocks[0].DiffContent), 0600); err != nil {
		t.Fatalf("Failed to write patch file: %v", err)
	}
	defer os.Remove(tmpFile)
	
	// Also debug the raw patch file content
	rawContent, _ := os.ReadFile(tmpFile)
	t.Logf("Raw patch file content (%d bytes): %q", len(rawContent), string(rawContent))
	
	// Test git apply command directly
	cmd := exec.Command("git", "apply", "--check", tmpFile)
	cmd.Dir = workspaceRoot
	output, err := cmd.CombinedOutput()
	
	t.Logf("Git apply --check output: %s", string(output))
	if err != nil {
		t.Logf("Git apply --check failed: %v", err)
		
		// Let's also try without --check to see the actual error
		cmd2 := exec.Command("git", "apply", "--reject", "--whitespace=nowarn", tmpFile)
		cmd2.Dir = workspaceRoot
		output2, err2 := cmd2.CombinedOutput()
		t.Logf("Git apply output: %s", string(output2))
		t.Logf("Git apply error: %v", err2)
	}
} 