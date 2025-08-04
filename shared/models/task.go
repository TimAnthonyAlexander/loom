package models

import (
	taskPkg "loom/task"
	"time"
)

// TaskInfo represents task information for the frontend
type TaskInfo struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"`
	Description string     `json:"description"`
	Status      string     `json:"status"` // "pending", "executing", "completed", "failed"
	CreatedAt   time.Time  `json:"createdAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	Error       string     `json:"error,omitempty"`
	Preview     string     `json:"preview,omitempty"`
	Result      string     `json:"result,omitempty"`
	// Reference to the underlying task
	Task     *taskPkg.Task         `json:"-"`
	Response *taskPkg.TaskResponse `json:"-"`
}

// TaskConfirmation represents a task waiting for user confirmation
type TaskConfirmation struct {
	TaskInfo TaskInfo `json:"taskInfo"`
	Preview  string   `json:"preview"`
	Approved bool     `json:"approved"`
}

// TaskExecutionEvent represents real-time task execution updates
type TaskExecutionEvent struct {
	TaskID    string    `json:"taskId"`
	Type      string    `json:"type"` // "started", "progress", "completed", "failed", "confirmation_needed"
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Progress  float64   `json:"progress,omitempty"` // 0.0 to 1.0
}
