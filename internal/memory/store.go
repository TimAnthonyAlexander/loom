package memory

import (
	"encoding/json"
	"fmt"
	"io/fs"
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
	// If no root dir specified, use ~/.loom by default
	if rootDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to determine user home directory: %w", err)
		}
		rootDir = filepath.Join(home, ".loom")
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
	// Ensure parent directory exists for nested keys
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}
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

	keys := make([]string, 0)
	// Walk recursively to include nested keys
	err := filepath.WalkDir(s.rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".json" {
			return nil
		}
		// Derive key relative to rootDir without .json
		rel, err := filepath.Rel(s.rootDir, path)
		if err != nil {
			return err
		}
		key := rel[:len(rel)-5]
		// Normalize to use '/' as separator in keys
		key = filepath.ToSlash(key)
		keys = append(keys, key)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}
	return keys, nil
}
