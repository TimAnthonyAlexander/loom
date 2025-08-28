package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestApplyEditWithVerification demonstrates the new verification feature
func TestApplyEditWithVerification(t *testing.T) {
	workspace := t.TempDir()
	registry := NewRegistry()

	// Register both tools
	if err := RegisterEditFile(registry, workspace); err != nil {
		t.Fatalf("Failed to register edit_file: %v", err)
	}
	if err := RegisterApplyEdit(registry, workspace); err != nil {
		t.Fatalf("Failed to register apply_edit: %v", err)
	}

	// Create initial JSON file
	originalJSON := `{
    "name": "test project",
    "version": "1.0.0",
    "dependencies": {
        "lodash": "^4.17.21"
    }
}`

	testFile := filepath.Join(workspace, "package.json")
	if err := os.WriteFile(testFile, []byte(originalJSON), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	t.Logf("📄 Original file:")
	t.Logf("%s", originalJSON)

	// Test the exact scenario that had the duplication bug (but now it's fixed)
	editArgs := EditFileArgs{
		Path:         "package.json",
		Action:       "ANCHOR_REPLACE",
		AnchorBefore: "    \"dependencies\": {",
		AnchorAfter:  "    }",
		Content:      "        \"lodash\": \"^4.17.21\",\n        \"axios\": \"^1.6.0\"",
	}

	// First, propose the edit
	editArgsBytes, _ := json.Marshal(editArgs)
	editResult, err := registry.Invoke(context.Background(), "edit_file", editArgsBytes)
	if err != nil {
		t.Fatalf("edit_file failed: %v", err)
	}

	execResult := editResult.(*ExecutionResult)
	t.Logf("\n📝 Edit proposal created successfully")
	t.Logf("   Safe: %v (requires approval)", execResult.Safe)

	// Now apply the edit (simulate approval)
	applyArgs := ApplyEditArgs{
		Path:         editArgs.Path,
		Action:       editArgs.Action,
		AnchorBefore: editArgs.AnchorBefore,
		AnchorAfter:  editArgs.AnchorAfter,
		Content:      editArgs.Content,
	}

	applyArgsBytes, _ := json.Marshal(applyArgs)
	applyResult, err := registry.Invoke(context.Background(), "apply_edit", applyArgsBytes)
	if err != nil {
		t.Fatalf("apply_edit failed: %v", err)
	}

	applyExecResult := applyResult.(*ExecutionResult)
	t.Logf("\n✅ Edit applied successfully!")
	t.Logf("\n📋 Complete verification output:")
	t.Logf("%s", applyExecResult.Content)

	// Verify the content was actually written correctly
	finalContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read final file: %v", err)
	}

	t.Logf("\n📄 Final file content:")
	t.Logf("%s", string(finalContent))

	// Check that verification includes expected elements
	verification := applyExecResult.Content
	if !strings.Contains(verification, "Edit Verification") {
		t.Errorf("Verification should contain 'Edit Verification' header")
	}
	if !strings.Contains(verification, "Changes Applied Successfully") {
		t.Errorf("Verification should confirm changes were applied")
	}
	if !strings.Contains(verification, "Context:") {
		t.Errorf("Verification should show context around changes")
	}
	if !strings.Contains(verification, "Summary:") {
		t.Errorf("Verification should include a summary")
	}

	// Verify the diff is also available in the Diff field
	if applyExecResult.Diff == "" {
		t.Errorf("ExecutionResult.Diff should contain verification diff")
	}

	// Verify no duplication occurred (the original bug)
	if strings.Count(string(finalContent), `"lodash": "^4.17.21"`) != 1 {
		t.Errorf("Duplication bug detected! lodash should appear exactly once")
	}

	// Verify axios was added
	if !strings.Contains(string(finalContent), `"axios": "^1.6.0"`) {
		t.Errorf("axios dependency should have been added")
	}

	t.Logf("\n🎉 All verification checks passed!")
}

// TestApplyEditVerificationFileCreation tests verification for file creation
func TestApplyEditVerificationFileCreation(t *testing.T) {
	workspace := t.TempDir()
	registry := NewRegistry()

	if err := RegisterEditFile(registry, workspace); err != nil {
		t.Fatalf("Failed to register edit_file: %v", err)
	}
	if err := RegisterApplyEdit(registry, workspace); err != nil {
		t.Fatalf("Failed to register apply_edit: %v", err)
	}

	// Create a new file
	content := `# My New Project

This is a test file created by the LLM.

## Features
- Feature 1
- Feature 2
`

	editArgs := EditFileArgs{
		Path:    "README.md",
		Action:  "CREATE",
		Content: content,
	}

	// Propose creation
	editArgsBytes, _ := json.Marshal(editArgs)
	_, err := registry.Invoke(context.Background(), "edit_file", editArgsBytes)
	if err != nil {
		t.Fatalf("edit_file failed: %v", err)
	}

	// Apply creation
	applyArgs := ApplyEditArgs{
		Path:    editArgs.Path,
		Action:  editArgs.Action,
		Content: editArgs.Content,
	}

	applyArgsBytes, _ := json.Marshal(applyArgs)
	applyResult, err := registry.Invoke(context.Background(), "apply_edit", applyArgsBytes)
	if err != nil {
		t.Fatalf("apply_edit failed: %v", err)
	}

	applyExecResult := applyResult.(*ExecutionResult)
	t.Logf("File creation verification:")
	t.Logf("%s", applyExecResult.Content)

	// Check that file creation verification shows content preview
	verification := applyExecResult.Content
	if !strings.Contains(verification, "File Created Successfully") {
		t.Errorf("Verification should indicate file was created")
	}
	if !strings.Contains(verification, "Content preview:") {
		t.Errorf("Verification should show content preview for new files")
	}

	// Verify file actually exists and has correct content
	testFile := filepath.Join(workspace, "README.md")
	actualContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Created file should exist: %v", err)
	}

	if string(actualContent) != content {
		t.Errorf("File content doesn't match expected")
	}

	t.Logf("✅ File creation verification working correctly!")
}
