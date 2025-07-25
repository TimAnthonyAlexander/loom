package task

import (
	"fmt"
	"io/ioutil"
	"loom/context"
	"loom/indexer"
	"loom/llm"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestFilenameSearchContextPreservation reproduces the bug where filename search results
// are not properly preserved during context optimization
func TestFilenameSearchContextPreservation(t *testing.T) {
	// Create a temp dir for testing
	tempDir := t.TempDir()

	// Create a sample.json file in the temp dir
	sampleFile := filepath.Join(tempDir, "sample.json")
	err := ioutil.WriteFile(sampleFile, []byte(`{"name": "test"}`), 0644)
	if err != nil {
		t.Fatalf("Failed to create sample file: %v", err)
	}

	// Create an index
	index := indexer.NewIndex(tempDir, 1024*1024)
	err = index.BuildIndex()
	if err != nil {
		t.Fatalf("Failed to build index: %v", err)
	}

	// Create a context manager for optimization with a small token limit to force optimization
	contextManager := context.NewContextManager(index, 2000) // Small token limit

	// Create executor with the temp dir as workspace
	executor := NewExecutor(tempDir, false, 1024*1024)

	// Create a mock chat session
	mockChat := &ContextTestMockChat{}

	// Create sequential manager with context manager
	manager := NewSequentialTaskManager(executor, nil, mockChat)
	manager.SetContextManager(index, 2000)

	// Create and execute a filename search task
	task := &Task{
		Type:        TaskTypeSearch,
		Path:        ".",
		Query:       "sample.json",
		SearchNames: true,
	}

	// Execute the search task
	taskResponse := executor.Execute(task)

	// Format the task result
	taskResultMsg := manager.formatTaskResultForExploration(task, taskResponse)

	// Verify the task result message has the proper role (system, not assistant)
	if taskResultMsg.Role != "system" {
		t.Errorf("Expected task result message role to be 'system', got '%s'", taskResultMsg.Role)
	}

	// Create a realistic set of messages that simulates a conversation
	// First the system message (always kept in optimization)
	systemMsg := llm.Message{
		Role:      "system",
		Content:   "You are an AI coding assistant",
		Timestamp: time.Now(),
	}

	// Create a large number of simulated conversation messages
	largeMessages := []llm.Message{systemMsg}

	// Add initial user message
	userMsg := llm.Message{
		Role:      "user",
		Content:   "I want to analyze this codebase",
		Timestamp: time.Now(),
	}
	largeMessages = append(largeMessages, userMsg)

	// Add many messages to simulate a longer conversation
	for i := 0; i < 20; i++ {
		userFillerMsg := llm.Message{
			Role:      "user",
			Content:   strings.Repeat(fmt.Sprintf("This is user message %d. ", i), 5),
			Timestamp: time.Now(),
		}

		assistantFillerMsg := llm.Message{
			Role:      "assistant",
			Content:   strings.Repeat(fmt.Sprintf("This is assistant response %d. ", i), 10),
			Timestamp: time.Now(),
		}

		largeMessages = append(largeMessages, userFillerMsg, assistantFillerMsg)
	}

	// Add our most recent user message (search request)
	searchRequestMsg := llm.Message{
		Role:      "user",
		Content:   "Can you search for sample.json file?",
		Timestamp: time.Now(),
	}
	largeMessages = append(largeMessages, searchRequestMsg)

	// Add assistant acknowledgment
	searchAckMsg := llm.Message{
		Role:      "assistant",
		Content:   "I'll search for the sample.json file",
		Timestamp: time.Now(),
	}
	largeMessages = append(largeMessages, searchAckMsg)

	// Add our filename search result message - this should be preserved
	largeMessages = append(largeMessages, taskResultMsg)

	// Verify the task result contains FOUND FILES MATCHING NAME or NO FILES FOUND MATCHING NAME
	if !strings.Contains(taskResultMsg.Content, "FOUND FILES MATCHING NAME") &&
		!strings.Contains(taskResultMsg.Content, "NO FILES FOUND MATCHING NAME") {
		t.Errorf("Expected task result to contain filename match marker, but it didn't: %s",
			truncateForTest(taskResultMsg.Content, 200))
	}

	// CRITICAL STEP: Apply context optimization, which is where the bug happens
	optimizedMessages, err := contextManager.OptimizeMessages(largeMessages)
	if err != nil {
		t.Fatalf("Failed to optimize messages: %v", err)
	}

	// Now check if the filename search results were preserved in the optimized context
	foundInOptimized := false
	for _, msg := range optimizedMessages {
		if strings.Contains(msg.Content, "TASK_RESULT:") &&
			strings.Contains(msg.Content, "Search for") &&
			strings.Contains(msg.Content, "sample.json") {

			if strings.Contains(msg.Content, "FOUND FILES MATCHING NAME") ||
				strings.Contains(msg.Content, "NO FILES FOUND MATCHING NAME") {
				foundInOptimized = true
				break
			}
		}
	}

	// This will fail if the bug is present
	if !foundInOptimized {
		t.Error("BUG CONFIRMED: Filename search results were lost during context optimization")

		// Print debug info
		t.Log("Original message count:", len(largeMessages))
		t.Log("Optimized message count:", len(optimizedMessages))

		// Check which messages were preserved
		for i, msg := range optimizedMessages {
			if strings.Contains(msg.Content, "TASK_RESULT") {
				t.Logf("Message %d contains TASK_RESULT but not filename markers: %s",
					i, truncateForTest(msg.Content, 100))
			}
		}
	}
}

// ContextTestMockChat implements ChatSession for testing context optimization
type ContextTestMockChat struct {
	messages []llm.Message
}

func (m *ContextTestMockChat) AddMessage(message llm.Message) error {
	m.messages = append(m.messages, message)
	return nil
}

func (m *ContextTestMockChat) GetMessages() []llm.Message {
	return m.messages
}

// Helper function to truncate strings for test output
func truncateForTest(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
