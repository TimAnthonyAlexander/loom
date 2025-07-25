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

// Helper function to truncate strings for error messages
func truncateStringLocal(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// TestFilenameSearchExtremeConditions tests the bug under extreme context pressure
func TestFilenameSearchExtremeConditions(t *testing.T) {
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

	// Create a context manager with an EXTREMELY small token limit to force aggressive optimization
	contextManager := loomctx.NewContextManager(index, 300) // Ultra small token limit!

	// Set up the key components
	executor := NewExecutor(tempDir, false, 1024*1024)

	// Create a mock chat session
	mockChat := &ExtremeMockChat{}

	// Create sequential manager
	manager := NewSequentialTaskManager(executor, nil, mockChat)
	manager.SetContextManager(index, 300)

	// 1. Create the search task exactly as reported by the user
	task := &Task{
		Type:        TaskTypeSearch,
		Path:        ".",
		Query:       "sample.json",
		SearchNames: true,
	}

	// 2. Execute the search task
	taskResponse := executor.Execute(task)

	// Verify the task was successful
	if !taskResponse.Success {
		t.Fatalf("Search task failed: %s", taskResponse.Error)
	}

	// Format the task result as the sequential manager would
	taskResultMsg := manager.formatTaskResultForExploration(task, taskResponse)

	// Verify message content contains filename marker
	if !strings.Contains(taskResultMsg.Content, "FOUND FILES MATCHING NAME") {
		t.Logf("Expected task response to contain 'FOUND FILES MATCHING NAME', but it didn't: %s",
			truncateStringLocal(taskResultMsg.Content, 300))
	} else {
		t.Logf("Confirmed: Task result message contains FOUND FILES MATCHING NAME")
	}

	// Create a VERY large set of messages to force extreme context optimization
	messages := createExtremeConversation(50) // 100 messages (50 pairs) with very long content

	// Insert our filename search result somewhere in the middle (not at the end)
	// This is important since the bug might be preserving only recent messages
	middleIndex := len(messages) / 2
	messages = append(messages[:middleIndex], append([]llm.Message{taskResultMsg}, messages[middleIndex:]...)...)

	// Log the key information before optimization
	t.Logf("Before optimization - Original message count: %d", len(messages))
	t.Logf("Filename search result is at index %d of %d", middleIndex, len(messages))

	// CRITICAL TEST: Apply extreme context optimization
	optimizedMessages, err := contextManager.OptimizeMessages(messages)
	if err != nil {
		t.Fatalf("Context optimization failed: %v", err)
	}

	// Log optimization results
	t.Logf("After optimization - Messages reduced from %d to %d",
		len(messages), len(optimizedMessages))

	// Now check if the filename search results were preserved in the optimized context
	foundFilenameResults := false
	for i, msg := range optimizedMessages {
		if strings.Contains(msg.Content, "FOUND FILES MATCHING NAME") {
			foundFilenameResults = true
			t.Logf("Found filename marker in message %d after optimization", i)
			break
		}
	}

	// Check for search results messages that lost their filename markers
	searchResultsWithoutMarkers := 0
	for i, msg := range optimizedMessages {
		if strings.Contains(msg.Content, "TASK_RESULT:") &&
			strings.Contains(msg.Content, "Search for") &&
			strings.Contains(msg.Content, "sample.json") &&
			!strings.Contains(msg.Content, "FOUND FILES MATCHING NAME") {

			searchResultsWithoutMarkers++
			t.Logf("Message %d has search results but no filename marker: %s",
				i, truncateStringLocal(msg.Content, 200))
		}
	}

	if searchResultsWithoutMarkers > 0 {
		t.Logf("Found %d search result messages that lost their filename markers",
			searchResultsWithoutMarkers)
	}

	// Check if the bug is present
	if !foundFilenameResults {
		t.Error("BUG CONFIRMED: Filename search results were lost during extreme context optimization!")
	} else {
		t.Logf("Filename search results were correctly preserved even with extreme context optimization")
	}
}

// createExtremeConversation creates a very large conversation
func createExtremeConversation(pairs int) []llm.Message {
	var messages []llm.Message

	// Start with a system message
	systemMsg := llm.Message{
		Role:      "system",
		Content:   "You are an AI coding assistant.",
		Timestamp: time.Now(),
	}
	messages = append(messages, systemMsg)

	// Add a large number of message pairs
	for i := 0; i < pairs; i++ {
		// Create very long messages to consume tokens
		userContent := strings.Repeat(fmt.Sprintf("This is user message %d with lots of repetitive content to consume many tokens. ", i), 20)
		assistantContent := strings.Repeat(fmt.Sprintf("This is assistant response %d with even more repetitive content to consume many tokens. ", i), 40)

		messages = append(messages, llm.Message{
			Role:      "user",
			Content:   userContent,
			Timestamp: time.Now(),
		})

		messages = append(messages, llm.Message{
			Role:      "assistant",
			Content:   assistantContent,
			Timestamp: time.Now(),
		})
	}

	return messages
}

// ExtremeMockChat implements the ChatSession interface for testing
type ExtremeMockChat struct {
	messages []llm.Message
}

func (m *ExtremeMockChat) AddMessage(message llm.Message) error {
	m.messages = append(m.messages, message)
	return nil
}

func (m *ExtremeMockChat) GetMessages() []llm.Message {
	return m.messages
}
