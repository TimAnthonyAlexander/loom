package undo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// UndoAction represents a single action that can be undone
type UndoAction struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"` // "file_edit", "file_create", "file_delete"
	FilePath    string    `json:"file_path"`
	BackupPath  string    `json:"backup_path"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
	Applied     bool      `json:"applied"`
	Undone      bool      `json:"undone"`
}

// UndoStack represents a stack of undo actions
type UndoStack struct {
	workspacePath string
	undoDir       string
	actions       []*UndoAction
	maxActions    int
}

// UndoManager manages undo operations for the workspace
type UndoManager struct {
	stack *UndoStack
}

// NewUndoManager creates a new undo manager
func NewUndoManager(workspacePath string) (*UndoManager, error) {
	// Create undo directory
	undoDir := filepath.Join(workspacePath, ".loom", "undo")
	if err := os.MkdirAll(undoDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create undo directory: %w", err)
	}

	stack := &UndoStack{
		workspacePath: workspacePath,
		undoDir:       undoDir,
		actions:       make([]*UndoAction, 0),
		maxActions:    50, // Keep last 50 undo actions
	}

	// Load existing undo stack
	if err := stack.load(); err != nil {
		// If loading fails, start with empty stack
		fmt.Printf("Warning: failed to load undo stack: %v\n", err)
	}

	return &UndoManager{
		stack: stack,
	}, nil
}

// RecordFileEdit records a file edit action for potential undo
func (um *UndoManager) RecordFileEdit(filePath string, description string) (*UndoAction, error) {
	// Create backup of current file content
	backupPath, err := um.createBackup(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup: %w", err)
	}

	action := &UndoAction{
		ID:          generateActionID(),
		Type:        "file_edit",
		FilePath:    filePath,
		BackupPath:  backupPath,
		Description: description,
		Timestamp:   time.Now(),
		Applied:     false,
		Undone:      false,
	}

	um.stack.push(action)
	return action, nil
}

// RecordFileCreate records a file creation action for potential undo
func (um *UndoManager) RecordFileCreate(filePath string, description string) (*UndoAction, error) {
	action := &UndoAction{
		ID:          generateActionID(),
		Type:        "file_create",
		FilePath:    filePath,
		BackupPath:  "", // No backup needed for new files
		Description: description,
		Timestamp:   time.Now(),
		Applied:     false,
		Undone:      false,
	}

	um.stack.push(action)
	return action, nil
}

// RecordFileDelete records a file deletion action for potential undo
func (um *UndoManager) RecordFileDelete(filePath string, description string) (*UndoAction, error) {
	// Create backup of file being deleted
	backupPath, err := um.createBackup(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup before deletion: %w", err)
	}

	action := &UndoAction{
		ID:          generateActionID(),
		Type:        "file_delete",
		FilePath:    filePath,
		BackupPath:  backupPath,
		Description: description,
		Timestamp:   time.Now(),
		Applied:     false,
		Undone:      false,
	}

	um.stack.push(action)
	return action, nil
}

// MarkApplied marks an action as applied
func (um *UndoManager) MarkApplied(actionID string) error {
	action := um.stack.findAction(actionID)
	if action == nil {
		return fmt.Errorf("action %s not found", actionID)
	}

	action.Applied = true
	return um.stack.save()
}

// UndoLast undoes the last applied action
func (um *UndoManager) UndoLast() (*UndoAction, error) {
	// Find the most recent applied action that hasn't been undone
	var lastAction *UndoAction
	for i := len(um.stack.actions) - 1; i >= 0; i-- {
		action := um.stack.actions[i]
		if action.Applied && !action.Undone {
			lastAction = action
			break
		}
	}

	if lastAction == nil {
		return nil, fmt.Errorf("no actions to undo")
	}

	return um.UndoAction(lastAction.ID)
}

// UndoAction undoes a specific action by ID
func (um *UndoManager) UndoAction(actionID string) (*UndoAction, error) {
	action := um.stack.findAction(actionID)
	if action == nil {
		return nil, fmt.Errorf("action %s not found", actionID)
	}

	if !action.Applied {
		return nil, fmt.Errorf("action %s has not been applied", actionID)
	}

	if action.Undone {
		return nil, fmt.Errorf("action %s has already been undone", actionID)
	}

	// Perform the undo based on action type
	switch action.Type {
	case "file_edit":
		if err := um.undoFileEdit(action); err != nil {
			return nil, fmt.Errorf("failed to undo file edit: %w", err)
		}
	case "file_create":
		if err := um.undoFileCreate(action); err != nil {
			return nil, fmt.Errorf("failed to undo file create: %w", err)
		}
	case "file_delete":
		if err := um.undoFileDelete(action); err != nil {
			return nil, fmt.Errorf("failed to undo file delete: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown action type: %s", action.Type)
	}

	action.Undone = true
	if err := um.stack.save(); err != nil {
		return nil, fmt.Errorf("failed to save undo stack: %w", err)
	}

	return action, nil
}

// undoFileEdit reverts a file edit by restoring from backup
func (um *UndoManager) undoFileEdit(action *UndoAction) error {
	if action.BackupPath == "" {
		return fmt.Errorf("no backup available for file edit")
	}

	// Check if backup exists
	if _, err := os.Stat(action.BackupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s", action.BackupPath)
	}

	// Get full path
	fullPath := filepath.Join(um.stack.workspacePath, action.FilePath)

	// Read backup content
	backupContent, err := os.ReadFile(action.BackupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	// Restore file from backup
	if err := os.WriteFile(fullPath, backupContent, 0644); err != nil {
		return fmt.Errorf("failed to restore file: %w", err)
	}

	return nil
}

// undoFileCreate deletes a created file
func (um *UndoManager) undoFileCreate(action *UndoAction) error {
	fullPath := filepath.Join(um.stack.workspacePath, action.FilePath)

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		// File doesn't exist, nothing to undo
		return nil
	}

	// Delete the file
	if err := os.Remove(fullPath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// undoFileDelete restores a deleted file from backup
func (um *UndoManager) undoFileDelete(action *UndoAction) error {
	if action.BackupPath == "" {
		return fmt.Errorf("no backup available for file deletion")
	}

	// Check if backup exists
	if _, err := os.Stat(action.BackupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s", action.BackupPath)
	}

	fullPath := filepath.Join(um.stack.workspacePath, action.FilePath)

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Read backup content
	backupContent, err := os.ReadFile(action.BackupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	// Restore file from backup
	if err := os.WriteFile(fullPath, backupContent, 0644); err != nil {
		return fmt.Errorf("failed to restore file: %w", err)
	}

	return nil
}

// GetUndoHistory returns the undo history
func (um *UndoManager) GetUndoHistory() []*UndoAction {
	// Return a copy of the actions slice
	history := make([]*UndoAction, len(um.stack.actions))
	copy(history, um.stack.actions)
	return history
}

// GetLastUndoableAction returns the most recent action that can be undone
func (um *UndoManager) GetLastUndoableAction() *UndoAction {
	for i := len(um.stack.actions) - 1; i >= 0; i-- {
		action := um.stack.actions[i]
		if action.Applied && !action.Undone {
			return action
		}
	}
	return nil
}

// Cleanup removes old backup files and actions
func (um *UndoManager) Cleanup() error {
	// Remove actions older than 30 days
	cutoff := time.Now().AddDate(0, 0, -30)

	var newActions []*UndoAction
	for _, action := range um.stack.actions {
		if action.Timestamp.After(cutoff) {
			newActions = append(newActions, action)
		} else {
			// Remove backup file if it exists
			if action.BackupPath != "" {
				os.Remove(action.BackupPath) // Ignore errors
			}
		}
	}

	um.stack.actions = newActions
	return um.stack.save()
}

// createBackup creates a backup of a file
func (um *UndoManager) createBackup(filePath string) (string, error) {
	fullPath := filepath.Join(um.stack.workspacePath, filePath)

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		// File doesn't exist, no backup needed
		return "", nil
	}

	// Read file content
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file for backup: %w", err)
	}

	// Create backup file path
	timestamp := time.Now().Format("20060102_150405_000")
	cleanPath := filepath.Clean(filePath)
	cleanPath = filepath.ToSlash(cleanPath) // Normalize separators
	cleanPath = strings.ReplaceAll(cleanPath, "/", "_")
	cleanPath = strings.ReplaceAll(cleanPath, "\\", "_")

	backupName := fmt.Sprintf("%s_%s.backup", cleanPath, timestamp)
	backupPath := filepath.Join(um.stack.undoDir, backupName)

	// Write backup file
	if err := os.WriteFile(backupPath, content, 0644); err != nil {
		return "", fmt.Errorf("failed to write backup file: %w", err)
	}

	return backupPath, nil
}

// UndoStack methods

// push adds an action to the stack
func (us *UndoStack) push(action *UndoAction) {
	us.actions = append(us.actions, action)

	// Trim if we exceed max actions
	if len(us.actions) > us.maxActions {
		// Remove oldest actions and their backup files
		excess := us.actions[:len(us.actions)-us.maxActions]
		for _, oldAction := range excess {
			if oldAction.BackupPath != "" {
				os.Remove(oldAction.BackupPath) // Ignore errors
			}
		}
		us.actions = us.actions[len(us.actions)-us.maxActions:]
	}

	us.save() // Auto-save
}

// findAction finds an action by ID
func (us *UndoStack) findAction(actionID string) *UndoAction {
	for _, action := range us.actions {
		if action.ID == actionID {
			return action
		}
	}
	return nil
}

// save saves the undo stack to disk
func (us *UndoStack) save() error {
	stackFile := filepath.Join(us.undoDir, "stack.json")

	data, err := json.MarshalIndent(us.actions, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal undo stack: %w", err)
	}

	if err := os.WriteFile(stackFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write undo stack: %w", err)
	}

	return nil
}

// load loads the undo stack from disk
func (us *UndoStack) load() error {
	stackFile := filepath.Join(us.undoDir, "stack.json")

	// Check if file exists
	if _, err := os.Stat(stackFile); os.IsNotExist(err) {
		// No existing stack, start fresh
		return nil
	}

	data, err := os.ReadFile(stackFile)
	if err != nil {
		return fmt.Errorf("failed to read undo stack: %w", err)
	}

	if err := json.Unmarshal(data, &us.actions); err != nil {
		return fmt.Errorf("failed to unmarshal undo stack: %w", err)
	}

	return nil
}

// generateActionID generates a unique ID for an undo action
func generateActionID() string {
	return fmt.Sprintf("undo_%d", time.Now().UnixNano())
}
