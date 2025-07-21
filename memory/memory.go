package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Memory represents a single memory entry
type Memory struct {
	ID          string    `json:"id"`
	Content     string    `json:"content"`
	Tags        []string  `json:"tags,omitempty"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Description string    `json:"description,omitempty"`
}

// MemoryStore manages persistent memories
type MemoryStore struct {
	workspacePath string
	memories      map[string]*Memory
}

// MemoryOperation represents the type of memory operation
type MemoryOperation string

const (
	MemoryOperationCreate MemoryOperation = "create"
	MemoryOperationUpdate MemoryOperation = "update"
	MemoryOperationDelete MemoryOperation = "delete"
	MemoryOperationGet    MemoryOperation = "get"
	MemoryOperationList   MemoryOperation = "list"
)

// MemoryRequest represents a memory operation request
type MemoryRequest struct {
	Operation   MemoryOperation `json:"operation"`
	ID          string          `json:"id"`
	Content     string          `json:"content,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
	Active      *bool           `json:"active,omitempty"`
	Description string          `json:"description,omitempty"`
}

// MemoryResponse represents the result of a memory operation
type MemoryResponse struct {
	Success  bool      `json:"success"`
	Error    string    `json:"error,omitempty"`
	Memory   *Memory   `json:"memory,omitempty"`
	Memories []*Memory `json:"memories,omitempty"`
	Message  string    `json:"message,omitempty"`
}

// NewMemoryStore creates a new memory store
func NewMemoryStore(workspacePath string) *MemoryStore {
	ms := &MemoryStore{
		workspacePath: workspacePath,
		memories:      make(map[string]*Memory),
	}

	// Load existing memories
	if err := ms.Load(); err != nil {
		// If loading fails, start with empty memory store
		fmt.Printf("Warning: failed to load memories: %v\n", err)
	}

	return ms
}

// memoriesPath returns the path to the memories.json file
func (ms *MemoryStore) memoriesPath() string {
	return filepath.Join(ms.workspacePath, ".loom", "memories.json")
}

// Load loads memories from disk
func (ms *MemoryStore) Load() error {
	memoriesPath := ms.memoriesPath()

	// Check if memories file exists
	if _, err := os.Stat(memoriesPath); os.IsNotExist(err) {
		// No memories file exists yet, start with empty store
		return nil
	}

	data, err := os.ReadFile(memoriesPath)
	if err != nil {
		return fmt.Errorf("failed to read memories file: %w", err)
	}

	var memoriesSlice []*Memory
	if err := json.Unmarshal(data, &memoriesSlice); err != nil {
		return fmt.Errorf("failed to unmarshal memories: %w", err)
	}

	// Convert slice to map
	ms.memories = make(map[string]*Memory)
	for _, memory := range memoriesSlice {
		ms.memories[memory.ID] = memory
	}

	return nil
}

// Save saves memories to disk
func (ms *MemoryStore) Save() error {
	// Ensure .loom directory exists
	loomDir := filepath.Join(ms.workspacePath, ".loom")
	if err := os.MkdirAll(loomDir, 0755); err != nil {
		return fmt.Errorf("failed to create .loom directory: %w", err)
	}

	// Convert map to slice for consistent ordering
	var memoriesSlice []*Memory
	for _, memory := range ms.memories {
		memoriesSlice = append(memoriesSlice, memory)
	}

	// Sort by ID for consistent output
	sort.Slice(memoriesSlice, func(i, j int) bool {
		return memoriesSlice[i].ID < memoriesSlice[j].ID
	})

	data, err := json.MarshalIndent(memoriesSlice, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal memories: %w", err)
	}

	memoriesPath := ms.memoriesPath()
	if err := os.WriteFile(memoriesPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write memories file: %w", err)
	}

	return nil
}

// ProcessRequest processes a memory operation request
func (ms *MemoryStore) ProcessRequest(req *MemoryRequest) *MemoryResponse {
	switch req.Operation {
	case MemoryOperationCreate:
		return ms.createMemory(req)
	case MemoryOperationUpdate:
		return ms.updateMemory(req)
	case MemoryOperationDelete:
		return ms.deleteMemory(req)
	case MemoryOperationGet:
		return ms.getMemory(req)
	case MemoryOperationList:
		return ms.listMemories(req)
	default:
		return &MemoryResponse{
			Success: false,
			Error:   fmt.Sprintf("unknown memory operation: %s", req.Operation),
		}
	}
}

// createMemory creates a new memory
func (ms *MemoryStore) createMemory(req *MemoryRequest) *MemoryResponse {
	if req.ID == "" {
		return &MemoryResponse{
			Success: false,
			Error:   "memory ID is required for create operation",
		}
	}

	if req.Content == "" {
		return &MemoryResponse{
			Success: false,
			Error:   "memory content is required for create operation",
		}
	}

	// Check if memory with this ID already exists
	if _, exists := ms.memories[req.ID]; exists {
		return &MemoryResponse{
			Success: false,
			Error:   fmt.Sprintf("memory with ID '%s' already exists", req.ID),
		}
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	memory := &Memory{
		ID:          req.ID,
		Content:     req.Content,
		Tags:        req.Tags,
		Active:      active,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Description: req.Description,
	}

	ms.memories[req.ID] = memory

	if err := ms.Save(); err != nil {
		return &MemoryResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to save memory: %v", err),
		}
	}

	return &MemoryResponse{
		Success: true,
		Memory:  memory,
		Message: fmt.Sprintf("Created memory '%s'", req.ID),
	}
}

