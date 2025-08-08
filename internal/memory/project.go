package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// generateProjectID creates a unique identifier for a workspace.
func generateProjectID(path string) string {
	// Create a hash of the workspace path
	hash := sha256.Sum256([]byte(path))
	return hex.EncodeToString(hash[:])[:16] // Use first 16 chars of hash
}
