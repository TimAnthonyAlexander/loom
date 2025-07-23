package undo

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewUndoManager(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom-undo-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create an undo manager
	manager, err := NewUndoManager(tempDir)
	if err != nil {
		t.Fatalf("Failed to create undo manager: %v", err)
	}

	if manager == nil {
		t.Fatalf("Expected non-nil undo manager")
	}

	if manager.stack == nil {
		t.Fatalf("Expected non-nil undo stack")
	}

	if manager.stack.workspacePath != tempDir {
		t.Errorf("Expected workspace path %s, got %s", tempDir, manager.stack.workspacePath)
	}

	if manager.stack.maxActions != 50 {
		t.Errorf("Expected max actions to be 50, got %d", manager.stack.maxActions)
	}
}

func TestRecordFileEdit(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom-undo-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFile := "test.txt"
	testFilePath := filepath.Join(tempDir, testFile)
	if err := os.WriteFile(testFilePath, []byte("original content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create an undo manager
	manager, err := NewUndoManager(tempDir)
	if err != nil {
		t.Fatalf("Failed to create undo manager: %v", err)
	}

	// Record a file edit
	action, err := manager.RecordFileEdit(testFile, "Edit test file")
	if err != nil {
		t.Fatalf("Failed to record file edit: %v", err)
	}

	if action == nil {
		t.Fatalf("Expected non-nil undo action")
	}

	if action.Type != "file_edit" {
		t.Errorf("Expected action type 'file_edit', got '%s'", action.Type)
	}

	if action.FilePath != testFile {
		t.Errorf("Expected file path '%s', got '%s'", testFile, action.FilePath)
	}

	if action.Description != "Edit test file" {
		t.Errorf("Expected description 'Edit test file', got '%s'", action.Description)
	}

	if action.Applied {
		t.Errorf("Expected action to not be applied initially")
	}

	if action.Undone {
		t.Errorf("Expected action to not be undone initially")
	}

	if action.BackupPath == "" {
		t.Errorf("Expected non-empty backup path")
	}

	// Check that backup file exists
	if _, err := os.Stat(action.BackupPath); os.IsNotExist(err) {
		t.Errorf("Backup file does not exist: %s", action.BackupPath)
	}

	// Mark action as applied
	err = manager.MarkApplied(action.ID)
	if err != nil {
		t.Fatalf("Failed to mark action as applied: %v", err)
	}

	// Check that action was marked as applied
	for _, a := range manager.stack.actions {
		if a.ID == action.ID {
			if !a.Applied {
				t.Errorf("Expected action to be marked as applied")
			}
			break
		}
	}
}

func TestRecordFileCreate(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom-undo-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create an undo manager
	manager, err := NewUndoManager(tempDir)
	if err != nil {
		t.Fatalf("Failed to create undo manager: %v", err)
	}

	// Record a file creation
	newFile := "new_file.txt"
	action, err := manager.RecordFileCreate(newFile, "Create new file")
	if err != nil {
		t.Fatalf("Failed to record file create: %v", err)
	}

	if action == nil {
		t.Fatalf("Expected non-nil undo action")
	}

	if action.Type != "file_create" {
		t.Errorf("Expected action type 'file_create', got '%s'", action.Type)
	}

	if action.FilePath != newFile {
		t.Errorf("Expected file path '%s', got '%s'", newFile, action.FilePath)
	}

	// No backup needed for file creation
	if action.BackupPath != "" {
		t.Errorf("Expected empty backup path for file creation, got '%s'", action.BackupPath)
	}

	// Mark action as applied
	err = manager.MarkApplied(action.ID)
	if err != nil {
		t.Fatalf("Failed to mark action as applied: %v", err)
	}
}

func TestRecordFileDelete(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom-undo-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file to be deleted
	testFile := "delete_me.txt"
	testFilePath := filepath.Join(tempDir, testFile)
	if err := os.WriteFile(testFilePath, []byte("delete me"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create an undo manager
	manager, err := NewUndoManager(tempDir)
	if err != nil {
		t.Fatalf("Failed to create undo manager: %v", err)
	}

	// Record a file deletion
	action, err := manager.RecordFileDelete(testFile, "Delete test file")
	if err != nil {
		t.Fatalf("Failed to record file delete: %v", err)
	}

	if action == nil {
		t.Fatalf("Expected non-nil undo action")
	}

	if action.Type != "file_delete" {
		t.Errorf("Expected action type 'file_delete', got '%s'", action.Type)
	}

	if action.FilePath != testFile {
		t.Errorf("Expected file path '%s', got '%s'", testFile, action.FilePath)
	}

	// Backup is needed for file deletion
	if action.BackupPath == "" {
		t.Errorf("Expected non-empty backup path for file deletion")
	}

	// Check that backup file exists
	if _, err := os.Stat(action.BackupPath); os.IsNotExist(err) {
		t.Errorf("Backup file does not exist: %s", action.BackupPath)
	}

	// Mark action as applied
	err = manager.MarkApplied(action.ID)
	if err != nil {
		t.Fatalf("Failed to mark action as applied: %v", err)
	}
}

func TestUndoFileCRUD(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom-undo-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create an undo manager
	manager, err := NewUndoManager(tempDir)
	if err != nil {
		t.Fatalf("Failed to create undo manager: %v", err)
	}

	// Create a test file
	testFile := "test_undo.txt"
	testFilePath := filepath.Join(tempDir, testFile)
	originalContent := "original content"
	if err := os.WriteFile(testFilePath, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test file edit and undo
	editAction, err := manager.RecordFileEdit(testFile, "Edit test file")
	if err != nil {
		t.Fatalf("Failed to record file edit: %v", err)
	}

	// Mark as applied
	if err := manager.MarkApplied(editAction.ID); err != nil {
		t.Fatalf("Failed to mark action as applied: %v", err)
	}

	// Modify the file
	modifiedContent := "modified content"
	if err := os.WriteFile(testFilePath, []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	// Get content before undo
	contentBeforeUndo, err := os.ReadFile(testFilePath)
	if err != nil {
		t.Fatalf("Failed to read file before undo: %v", err)
	}
	if string(contentBeforeUndo) != modifiedContent {
		t.Errorf("Expected content '%s' before undo, got '%s'", modifiedContent, string(contentBeforeUndo))
	}

	// Undo the edit
	undoAction, err := manager.UndoAction(editAction.ID)
	if err != nil {
		t.Fatalf("Failed to undo file edit: %v", err)
	}

	if !undoAction.Undone {
		t.Errorf("Expected action to be marked as undone")
	}

	// Check file content after undo
	contentAfterUndo, err := os.ReadFile(testFilePath)
	if err != nil {
		t.Fatalf("Failed to read file after undo: %v", err)
	}
	if string(contentAfterUndo) != originalContent {
		t.Errorf("Expected content '%s' after undo, got '%s'", originalContent, string(contentAfterUndo))
	}

	// Test UndoLast convenience method
	// First create a new file
	newFile := "new_file.txt"
	newFilePath := filepath.Join(tempDir, newFile)
	if err := os.WriteFile(newFilePath, []byte("new content"), 0644); err != nil {
		t.Fatalf("Failed to create new file: %v", err)
	}

	// Record file create
	createAction, err := manager.RecordFileCreate(newFile, "Create new file")
	if err != nil {
		t.Fatalf("Failed to record file create: %v", err)
	}

	// Mark as applied
	if err := manager.MarkApplied(createAction.ID); err != nil {
		t.Fatalf("Failed to mark action as applied: %v", err)
	}

	// Undo last action (which is the file create)
	lastAction, err := manager.UndoLast()
	if err != nil {
		t.Fatalf("Failed to undo last action: %v", err)
	}

	if lastAction.ID != createAction.ID {
		t.Errorf("Expected to undo the file create action, but undid %s", lastAction.Type)
	}

	if !lastAction.Undone {
		t.Errorf("Expected action to be marked as undone")
	}

	// Check that file was deleted by the undo
	if _, err := os.Stat(newFilePath); !os.IsNotExist(err) {
		t.Errorf("Expected file to be deleted by undo")
	}
}

func TestUndoStack(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom-undo-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create an undo manager
	manager, err := NewUndoManager(tempDir)
	if err != nil {
		t.Fatalf("Failed to create undo manager: %v", err)
	}

	// Add several actions to the stack
	for i := 0; i < 5; i++ {
		action := &UndoAction{
			ID:          generateActionID(),
			Type:        "file_edit",
			FilePath:    "test.txt",
			Description: "Test action",
			Timestamp:   time.Now(),
			Applied:     true,
			Undone:      false,
		}
		manager.stack.push(action)
	}

	// Get undo history
	history := manager.GetUndoHistory()
	if len(history) != 5 {
		t.Errorf("Expected 5 actions in history, got %d", len(history))
	}

	// Test GetLastUndoableAction
	lastUndoable := manager.GetLastUndoableAction()
	if lastUndoable == nil {
		t.Fatalf("Expected non-nil last undoable action")
	}

	// Mark the action as undone
	lastUndoable.Undone = true

	// Now there should be a different last undoable action
	newLastUndoable := manager.GetLastUndoableAction()
	if newLastUndoable == nil {
		t.Fatalf("Expected non-nil new last undoable action")
	}
	if newLastUndoable.ID == lastUndoable.ID {
		t.Errorf("Expected different last undoable action after marking previous one as undone")
	}
}

func TestCreateBackup(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom-undo-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFile := "backup_test.txt"
	testFilePath := filepath.Join(tempDir, testFile)
	testContent := "content to backup"
	if err := os.WriteFile(testFilePath, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create an undo manager
	manager, err := NewUndoManager(tempDir)
	if err != nil {
		t.Fatalf("Failed to create undo manager: %v", err)
	}

	// Create backup
	backupPath, err := manager.createBackup(testFile)
	if err != nil {
		t.Fatalf("Failed to create backup: %v", err)
	}

	if backupPath == "" {
		t.Fatalf("Expected non-empty backup path")
	}

	// Check that backup file exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Errorf("Backup file does not exist: %s", backupPath)
	}

	// Check backup content
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("Failed to read backup file: %v", err)
	}
	if string(backupContent) != testContent {
		t.Errorf("Expected backup content '%s', got '%s'", testContent, string(backupContent))
	}

	// Test backup of non-existent file
	nonExistentFile := "non_existent.txt"
	nonExistentPath, err := manager.createBackup(nonExistentFile)
	if err != nil {
		t.Fatalf("Failed to handle non-existent file: %v", err)
	}
	if nonExistentPath != "" {
		t.Errorf("Expected empty backup path for non-existent file, got '%s'", nonExistentPath)
	}
}
