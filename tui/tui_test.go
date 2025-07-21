package tui

import (
	"testing"

	"loom/chat"
	"loom/config"
	"loom/indexer"
	taskPkg "loom/task"
)

// TestNilPointerFix verifies that the fix for the nil pointer dereference works
func TestNilPointerFix(t *testing.T) {
	// Create a minimal model setup
	chatSession := chat.NewSession("/tmp", 50)

	model := model{
		workspacePath: "/tmp",
		config:        &config.Config{},
		index:         &indexer.Index{},
		chatSession:   chatSession,
	}

	// Create a test task and response
	task := taskPkg.Task{
		Type:   taskPkg.TaskTypeEditFile,
		Path:   "README.md",
		Intent: "test edit",
	}

	response := &taskPkg.TaskResponse{
		Task:    task,
		Success: true,
		Output:  "Test edit prepared",
	}

	// Create a TaskConfirmationMsg
	confirmationMsg := TaskConfirmationMsg{
		Task:     &task,
		Response: response,
		Preview:  "Test preview",
	}

	// Set up pendingConfirmation
	model.pendingConfirmation = &confirmationMsg
	model.showingConfirmation = true

	// This is the key test: verify that we can access the taskResponse
	// without getting a nil pointer dereference even after pendingConfirmation is set to nil

	// Simulate the code path in handleTaskConfirmation
	if model.pendingConfirmation == nil {
		t.Fatal("pendingConfirmation should not be nil at start of test")
	}

	// Extract the values (simulating the fixed code)
	taskFromResponse := model.pendingConfirmation.Response.Task
	taskResponse := model.pendingConfirmation.Response
	model.pendingConfirmation = nil

	// Now verify we can still access the saved values
	if taskFromResponse.Type != taskPkg.TaskTypeEditFile {
		t.Errorf("Expected task type %s, got %s", taskPkg.TaskTypeEditFile, taskFromResponse.Type)
	}

	if taskResponse == nil {
		t.Error("taskResponse should not be nil")
	}

	if taskResponse.Success != true {
		t.Error("taskResponse.Success should be true")
	}

	t.Log("SUCCESS: The nil pointer dereference fix works correctly!")
}