// updateMemory updates an existing memory
func (ms *MemoryStore) updateMemory(req *MemoryRequest) *MemoryResponse {
	if req.ID == "" {
		return &MemoryResponse{
			Success: false,
			Error:   "memory ID is required for update operation",
		}
	}

	memory, exists := ms.memories[req.ID]
	if !exists {
		return &MemoryResponse{
			Success: false,
			Error:   fmt.Sprintf("memory with ID '%s' not found", req.ID),
		}
	}

	// Update fields that are provided
	if req.Content != "" {
		memory.Content = req.Content
	}

	if req.Tags != nil {
		memory.Tags = req.Tags
	}

	if req.Active != nil {
		memory.Active = *req.Active
	}

	if req.Description != "" {
		memory.Description = req.Description
	}

	memory.UpdatedAt = time.Now()

	if err := ms.Save(); err != nil {
		return &MemoryResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to save memory: %v", err),
		}
	}

	return &MemoryResponse{
		Success: true,
		Memory:  memory,
		Message: fmt.Sprintf("Updated memory '%s'", req.ID),
	}
}

// deleteMemory deletes a memory
func (ms *MemoryStore) deleteMemory(req *MemoryRequest) *MemoryResponse {
	if req.ID == "" {
		return &MemoryResponse{
			Success: false,
			Error:   "memory ID is required for delete operation",
		}
	}

	memory, exists := ms.memories[req.ID]
	if !exists {
		return &MemoryResponse{
			Success: false,
			Error:   fmt.Sprintf("memory with ID '%s' not found", req.ID),
		}
	}

	delete(ms.memories, req.ID)

	if err := ms.Save(); err != nil {
		return &MemoryResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to save memory: %v", err),
		}
	}

	return &MemoryResponse{
		Success: true,
		Memory:  memory,
		Message: fmt.Sprintf("Deleted memory '%s'", req.ID),
	}
}

// getMemory retrieves a specific memory
func (ms *MemoryStore) getMemory(req *MemoryRequest) *MemoryResponse {
	if req.ID == "" {
		return &MemoryResponse{
			Success: false,
			Error:   "memory ID is required for get operation",
		}
	}

	memory, exists := ms.memories[req.ID]
	if !exists {
		return &MemoryResponse{
			Success: false,
			Error:   fmt.Sprintf("memory with ID '%s' not found", req.ID),
		}
	}

	return &MemoryResponse{
		Success: true,
		Memory:  memory,
		Message: fmt.Sprintf("Retrieved memory '%s'", req.ID),
	}
}

// listMemories lists all memories or active memories
func (ms *MemoryStore) listMemories(req *MemoryRequest) *MemoryResponse {
	var memories []*Memory

	for _, memory := range ms.memories {
		// If active filter is specified, only include memories matching that filter
		if req.Active != nil && memory.Active != *req.Active {
			continue
		}
		memories = append(memories, memory)
	}

	// Sort by creation time (newest first)
	sort.Slice(memories, func(i, j int) bool {
		return memories[i].CreatedAt.After(memories[j].CreatedAt)
	})

	activeFilter := ""
	if req.Active != nil {
		if *req.Active {
			activeFilter = " (active only)"
		} else {
			activeFilter = " (inactive only)"
		}
	}

	return &MemoryResponse{
		Success:  true,
		Memories: memories,
		Message:  fmt.Sprintf("Listed %d memories%s", len(memories), activeFilter),
	}
}

// GetActiveMemories returns all active memories for injection into system prompt
func (ms *MemoryStore) GetActiveMemories() []*Memory {
	var activeMemories []*Memory

	for _, memory := range ms.memories {
		if memory.Active {
			activeMemories = append(activeMemories, memory)
		}
	}

	// Sort by creation time (oldest first for system prompt context)
	sort.Slice(activeMemories, func(i, j int) bool {
		return activeMemories[i].CreatedAt.Before(activeMemories[j].CreatedAt)
	})

	return activeMemories
}

// FormatMemoriesForPrompt formats active memories for inclusion in system prompt
func (ms *MemoryStore) FormatMemoriesForPrompt() string {
	activeMemories := ms.GetActiveMemories()

	if len(activeMemories) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("## Active Memories\n\n")
	builder.WriteString("The following memories contain important context for this session:\n\n")

	for _, memory := range activeMemories {
		builder.WriteString(fmt.Sprintf("### Memory: %s\n", memory.ID))
		if memory.Description != "" {
			builder.WriteString(fmt.Sprintf("*%s*\n\n", memory.Description))
		}
		builder.WriteString(fmt.Sprintf("%s\n\n", memory.Content))

		if len(memory.Tags) > 0 {
			builder.WriteString(fmt.Sprintf("*Tags: %s*\n\n", strings.Join(memory.Tags, ", ")))
		}

		builder.WriteString("---\n\n")
	}

	return builder.String()
}

// GetMemoryCount returns the total number of memories and active memories
func (ms *MemoryStore) GetMemoryCount() (total int, active int) {
	total = len(ms.memories)

	for _, memory := range ms.memories {
		if memory.Active {
			active++
		}
	}

	return total, active
}
