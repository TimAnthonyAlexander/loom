package memory

import (
	"encoding/json"
	"fmt"
	"loom/paths"
	"os"
	"path/filepath"
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

// MemoryStore manages project-specific memories
type MemoryStore struct {
	workspacePath string
	projectPaths  *paths.ProjectPaths
	memories      map[string]*Memory
}

// MemoryOperation represents the type of memory operation
type MemoryOperation string

const (
	MemoryOpCreate MemoryOperation = "create"
	MemoryOpUpdate MemoryOperation = "update"
	MemoryOpDelete MemoryOperation = "delete"
	MemoryOpGet    MemoryOperation = "get"
	MemoryOpList   MemoryOperation = "list"
)

// MemoryRequest represents a request to modify memories
type MemoryRequest struct {
	Operation   MemoryOperation `json:"operation"`
	ID          string          `json:"id"`
	Content     string          `json:"content,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
	Active      *bool           `json:"active,omitempty"`
	Description string          `json:"description,omitempty"`
}

// MemoryResponse represents the response from a memory operation
type MemoryResponse struct {
	Success  bool      `json:"success"`
	Error    string    `json:"error,omitempty"`
	Memory   *Memory   `json:"memory,omitempty"`
	Memories []*Memory `json:"memories,omitempty"`
	Message  string    `json:"message,omitempty"`
}

// NewMemoryStore creates a new memory store for the workspace
func NewMemoryStore(workspacePath string) *MemoryStore {
	// Get project paths
	projectPaths, err := paths.NewProjectPaths(workspacePath)
	if err != nil {
		fmt.Printf("Warning: failed to create project paths for memory store: %v\n", err)
		// Fallback to legacy behavior
		return &MemoryStore{
			workspacePath: workspacePath,
			projectPaths:  nil,
			memories:      make(map[string]*Memory),
		}
	}

	// Ensure project directories exist
	if err := projectPaths.EnsureProjectDir(); err != nil {
		fmt.Printf("Warning: failed to create project directories: %v\n", err)
	}

	ms := &MemoryStore{
		workspacePath: workspacePath,
		projectPaths:  projectPaths,
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
	if ms.projectPaths != nil {
		return ms.projectPaths.MemoriesPath()
	}
	// Fallback to legacy path
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
	memoriesPath := ms.memoriesPath()

	// Ensure parent directory exists
	if ms.projectPaths != nil {
		// Already ensured by EnsureProjectDir in NewMemoryStore
	} else {
		// Legacy fallback - ensure .loom directory exists
		loomDir := filepath.Dir(memoriesPath)
		if err := os.MkdirAll(loomDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Convert map to slice for JSON serialization
	var memoriesSlice []*Memory
	for _, memory := range ms.memories {
		memoriesSlice = append(memoriesSlice, memory)
	}

	data, err := json.MarshalIndent(memoriesSlice, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal memories: %w", err)
	}

	if err := os.WriteFile(memoriesPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write memories file: %w", err)
	}

	return nil
}

// ProcessRequest handles memory operations
func (ms *MemoryStore) ProcessRequest(req *MemoryRequest) *MemoryResponse {
	switch req.Operation {
	case MemoryOpCreate:
		return ms.createMemory(req)
	case MemoryOpUpdate:
		return ms.updateMemory(req)
	case MemoryOpDelete:
		return ms.deleteMemory(req)
	case MemoryOpGet:
		return ms.getMemory(req)
	case MemoryOpList:
		return ms.listMemories(req)
	default:
		return &MemoryResponse{
			Success: false,
			Error:   fmt.Sprintf("unknown operation: %s", req.Operation),
		}
	}
}

// createMemory creates a new memory
func (ms *MemoryStore) createMemory(req *MemoryRequest) *MemoryResponse {
	if req.ID == "" {
		return &MemoryResponse{
			Success: false,
			Error:   "memory ID is required",
		}
	}

	if req.Content == "" {
		return &MemoryResponse{
			Success: false,
			Error:   "memory content is required",
		}
	}

	// Check if memory already exists
	if _, exists := ms.memories[req.ID]; exists {
		return &MemoryResponse{
			Success: false,
			Error:   fmt.Sprintf("memory with ID '%s' already exists", req.ID),
		}
	}

	// Set default active state
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

	// Save to disk
	if err := ms.Save(); err != nil {
		return &MemoryResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to save memory: %v", err),
		}
	}

	return &MemoryResponse{
		Success: true,
		Memory:  memory,
		Message: fmt.Sprintf("Memory '%s' created successfully", req.ID),
	}
}

// updateMemory updates an existing memory
func (ms *MemoryStore) updateMemory(req *MemoryRequest) *MemoryResponse {
	if req.ID == "" {
		return &MemoryResponse{
			Success: false,
			Error:   "memory ID is required",
		}
	}

	memory, exists := ms.memories[req.ID]
	if !exists {
		return &MemoryResponse{
			Success: false,
			Error:   fmt.Sprintf("memory with ID '%s' not found", req.ID),
		}
	}

	// Update fields if provided
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

	// Save to disk
	if err := ms.Save(); err != nil {
		return &MemoryResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to save memory: %v", err),
		}
	}

	return &MemoryResponse{
		Success: true,
		Memory:  memory,
		Message: fmt.Sprintf("Memory '%s' updated successfully", req.ID),
	}
}

// deleteMemory deletes an existing memory
func (ms *MemoryStore) deleteMemory(req *MemoryRequest) *MemoryResponse {
	if req.ID == "" {
		return &MemoryResponse{
			Success: false,
			Error:   "memory ID is required",
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

	// Save to disk
	if err := ms.Save(); err != nil {
		return &MemoryResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to save after deletion: %v", err),
		}
	}

	return &MemoryResponse{
		Success: true,
		Memory:  memory,
		Message: fmt.Sprintf("Memory '%s' deleted successfully", req.ID),
	}
}

// getMemory retrieves a specific memory
func (ms *MemoryStore) getMemory(req *MemoryRequest) *MemoryResponse {
	if req.ID == "" {
		return &MemoryResponse{
			Success: false,
			Error:   "memory ID is required",
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
	}
}

// listMemories lists all memories
func (ms *MemoryStore) listMemories(req *MemoryRequest) *MemoryResponse {
	var memories []*Memory
	for _, memory := range ms.memories {
		memories = append(memories, memory)
	}

	return &MemoryResponse{
		Success:  true,
		Memories: memories,
		Message:  fmt.Sprintf("Found %d memories", len(memories)),
	}
}

// GetActiveMemories returns all active memories
func (ms *MemoryStore) GetActiveMemories() []*Memory {
	var active []*Memory
	for _, memory := range ms.memories {
		if memory.Active {
			active = append(active, memory)
		}
	}
	return active
}

// FormatMemoriesForPrompt formats active memories for inclusion in system prompts
func (ms *MemoryStore) FormatMemoriesForPrompt() string {
	activeMemories := ms.GetActiveMemories()
	if len(activeMemories) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("## Project Memories\n\n")
	builder.WriteString("The following memories contain important context about this project:\n\n")

	for _, memory := range activeMemories {
		builder.WriteString(fmt.Sprintf("### %s\n", memory.ID))
		if memory.Description != "" {
			builder.WriteString(fmt.Sprintf("*%s*\n\n", memory.Description))
		}
		builder.WriteString(fmt.Sprintf("%s\n\n", memory.Content))
		if len(memory.Tags) > 0 {
			builder.WriteString(fmt.Sprintf("Tags: %s\n\n", strings.Join(memory.Tags, ", ")))
		}
	}

	return builder.String()
}

// GetMemoryCount returns total and active memory counts
func (ms *MemoryStore) GetMemoryCount() (total int, active int) {
	total = len(ms.memories)
	for _, memory := range ms.memories {
		if memory.Active {
			active++
		}
	}
	return total, active
}
