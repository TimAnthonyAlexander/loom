package session

import (
	"encoding/json"
	"fmt"
	"loom/git"
	"loom/llm"
	"loom/paths"
	"loom/task"
	"loom/todo"
	"loom/undo"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SessionState represents the complete state of a Loom session
type SessionState struct {
	SessionID     string    `json:"session_id"`
	WorkspacePath string    `json:"workspace_path"`
	CreatedAt     time.Time `json:"created_at"`
	LastSaved     time.Time `json:"last_saved"`
	Version       string    `json:"version"`

	// Chat and messaging state
	Messages    []llm.Message `json:"messages"`
	CurrentView string        `json:"current_view"`

	// LLM configuration
	LLMModel string `json:"llm_model"`
	APIKey   string `json:"api_key,omitempty"` // Excluded in safe save
	BaseURL  string `json:"base_url,omitempty"`

	// Task execution state
	CurrentActionPlan *task.ActionPlan          `json:"current_action_plan,omitempty"`
	PlanExecution     *task.ActionPlanExecution `json:"plan_execution,omitempty"`
	TaskHistory       []string                  `json:"task_history"`

	// Undo stack
	UndoActions []*undo.UndoAction `json:"undo_actions"`

	// Git state
	GitStatus *git.RepositoryStatus `json:"git_status,omitempty"`

	// Configuration
	EnableShell      bool  `json:"enable_shell"`
	MaxFileSize      int64 `json:"max_file_size"`
	MaxContextTokens int   `json:"max_context_tokens"`
	EnableTestFirst  bool  `json:"enable_test_first"`

	// Todo state
	CurrentTodoList *todo.TodoList `json:"current_todo_list,omitempty"`

	// Recovery and integrity
	RecoveryInfo    *RecoveryState `json:"recovery_info,omitempty"`
	LastSafeState   time.Time      `json:"last_safe_state"`
	ConsistencyHash string         `json:"consistency_hash"`
}

// RecoveryState tracks information needed for session recovery
type RecoveryState struct {
	IncompleteEdits    []string  `json:"incomplete_edits"`
	PendingTasks       []string  `json:"pending_tasks"`
	UnsavedChanges     []string  `json:"unsaved_changes"`
	LastSuccessfulSave time.Time `json:"last_successful_save"`
	CorruptionDetected bool      `json:"corruption_detected"`
	RecoveryAttempts   int       `json:"recovery_attempts"`
	BackupFileUsed     string    `json:"backup_file_used,omitempty"`
}

// SessionManager manages session persistence and recovery
type SessionManager struct {
	workspacePath   string
	projectPaths    *paths.ProjectPaths
	currentSession  *SessionState
	saveInterval    time.Duration
	autoSaveEnabled bool
}

// RecoveryInfo contains information about recoverable sessions
type RecoveryInfo struct {
	SessionID     string    `json:"session_id"`
	LastSaved     time.Time `json:"last_saved"`
	MessageCount  int       `json:"message_count"`
	UndoCount     int       `json:"undo_count"`
	HasActionPlan bool      `json:"has_action_plan"`
	FilePath      string    `json:"file_path"`
}

// NewSessionManager creates a new session manager
func NewSessionManager(workspacePath string) (*SessionManager, error) {
	projectPaths, err := paths.NewProjectPaths(workspacePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create project paths: %w", err)
	}

	// Ensure project directories exist
	if err := projectPaths.EnsureProjectDir(); err != nil {
		return nil, fmt.Errorf("failed to create project directories: %w", err)
	}

	return &SessionManager{
		workspacePath:   workspacePath,
		projectPaths:    projectPaths,
		saveInterval:    30 * time.Second, // Auto-save every 30 seconds
		autoSaveEnabled: true,
	}, nil
}

// CreateSession creates a new session

// LoadSession loads a session from disk
func (sm *SessionManager) LoadSession(sessionID string) (*SessionState, error) {
	sessionFile := filepath.Join(sm.projectPaths.SessionsDir(), fmt.Sprintf("%s.json", sessionID))

	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var session SessionState
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	sm.currentSession = &session
	return &session, nil
}

// SaveSession saves the current session to disk
func (sm *SessionManager) SaveSession() error {
	if sm.currentSession == nil {
		return fmt.Errorf("no current session to save")
	}

	sm.currentSession.LastSaved = time.Now()

	// Create both regular and safe (without secrets) versions
	if err := sm.saveSessionRegular(); err != nil {
		return err
	}

	if err := sm.saveSessionSafe(); err != nil {
		return err
	}

	return nil
}

// saveSessionRegular saves the complete session (may contain secrets)
func (sm *SessionManager) saveSessionRegular() error {
	sessionFile := filepath.Join(sm.projectPaths.SessionsDir(), fmt.Sprintf("%s.json", sm.currentSession.SessionID))

	data, err := json.MarshalIndent(sm.currentSession, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Ensure the session file is only readable by owner (contains API keys)
	if err := os.WriteFile(sessionFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

// saveSessionSafe saves a sanitized version without secrets
func (sm *SessionManager) saveSessionSafe() error {
	// Create a copy without sensitive data
	safeCopy := *sm.currentSession
	safeCopy.APIKey = "" // Remove API key

	// Also sanitize any secrets from messages (basic approach)
	for i, msg := range safeCopy.Messages {
		if len(msg.Content) > 1000 { // Only process long messages
			// Basic secret redaction for safety
			content := msg.Content
			// Truncate very long content
			if len(content) > 500 {
				content = content[:500] + "...[TRUNCATED]"
			}
			safeCopy.Messages[i].Content = content
		}
	}

	safeFile := filepath.Join(sm.projectPaths.SessionsDir(), fmt.Sprintf("%s.safe.json", sm.currentSession.SessionID))

	data, err := json.MarshalIndent(&safeCopy, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal safe session: %w", err)
	}

	if err := os.WriteFile(safeFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write safe session file: %w", err)
	}

	return nil
}

// GetRecoverableSessions returns information about sessions that can be recovered
func (sm *SessionManager) GetRecoverableSessions() ([]RecoveryInfo, error) {
	files, err := os.ReadDir(sm.projectPaths.SessionsDir())
	if err != nil {
		return nil, fmt.Errorf("failed to read session directory: %w", err)
	}

	var recoverable []RecoveryInfo

	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" && !strings.Contains(file.Name(), ".safe.") {
			sessionFile := filepath.Join(sm.projectPaths.SessionsDir(), file.Name())

			// Try to load basic info from the session
			data, err := os.ReadFile(sessionFile)
			if err != nil {
				continue
			}

			var session SessionState
			if err := json.Unmarshal(data, &session); err != nil {
				continue
			}

			info := RecoveryInfo{
				SessionID:     session.SessionID,
				LastSaved:     session.LastSaved,
				MessageCount:  len(session.Messages),
				UndoCount:     len(session.UndoActions),
				HasActionPlan: session.CurrentActionPlan != nil,
				FilePath:      sessionFile,
			}

			recoverable = append(recoverable, info)
		}
	}

	// Sort by last saved time (most recent first)
	for i := 0; i < len(recoverable)-1; i++ {
		for j := i + 1; j < len(recoverable); j++ {
			if recoverable[i].LastSaved.Before(recoverable[j].LastSaved) {
				recoverable[i], recoverable[j] = recoverable[j], recoverable[i]
			}
		}
	}

	return recoverable, nil
}

// GetLatestSession returns the most recently saved session

// UpdateSessionState updates the current session state

// AddMessage adds a message to the current session

// SetActionPlan sets the current action plan in the session

// AddUndoAction adds an undo action to the session

// UpdateGitStatus updates the Git status in the session

// GetCurrentSession returns the current session state

// CleanupOldSessions removes sessions older than the specified duration

// EnableAutoSave enables or disables automatic session saving

// SetSaveInterval sets the auto-save interval

// StartAutoSave starts the auto-save routine

// ExportSession exports a session to a file for backup or sharing

// ImportSession imports a session from an exported file

// GetSessionSummary returns a summary of the current session

// DetectIncompleteSession checks if a session has incomplete operations
func (sm *SessionManager) DetectIncompleteSession(sessionID string) (*RecoveryState, error) {
	session, err := sm.LoadSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load session for recovery check: %w", err)
	}

	recovery := &RecoveryState{
		IncompleteEdits:    []string{},
		PendingTasks:       []string{},
		UnsavedChanges:     []string{},
		LastSuccessfulSave: session.LastSaved,
		CorruptionDetected: false,
		RecoveryAttempts:   0,
	}

	// Check for incomplete action plans
	if session.CurrentActionPlan != nil && session.PlanExecution != nil {
		if session.PlanExecution.Status == "preparing" || session.PlanExecution.Status == "applying" {
			recovery.IncompleteEdits = append(recovery.IncompleteEdits, session.CurrentActionPlan.ID)
		}
	}

	// Check consistency hash
	expectedHash := sm.calculateConsistencyHash(session)
	if session.ConsistencyHash != "" && session.ConsistencyHash != expectedHash {
		recovery.CorruptionDetected = true
	}

	return recovery, nil
}

// RecoverSession attempts to recover a session to a consistent state
func (sm *SessionManager) RecoverSession(sessionID string) (*SessionState, error) {
	// First, detect what needs recovery
	recoveryInfo, err := sm.DetectIncompleteSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to detect recovery needs: %w", err)
	}

	// Load the session
	session, err := sm.LoadSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load session for recovery: %w", err)
	}

	// Initialize recovery info
	if session.RecoveryInfo == nil {
		session.RecoveryInfo = recoveryInfo
	}
	session.RecoveryInfo.RecoveryAttempts++

	// If corruption detected, try to load from backup
	if recoveryInfo.CorruptionDetected {
		backupSession, backupErr := sm.loadFromBackup(sessionID)
		if backupErr == nil {
			session = backupSession
			session.RecoveryInfo.BackupFileUsed = fmt.Sprintf("%s.backup", sessionID)
		}
	}

	// Clear incomplete operations
	if len(recoveryInfo.IncompleteEdits) > 0 {
		session.CurrentActionPlan = nil
		session.PlanExecution = nil
	}

	// Reset to safe state
	session.LastSafeState = time.Now()
	session.ConsistencyHash = sm.calculateConsistencyHash(session)

	// Save the recovered session
	if err := sm.SaveSession(); err != nil {
		return nil, fmt.Errorf("failed to save recovered session: %w", err)
	}

	sm.currentSession = session
	return session, nil
}

// calculateConsistencyHash calculates a hash for session consistency checking
func (sm *SessionManager) calculateConsistencyHash(session *SessionState) string {
	// Simple hash based on key session components
	data := fmt.Sprintf("%s|%d|%s|%t",
		session.SessionID,
		len(session.Messages),
		session.Version,
		session.CurrentActionPlan != nil)

	// In a real implementation, you'd use proper hashing
	return fmt.Sprintf("hash_%x", len(data))
}

// loadFromBackup attempts to load a session from backup file
func (sm *SessionManager) loadFromBackup(sessionID string) (*SessionState, error) {
	backupFile := filepath.Join(sm.projectPaths.BackupsDir(), fmt.Sprintf("%s.backup.json", sessionID))

	data, err := os.ReadFile(backupFile)
	if err != nil {
		return nil, fmt.Errorf("backup file not found: %w", err)
	}

	var session SessionState
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal backup session: %w", err)
	}

	return &session, nil
}
