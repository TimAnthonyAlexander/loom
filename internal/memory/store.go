package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Store manages persistent storage for Loom.
type Store struct {
	rootDir string
	mu      sync.RWMutex
	cache   map[string]interface{}
}

// NewStore creates a new Store.
func NewStore(rootDir string) (*Store, error) {
	// If no root dir specified, use user config directory
	if rootDir == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return nil, fmt.Errorf("failed to determine user config directory: %w", err)
		}
		rootDir = filepath.Join(configDir, "loom")
	}

	// Ensure directory exists
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create store directory: %w", err)
	}

	return &Store{
		rootDir: rootDir,
		cache:   make(map[string]interface{}),
	}, nil
}

// Get retrieves a value from storage.
func (s *Store) Get(key string, valuePtr interface{}) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Try to get from cache first
	if cachedVal, ok := s.cache[key]; ok {
		// Convert cached value to JSON and then unmarshal
		bytes, err := json.Marshal(cachedVal)
		if err != nil {
			return fmt.Errorf("failed to marshal cached value: %w", err)
		}

		return json.Unmarshal(bytes, valuePtr)
	}

	// Read from file
	filePath := filepath.Join(s.rootDir, key+".json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("key not found: %s", key)
		}
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Parse the JSON data
	if err := json.Unmarshal(data, valuePtr); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Update cache
	var cachedVal interface{}
	if err := json.Unmarshal(data, &cachedVal); err == nil {
		s.mu.RUnlock()
		s.mu.Lock()
		s.cache[key] = cachedVal
		s.mu.Unlock()
		s.mu.RLock()
	}

	return nil
}

// Set stores a value.
func (s *Store) Set(key string, value interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Update cache
	s.cache[key] = value

	// Serialize to JSON
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize value: %w", err)
	}

	// Write to file
	filePath := filepath.Join(s.rootDir, key+".json")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Delete removes a value from storage.
func (s *Store) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove from cache
	delete(s.cache, key)

	// Remove from disk
	filePath := filepath.Join(s.rootDir, key+".json")
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			// Already deleted, not an error
			return nil
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// Has checks if a key exists.
func (s *Store) Has(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check cache first
	if _, ok := s.cache[key]; ok {
		return true
	}

	// Check on disk
	filePath := filepath.Join(s.rootDir, key+".json")
	_, err := os.Stat(filePath)
	return err == nil
}

// Keys returns all keys in the store.
func (s *Store) Keys() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Read directory
	entries, err := os.ReadDir(s.rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory: %w", err)
	}

	// Filter JSON files and extract keys
	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if filepath.Ext(name) == ".json" {
			// Remove .json extension to get the key
			key := name[:len(name)-5] // len(".json") == 5
			keys = append(keys, key)
		}
	}

	return keys, nil
}
