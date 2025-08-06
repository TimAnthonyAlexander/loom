package models

import (
	"time"
)

// Message represents a chat message between user and AI
type Message struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	IsUser    bool      `json:"isUser"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`    // "user", "assistant", "system", "debug"
	Visible   bool      `json:"visible"` // Whether the message should be displayed in the UI
}

// ChatState represents the current state of the chat session
type ChatState struct {
	Messages         []Message `json:"messages"`
	IsStreaming      bool      `json:"isStreaming"`
	StreamingContent string    `json:"streamingContent"`
	SessionID        string    `json:"sessionId"`
	WorkspacePath    string    `json:"workspacePath"`
}

// FileInfo represents file information for the frontend
type FileInfo struct {
	Path         string    `json:"path"`
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	IsDirectory  bool      `json:"isDirectory"`
	Language     string    `json:"language"`
	ModifiedTime time.Time `json:"modifiedTime"`
}

// ProjectSummary represents the AI-generated project overview
type ProjectSummary struct {
	Summary     string             `json:"summary"`
	Languages   map[string]float64 `json:"languages"`
	FileCount   int                `json:"fileCount"`
	TotalLines  int                `json:"totalLines"`
	GeneratedAt time.Time          `json:"generatedAt"`
}
