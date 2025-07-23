package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewMemoryStore(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom-memory-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a memory store
	store := NewMemoryStore(tempDir)
	if store == nil {
		t.Fatalf("Expected non-nil memory store")
	}

	// Check that the workspace path is set correctly
	if store.workspacePath != tempDir {
		t.Errorf("Expected workspace path %s, got %s", tempDir, store.workspacePath)
	}
}

func TestMemoryOperations(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom-memory-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a memory store
	store := NewMemoryStore(tempDir)

	// Test creating a memory
	createReq := &MemoryRequest{
		Operation:   MemoryOpCreate,
		ID:          "test-memory",
		Content:     "This is a test memory",
		Tags:        []string{"test", "memory"},
		Description: "Test memory description",
	}

	createResp := store.ProcessRequest(createReq)
	if !createResp.Success {
		t.Errorf("Expected successful memory creation, got error: %s", createResp.Error)
	}

	if createResp.Memory == nil {
		t.Fatalf("Expected memory object in response, got nil")
	}

	if createResp.Memory.ID != "test-memory" {
		t.Errorf("Expected memory ID 'test-memory', got '%s'", createResp.Memory.ID)
	}

	if createResp.Memory.Content != "This is a test memory" {
		t.Errorf("Expected memory content 'This is a test memory', got '%s'", createResp.Memory.Content)
	}

	if len(createResp.Memory.Tags) != 2 || createResp.Memory.Tags[0] != "test" || createResp.Memory.Tags[1] != "memory" {
		t.Errorf("Memory tags don't match expected values: %v", createResp.Memory.Tags)
	}

	if createResp.Memory.Description != "Test memory description" {
		t.Errorf("Expected description 'Test memory description', got '%s'", createResp.Memory.Description)
	}

	if !createResp.Memory.Active {
		t.Errorf("Expected memory to be active by default")
	}

	// Test getting a memory
	getReq := &MemoryRequest{
		Operation: MemoryOpGet,
		ID:        "test-memory",
	}

	getResp := store.ProcessRequest(getReq)
	if !getResp.Success {
		t.Errorf("Expected successful memory retrieval, got error: %s", getResp.Error)
	}

	if getResp.Memory == nil {
		t.Fatalf("Expected memory object in response, got nil")
	}

	if getResp.Memory.ID != "test-memory" {
		t.Errorf("Expected memory ID 'test-memory', got '%s'", getResp.Memory.ID)
	}

	// Test updating a memory
	updateReq := &MemoryRequest{
		Operation:   MemoryOpUpdate,
		ID:          "test-memory",
		Content:     "Updated content",
		Description: "Updated description",
	}

	updateResp := store.ProcessRequest(updateReq)
	if !updateResp.Success {
		t.Errorf("Expected successful memory update, got error: %s", updateResp.Error)
	}

	if updateResp.Memory == nil {
		t.Fatalf("Expected memory object in response, got nil")
	}

	if updateResp.Memory.Content != "Updated content" {
		t.Errorf("Expected updated content 'Updated content', got '%s'", updateResp.Memory.Content)
	}

	if updateResp.Memory.Description != "Updated description" {
		t.Errorf("Expected updated description 'Updated description', got '%s'", updateResp.Memory.Description)
	}

	// Test listing memories
	listReq := &MemoryRequest{
		Operation: MemoryOpList,
	}

	listResp := store.ProcessRequest(listReq)
	if !listResp.Success {
		t.Errorf("Expected successful memory listing, got error: %s", listResp.Error)
	}

	if len(listResp.Memories) != 1 {
		t.Errorf("Expected 1 memory in list, got %d", len(listResp.Memories))
	}

	// Test deleting a memory
	deleteReq := &MemoryRequest{
		Operation: MemoryOpDelete,
		ID:        "test-memory",
	}

	deleteResp := store.ProcessRequest(deleteReq)
	if !deleteResp.Success {
		t.Errorf("Expected successful memory deletion, got error: %s", deleteResp.Error)
	}

	// Verify memory is deleted
	verifyReq := &MemoryRequest{
		Operation: MemoryOpGet,
		ID:        "test-memory",
	}

	verifyResp := store.ProcessRequest(verifyReq)
	if verifyResp.Success {
		t.Errorf("Expected failure when getting deleted memory")
	}
}

func TestMemoryFormatting(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom-memory-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a memory store
	store := NewMemoryStore(tempDir)

	// Add a test memory
	createReq := &MemoryRequest{
		Operation:   MemoryOpCreate,
		ID:          "format-test",
		Content:     "Memory content for formatting test",
		Tags:        []string{"format", "test"},
		Description: "Formatting test",
	}

	createResp := store.ProcessRequest(createReq)
	if !createResp.Success {
		t.Errorf("Failed to create test memory: %s", createResp.Error)
	}

	// Test formatting for prompt
	formattedText := store.FormatMemoriesForPrompt()
	if formattedText == "" {
		t.Errorf("Expected non-empty formatted text")
	}

	// Test memory count
	total, active := store.GetMemoryCount()
	if total != 1 {
		t.Errorf("Expected 1 total memory, got %d", total)
	}

	if active != 1 {
		t.Errorf("Expected 1 active memory, got %d", active)
	}

	// Test getting active memories
	activeMemories := store.GetActiveMemories()
	if len(activeMemories) != 1 {
		t.Errorf("Expected 1 active memory, got %d", len(activeMemories))
	}
}

func TestMemoryPersistence(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom-memory-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create .loom directory
	loomDir := filepath.Join(tempDir, ".loom")
	if err := os.MkdirAll(loomDir, 0755); err != nil {
		t.Fatalf("Failed to create .loom directory: %v", err)
	}

	// Create a memory store
	store1 := NewMemoryStore(tempDir)

	// Add a test memory
	createReq := &MemoryRequest{
		Operation:   MemoryOpCreate,
		ID:          "persistence-test",
		Content:     "Testing persistence",
		Tags:        []string{"persistence"},
		Description: "Persistence test",
	}

	createResp := store1.ProcessRequest(createReq)
	if !createResp.Success {
		t.Errorf("Failed to create test memory: %s", createResp.Error)
	}

	// Save memories
	if err := store1.Save(); err != nil {
		t.Errorf("Failed to save memories: %v", err)
	}

	// Create a new memory store instance to load from disk
	store2 := NewMemoryStore(tempDir)

	// Verify memory was loaded
	getReq := &MemoryRequest{
		Operation: MemoryOpGet,
		ID:        "persistence-test",
	}

	getResp := store2.ProcessRequest(getReq)
	if !getResp.Success {
		t.Errorf("Failed to load persisted memory: %s", getResp.Error)
	}

	if getResp.Memory == nil {
		t.Fatalf("Expected memory object in response after load, got nil")
	}

	if getResp.Memory.ID != "persistence-test" {
		t.Errorf("Expected memory ID 'persistence-test', got '%s'", getResp.Memory.ID)
	}

	if getResp.Memory.Content != "Testing persistence" {
		t.Errorf("Expected memory content 'Testing persistence', got '%s'", getResp.Memory.Content)
	}
}
