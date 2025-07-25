package undo

import (
	"loom/paths"
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
	projectPaths  *paths.ProjectPaths
	actions       []*UndoAction
	maxActions    int
}

// UndoManager manages undo operations for the workspace
type UndoManager struct {
	stack *UndoStack
}

// NewUndoManager creates a new undo manager

// actionCounter is a monotonically increasing counter used to guarantee
// unique undo action IDs even when multiple actions are created within the
// same nanosecond on platforms with coarse time resolution (e.g. Windows).
// It must be accessed atomically since undo actions can potentially be
// created from concurrent goroutines.
var actionCounter uint64
