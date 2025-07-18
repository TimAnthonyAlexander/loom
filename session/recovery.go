package session

import (
	"fmt"
	"loom/llm"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RecoveryManager handles session recovery operations
type RecoveryManager struct {
	workspacePath string
	sessionMgr    *SessionManager
}

// NewRecoveryManager creates a new recovery manager
func NewRecoveryManager(workspacePath string) (*RecoveryManager, error) {
	sessionMgr, err := NewSessionManager(workspacePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create session manager: %w", err)
	}

	return &RecoveryManager{
		workspacePath: workspacePath,
		sessionMgr:    sessionMgr,
	}, nil
}

// CheckForRecovery checks if there are sessions that need recovery
func (rm *RecoveryManager) CheckForRecovery() (*RecoveryOptions, error) {
	recoverable, err := rm.sessionMgr.GetRecoverableSessions()
	if err != nil {
		return nil, fmt.Errorf("failed to get recoverable sessions: %w", err)
	}

	if len(recoverable) == 0 {
		return nil, nil // No recovery needed
	}

	// Check each session for recovery needs
	var incompleteOps []IncompleteOperation
	var corruptedSessions []CorruptedSession
	var lastSession *RecoveryInfo

	for _, sessionInfo := range recoverable {
		// Check if this is the most recent session
		if lastSession == nil || sessionInfo.LastSaved.After(lastSession.LastSaved) {
			lastSession = &sessionInfo
		}

		// Check for incomplete operations
		recovery, err := rm.sessionMgr.DetectIncompleteSession(sessionInfo.SessionID)
		if err != nil {
			continue // Skip sessions we can't analyze
		}

		if len(recovery.IncompleteEdits) > 0 || len(recovery.PendingTasks) > 0 {
			incompleteOps = append(incompleteOps, IncompleteOperation{
				SessionID:    sessionInfo.SessionID,
				LastSaved:    sessionInfo.LastSaved,
				Operations:   append(recovery.IncompleteEdits, recovery.PendingTasks...),
				RecoveryInfo: recovery,
			})
		}

		if recovery.CorruptionDetected {
			corruptedSessions = append(corruptedSessions, CorruptedSession{
				SessionID:    sessionInfo.SessionID,
				LastSaved:    sessionInfo.LastSaved,
				RecoveryInfo: recovery,
			})
		}
	}

	// Create recovery options if needed
	if len(incompleteOps) > 0 || len(corruptedSessions) > 0 {
		return &RecoveryOptions{
			IncompleteOperations: incompleteOps,
			CorruptedSessions:    corruptedSessions,
			LastSession:          lastSession,
			AutoRecoveryAvailable: lastSession != nil,
		}, nil
	}

	return nil, nil // No recovery needed
}

// AutoRecover attempts automatic recovery of the most recent session
func (rm *RecoveryManager) AutoRecover() (*SessionState, error) {
	recoverable, err := rm.sessionMgr.GetRecoverableSessions()
	if err != nil {
		return nil, fmt.Errorf("failed to get recoverable sessions: %w", err)
	}

	if len(recoverable) == 0 {
		return nil, fmt.Errorf("no sessions available for recovery")
	}

	// Try to recover the most recent session
	mostRecent := recoverable[0]

	// Check if it needs recovery
	recovery, err := rm.sessionMgr.DetectIncompleteSession(mostRecent.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to detect recovery needs: %w", err)
	}

	// If no recovery needed, just load the session
	if len(recovery.IncompleteEdits) == 0 && len(recovery.PendingTasks) == 0 && !recovery.CorruptionDetected {
		return rm.sessionMgr.LoadSession(mostRecent.SessionID)
	}

	// Perform recovery
	return rm.sessionMgr.RecoverSession(mostRecent.SessionID)
}

// RecoverSession manually recovers a specific session
func (rm *RecoveryManager) RecoverSession(sessionID string) (*SessionState, error) {
	return rm.sessionMgr.RecoverSession(sessionID)
}

// CreateCleanSession creates a new session after backing up any existing sessions
func (rm *RecoveryManager) CreateCleanSession() (*SessionState, error) {
	// Backup any existing sessions first
	if err := rm.backupExistingSessions(); err != nil {
		// Log warning but don't fail
		fmt.Printf("Warning: failed to backup existing sessions: %v\n", err)
	}

	// Create new session
	return rm.sessionMgr.CreateSession(), nil
}

// backupExistingSessions creates backups of existing sessions
func (rm *RecoveryManager) backupExistingSessions() error {
	recoverable, err := rm.sessionMgr.GetRecoverableSessions()
	if err != nil {
		return err
	}

	backupDir := filepath.Join(rm.workspacePath, ".loom", "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")

	for _, sessionInfo := range recoverable {
		// Only backup recent sessions (last 7 days)
		if time.Since(sessionInfo.LastSaved) > 7*24*time.Hour {
			continue
		}

		backupPath := filepath.Join(backupDir, fmt.Sprintf("%s_%s.backup", sessionInfo.SessionID, timestamp))
		if err := rm.sessionMgr.ExportSession(sessionInfo.SessionID, backupPath); err != nil {
			fmt.Printf("Warning: failed to backup session %s: %v\n", sessionInfo.SessionID, err)
		}
	}

	return nil
}

// GenerateRecoveryReport creates a human-readable recovery report
func (rm *RecoveryManager) GenerateRecoveryReport(options *RecoveryOptions) string {
	if options == nil {
		return "No recovery needed - workspace is in a consistent state."
	}

	var report strings.Builder
	report.WriteString("🔧 Session Recovery Report\n")
	report.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(options.IncompleteOperations) > 0 {
		report.WriteString("⚠️  Incomplete Operations Found:\n")
		for _, op := range options.IncompleteOperations {
			report.WriteString(fmt.Sprintf("  • Session %s (last saved: %s)\n", 
				op.SessionID, op.LastSaved.Format("15:04:05")))
			for _, operation := range op.Operations {
				report.WriteString(fmt.Sprintf("    - %s\n", operation))
			}
		}
		report.WriteString("\n")
	}

	if len(options.CorruptedSessions) > 0 {
		report.WriteString("❌ Corrupted Sessions Found:\n")
		for _, corrupted := range options.CorruptedSessions {
			report.WriteString(fmt.Sprintf("  • Session %s (last saved: %s)\n", 
				corrupted.SessionID, corrupted.LastSaved.Format("15:04:05")))
			if corrupted.RecoveryInfo.BackupFileUsed != "" {
				report.WriteString(fmt.Sprintf("    Backup available: %s\n", corrupted.RecoveryInfo.BackupFileUsed))
			}
		}
		report.WriteString("\n")
	}

	if options.AutoRecoveryAvailable && options.LastSession != nil {
		report.WriteString("✅ Auto-recovery Available:\n")
		report.WriteString(fmt.Sprintf("  • Most recent session: %s\n", options.LastSession.SessionID))
		report.WriteString(fmt.Sprintf("  • Last saved: %s\n", options.LastSession.LastSaved.Format("2006-01-02 15:04:05")))
		report.WriteString(fmt.Sprintf("  • Messages: %d\n", options.LastSession.MessageCount))
		if options.LastSession.HasActionPlan {
			report.WriteString("  • Has active action plan\n")
		}
		report.WriteString("\n")
	}

	report.WriteString("Recovery Options:\n")
	report.WriteString("1. Auto-recover most recent session\n")
	report.WriteString("2. Manual session selection\n")
	report.WriteString("3. Start fresh (with backup)\n")

	return report.String()
}

// RecoveryOptions represents available recovery options
type RecoveryOptions struct {
	IncompleteOperations  []IncompleteOperation `json:"incomplete_operations"`
	CorruptedSessions     []CorruptedSession    `json:"corrupted_sessions"`
	LastSession           *RecoveryInfo         `json:"last_session"`
	AutoRecoveryAvailable bool                  `json:"auto_recovery_available"`
}

// IncompleteOperation represents an incomplete operation that needs recovery
type IncompleteOperation struct {
	SessionID    string         `json:"session_id"`
	LastSaved    time.Time      `json:"last_saved"`
	Operations   []string       `json:"operations"`
	RecoveryInfo *RecoveryState `json:"recovery_info"`
}

// CorruptedSession represents a corrupted session that needs recovery
type CorruptedSession struct {
	SessionID    string         `json:"session_id"`
	LastSaved    time.Time      `json:"last_saved"`
	RecoveryInfo *RecoveryState `json:"recovery_info"`
}

// RecoveryChoice represents a user's choice for recovery
type RecoveryChoice int

const (
	RecoveryChoiceAuto RecoveryChoice = iota
	RecoveryChoiceManual
	RecoveryChoiceFresh
	RecoveryChoiceSkip
)

// AddRecoveryMessage adds a recovery message to a session
func (rm *RecoveryManager) AddRecoveryMessage(session *SessionState, recoveryType string, details string) error {
	recoveryMsg := llm.Message{
		Role:      "system",
		Content:   fmt.Sprintf("SESSION_RECOVERY: %s - %s", recoveryType, details),
		Timestamp: time.Now(),
	}

	session.Messages = append(session.Messages, recoveryMsg)
	return nil
}

// CleanupOldBackups removes old backup files
func (rm *RecoveryManager) CleanupOldBackups(olderThan time.Duration) error {
	backupDir := filepath.Join(rm.workspacePath, ".loom", "backups")
	
	files, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No backup directory exists
		}
		return fmt.Errorf("failed to read backup directory: %w", err)
	}

	cutoff := time.Now().Add(-olderThan)

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".backup") {
			info, err := file.Info()
			if err != nil {
				continue
			}

			if info.ModTime().Before(cutoff) {
				backupPath := filepath.Join(backupDir, file.Name())
				if err := os.Remove(backupPath); err != nil {
					fmt.Printf("Warning: failed to remove old backup %s: %v\n", file.Name(), err)
				}
			}
		}
	}

	return nil
}

// GetSessionManager returns the underlying session manager
func (rm *RecoveryManager) GetSessionManager() *SessionManager {
	return rm.sessionMgr
} 