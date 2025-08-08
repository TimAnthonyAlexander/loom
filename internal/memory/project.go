package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Project manages workspace-specific persistent storage.
type Project struct {
	store         *Store
	workspacePath string
	projectID     string
	mu            sync.RWMutex
}

// NewProject creates a new project storage for the given workspace.
func NewProject(store *Store, workspacePath string) (*Project, error) {
	// Normalize the workspace path
	absPath, err := filepath.Abs(workspacePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Generate a consistent project ID from the workspace path
	projectID := generateProjectID(absPath)

	// Create projects directory if needed
	projectsDir := filepath.Join(store.rootDir, "projects")
	if err := os.MkdirAll(projectsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create projects directory: %w", err)
	}

	return &Project{
		store:         store,
		workspacePath: absPath,
		projectID:     projectID,
	}, nil
}

// Get retrieves a value from project storage.
func (p *Project) Get(key string, valuePtr interface{}) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	projectKey := fmt.Sprintf("projects/%s/%s", p.projectID, key)
	return p.store.Get(projectKey, valuePtr)
}

// Set stores a value in project storage.
func (p *Project) Set(key string, value interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	projectKey := fmt.Sprintf("projects/%s/%s", p.projectID, key)
	return p.store.Set(projectKey, value)
}

// Delete removes a value from project storage.
func (p *Project) Delete(key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	projectKey := fmt.Sprintf("projects/%s/%s", p.projectID, key)
	return p.store.Delete(projectKey)
}

// Has checks if a key exists in project storage.
func (p *Project) Has(key string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	projectKey := fmt.Sprintf("projects/%s/%s", p.projectID, key)
	return p.store.Has(projectKey)
}

// Keys returns all keys in the project storage.
func (p *Project) Keys() ([]string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	allKeys, err := p.store.Keys()
	if err != nil {
		return nil, err
	}

	// Filter keys that belong to this project
	prefix := fmt.Sprintf("projects/%s/", p.projectID)
	projectKeys := make([]string, 0)

	for _, key := range allKeys {
		if strings.HasPrefix(key, prefix) {
			// Extract just the project-specific part of the key
			shortKey := strings.TrimPrefix(key, prefix)
			projectKeys = append(projectKeys, shortKey)
		}
	}

	return projectKeys, nil
}

// SaveConversation stores a conversation in the project memory.
func (p *Project) SaveConversation(conversationID string, messages []interface{}) error {
	return p.Set("conversations/"+conversationID, messages)
}

// GetConversations retrieves all stored conversations.
func (p *Project) GetConversations() (map[string][]interface{}, error) {
	keys, err := p.Keys()
	if err != nil {
		return nil, err
	}

	conversations := make(map[string][]interface{})

	for _, key := range keys {
		if strings.HasPrefix(key, "conversations/") {
			convID := strings.TrimPrefix(key, "conversations/")
			var messages []interface{}
			if err := p.Get(key, &messages); err != nil {
				return nil, err
			}
			conversations[convID] = messages
		}
	}

	return conversations, nil
}

// ConversationSummary is a lightweight summary for listing conversations.
type ConversationSummary struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ConversationMeta stores additional metadata for a conversation
type ConversationMeta struct {
	Title     string    `json:"title,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// CurrentConversationID returns the currently active conversation id.
func (p *Project) CurrentConversationID() string {
	var id string
	if p.Has("conversations/current_id") {
		if err := p.Get("conversations/current_id", &id); err == nil {
			return id
		}
	}
	return ""
}

// SetCurrentConversationID sets the current conversation id.
func (p *Project) SetCurrentConversationID(id string) error {
	return p.Set("conversations/current_id", id)
}

// CreateNewConversation creates a new empty conversation, sets it current, and returns its id.
func (p *Project) CreateNewConversation() string {
	id := generateConversationID()
	// Initialize empty message list
	_ = p.Set("conversations/"+id, []Message{})
	_ = p.SetCurrentConversationID(id)
	return id
}

// ListConversationSummaries returns summaries for all conversations sorted by UpdatedAt desc.
func (p *Project) ListConversationSummaries() ([]ConversationSummary, error) {
	keys, err := p.Keys()
	if err != nil {
		return nil, err
	}
	var summaries []ConversationSummary
	for _, key := range keys {
		if !strings.HasPrefix(key, "conversations/") || key == "conversations/current_id" {
			continue
		}
		convID := strings.TrimPrefix(key, "conversations/")
		var messages []Message
		if err := p.Get(key, &messages); err != nil {
			// Skip malformed
			continue
		}
		// Title from meta if available; otherwise derive from first non-system message
		title := convID
		var meta ConversationMeta
		if p.Has("conversations_meta/" + convID) {
			if err := p.Get("conversations_meta/"+convID, &meta); err == nil && strings.TrimSpace(meta.Title) != "" {
				title = meta.Title
			}
		}
		var updated time.Time
		if len(messages) > 0 {
			for _, m := range messages {
				if m.Timestamp.After(updated) {
					updated = m.Timestamp
				}
			}
		}
		if !meta.UpdatedAt.IsZero() && meta.UpdatedAt.After(updated) {
			updated = meta.UpdatedAt
		}
		summaries = append(summaries, ConversationSummary{ID: convID, Title: title, UpdatedAt: updated})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].UpdatedAt.After(summaries[j].UpdatedAt)
	})
	return summaries, nil
}

func trimTitle(s string) string {
	s = strings.TrimSpace(s)
	// first line only
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		s = s[:idx]
	}
	// limit length
	const max = 80
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

func generateConversationID() string {
	return time.Now().Format("20060102-150405")
}

// SetConversationTitle stores a title for the conversation in meta.
func (p *Project) SetConversationTitle(id string, title string) error {
	meta := ConversationMeta{Title: trimTitle(title), UpdatedAt: time.Now()}
	return p.Set("conversations_meta/"+id, meta)
}

// GetConversationTitle retrieves a stored title, if any.
func (p *Project) GetConversationTitle(id string) string {
	var meta ConversationMeta
	if err := p.Get("conversations_meta/"+id, &meta); err == nil {
		return meta.Title
	}
	return ""
}

// DeleteConversation removes a conversation and its metadata.
func (p *Project) DeleteConversation(id string) error {
	_ = p.Delete("conversations/" + id)
	_ = p.Delete("conversations_meta/" + id)
	return nil
}

// CleanupEmptyConversations deletes all non-current conversations that have no user messages.
func (p *Project) CleanupEmptyConversations(currentID string) {
	keys, err := p.Keys()
	if err != nil {
		return
	}
	for _, key := range keys {
		if !strings.HasPrefix(key, "conversations/") || key == "conversations/current_id" {
			continue
		}
		convID := strings.TrimPrefix(key, "conversations/")
		if convID == currentID {
			continue
		}
		var messages []Message
		if err := p.Get(key, &messages); err != nil {
			continue
		}
		hasUser := false
		for _, m := range messages {
			if m.Role == "user" {
				hasUser = true
				break
			}
		}
		if !hasUser {
			_ = p.DeleteConversation(convID)
		}
	}
}

// generateProjectID creates a unique identifier for a workspace.
func generateProjectID(path string) string {
	// Create a hash of the workspace path
	hash := sha256.Sum256([]byte(path))
	return hex.EncodeToString(hash[:])[:16] // Use first 16 chars of hash
}
