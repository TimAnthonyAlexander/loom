package task

import (
	"fmt"
	"io/ioutil"
	loomctx "loom/context"
	"loom/indexer"
	"loom/llm"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestUserReportedFilenameSearchBug attempts to precisely reproduce the user-reported bug
// where a message containing "Task: 🔍 Search for 'sample.json' (including filename matches)"
// is not preserved in the context.
func TestUserReportedFilenameSearchBug(t *testing.T) {
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

	// Create a context manager with a deliberately small token limit to force optimization
	contextManager := loomctx.NewContextManager(index, 1500)

	// Set up the key components
	executor := NewExecutor(tempDir, false, 1024*1024)

	// Create a mock chat session
	mockChat := &UserReportedBugMockChat{}

	// Create sequential manager - use nil for LLMAdapter since we don't need it for this test
	manager := NewSequentialTaskManager(executor, nil, mockChat)
	manager.SetContextManager(index, 1500)

	// 1. SIMULATE THE USER-REPORTED BUG

	// Create the EXACT search task as reported by the user
	// This is the key difference - using the exact format reported in the issue
	llmResponse := `Task: 🔍 Search for 'sample.json' (including filename matches)`

	// Parse the task - this may fail as we suspect the task parsing might be part of the issue
	task, _, err := manager.ParseSingleTask(llmResponse)
	if err != nil {
		t.Logf("Failed to parse search task: %v", err)
	}

	// If parsing fails (which is part of the bug), create the task manually
	if task == nil {
		t.Logf("Task parsing failed as expected, creating search task manually")
		task = &Task{
			Type:        TaskTypeSearch,
			Path:        ".",
			Query:       "sample.json",
			SearchNames: true, // This is the critical flag for filename search
		}
	} else {
		t.Logf("Task parsing succeeded: %+v", task)
	}

	// Make sure the task is correctly configured for filename search
	if !task.SearchNames {
		t.Logf("SearchNames was not enabled, enabling it now")
		task.SearchNames = true
	}

	// 2. Execute the search task
	taskResponse := executor.Execute(task)

	// Verify the task was successful
	if !taskResponse.Success {
		t.Fatalf("Search task failed: %s", taskResponse.Error)
	}

	// Verify task result contains filename match marker
	if !strings.Contains(taskResponse.ActualContent, "FOUND FILES MATCHING NAME") {
		t.Logf("Expected task response to contain 'FOUND FILES MATCHING NAME', but it didn't: %s",
			truncateString(taskResponse.ActualContent, 200))

		// Check what's actually in the response
		t.Logf("Actual task response content: %s", truncateString(taskResponse.ActualContent, 300))
	} else {
		t.Logf("Confirmed: Task response contains FOUND FILES MATCHING NAME")
	}

	// 3. Format the task result as the sequential manager would
	taskResultMsg := manager.formatTaskResultForExploration(task, taskResponse)

	// Verify the message role is "system" (critical for optimization)
	if taskResultMsg.Role != "system" {
		t.Errorf("Expected task result message role to be 'system', got '%s'", taskResultMsg.Role)
	}

	// Verify the formatted message contains the filename marker
	if !strings.Contains(taskResultMsg.Content, "FOUND FILES MATCHING NAME") {
		t.Errorf("Expected formatted message to contain 'FOUND FILES MATCHING NAME', got: %s",
			truncateString(taskResultMsg.Content, 200))
	} else {
		t.Logf("Confirmed: Formatted message contains FOUND FILES MATCHING NAME")
	}

	// 4. Manually add the message to exploration context and chat session
	manager.addToExplorationContext(taskResultMsg)
	mockChat.AddMessage(taskResultMsg)

	// Create a large set of messages to force context optimization
	messages := createTestConversation(50)

	// Add our special task result message
	messages = append(messages, taskResultMsg)

	// Log the key information before optimization
	t.Logf("Before optimization - Original message count: %d", len(messages))
	t.Logf("Checking message content for filename markers before optimization...")

	foundBeforeOpt := false
	for i, msg := range messages {
		if strings.Contains(msg.Content, "FOUND FILES MATCHING NAME") {
			foundBeforeOpt = true
			t.Logf("Found filename marker in message %d before optimization", i)
			break
		}
	}

	if !foundBeforeOpt {
		t.Logf("WARNING: No filename marker found in any message before optimization!")
	}

	// 5. CRITICAL TEST: Context optimization
	optimizedContext, err := contextManager.OptimizeMessages(messages)
	if err != nil {
		t.Fatalf("Context optimization failed: %v", err)
	}

	// Log optimization results
	t.Logf("After optimization - Optimized message count: %d", len(optimizedContext))

	// Now check if the filename search results were preserved in the optimized context
	foundFilenameResults := false
	for i, msg := range optimizedContext {
		if strings.Contains(msg.Content, "FOUND FILES MATCHING NAME") {
			foundFilenameResults = true
			t.Logf("Found filename marker in message %d after optimization", i)
			break
		}
	}

	// Check if the bug is present
	if !foundFilenameResults {
		t.Error("BUG CONFIRMED: Filename search results were lost during context optimization!")

		// Check if any search result messages were preserved but without filename markers
		for i, msg := range optimizedContext {
			if strings.Contains(msg.Content, "TASK_RESULT:") &&
				strings.Contains(msg.Content, "Search for") &&
				strings.Contains(msg.Content, "sample.json") {
				t.Logf("Message %d contains search results but not filename markers: %s",
					i, truncateString(msg.Content, 300))
			}
		}
	} else {
		t.Logf("Filename search results were correctly preserved in the optimized context")
	}
}

// createTestConversation creates a conversation with the specified number of messages
func createTestConversation(count int) []llm.Message {
	var messages []llm.Message

	// Start with a system message
	systemMsg := llm.Message{
		Role:      "system",
		Content:   "You are an AI coding assistant.",
		Timestamp: time.Now(),
	}
	messages = append(messages, systemMsg)

	// Add initial user message
	userMsg := llm.Message{
		Role:      "user",
		Content:   "I want to analyze this codebase.",
		Timestamp: time.Now(),
	}
	messages = append(messages, userMsg)

	// Add a sequence of messages to simulate a conversation
	for i := 0; i < count/2; i++ {
		messages = append(messages, llm.Message{
			Role:      "assistant",
			Content:   fmt.Sprintf("I'll help you analyze this codebase. Here's some information about file %d.", i),
			Timestamp: time.Now(),
		})

		messages = append(messages, llm.Message{
			Role:      "user",
			Content:   fmt.Sprintf("Can you tell me more about component %d?", i),
			Timestamp: time.Now(),
		})
	}

	// Add the search request as the last user message
	messages = append(messages, llm.Message{
		Role:      "user",
		Content:   "Can you search for sample.json file in this codebase?",
		Timestamp: time.Now(),
	})

	// Add the assistant's acknowledgment
	messages = append(messages, llm.Message{
		Role:      "assistant",
		Content:   "I'll search for the sample.json file.",
		Timestamp: time.Now(),
	})

	return messages
}

// UserReportedBugMockChat implements the ChatSession interface for testing
type UserReportedBugMockChat struct {
	messages []llm.Message
}

func (m *UserReportedBugMockChat) AddMessage(message llm.Message) error {
	m.messages = append(m.messages, message)
	return nil
}

func (m *UserReportedBugMockChat) GetMessages() []llm.Message {
	return m.messages
}
