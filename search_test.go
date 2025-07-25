package main_test

import (
	"fmt"
	"loom/task"
	"os"
	"strings"
	"testing"
	"time"

	"loom/context"
	"loom/indexer"
	"loom/llm"
)

// Test direct executor functionality without context manager
func TestDirectSearch(t *testing.T) {
	workspacePath, _ := os.Getwd()
	executor := task.NewExecutor(workspacePath, true, 10*1024*1024)

	// Test with filename search
	searchTask := &task.Task{
		Type:           task.TaskTypeSearch,
		Path:           ".",
		Query:          "tui.go",
		SearchNames:    true,
		CombineResults: false, // Only do filename search
	}

	fmt.Println("=== TEST: Direct Filename Search ===")
	response := executor.Execute(searchTask)

	if !response.Success {
		t.Errorf("Search failed: %s", response.Error)
	}

	fmt.Println("Output:", response.Output)

	if response.ActualContent == "" {
		t.Errorf("ActualContent is empty for direct filename search")
	} else {
		fmt.Println("ActualContent contains data")
	}

	// Test content search
	contentTask := &task.Task{
		Type:        task.TaskTypeSearch,
		Path:        ".",
		Query:       "executeSearch",
		SearchNames: false,
	}

	fmt.Println("\n=== TEST: Direct Content Search ===")
	contentResponse := executor.Execute(contentTask)

	if !contentResponse.Success {
		t.Errorf("Content search failed: %s", contentResponse.Error)
	}

	fmt.Println("Output:", contentResponse.Output)

	if contentResponse.ActualContent == "" {
		t.Errorf("ActualContent is empty for direct content search")
	} else {
		fmt.Println("ActualContent contains data")
	}
}

// Test the formatting of task results for LLM consumption
func TestFormatTaskResultForLLM(t *testing.T) {
	workspacePath, _ := os.Getwd()
	executor := task.NewExecutor(workspacePath, true, 10*1024*1024)
	manager := task.NewSequentialTaskManager(executor, nil, nil)

	// Test with filename search
	searchTask := &task.Task{
		Type:           task.TaskTypeSearch,
		Path:           ".",
		Query:          "tui.go",
		SearchNames:    true,
		CombineResults: true,
	}

	fmt.Println("=== TEST: Format Task Result for LLM ===")
	fmt.Println("Executing search for 'tui.go'...")
	response := executor.Execute(searchTask)

	if !response.Success {
		t.Errorf("Search failed: %s", response.Error)
	}

	// Get the formatted message for LLM
	message := manager.FormatTaskResultForTest(searchTask, response)
	fmt.Println("\nFormatted LLM Message:")
	fmt.Println(message)

	// Verify the message contains the filename results
	if !contains(message, "FOUND FILES MATCHING NAME") {
		t.Errorf("Formatted LLM message missing filename search results")
	}
}

// Test context manager's handling of search results
func TestContextManagerWithSearchResults(t *testing.T) {
	workspacePath, _ := os.Getwd()
	index := indexer.NewIndex(workspacePath, 10*1024*1024) // Add the maxFileSize parameter
	contextManager := context.NewContextManager(index, 4000)

	// Create a search response with filename results
	searchContent := `TASK_RESULT: 🔍 Search for 'tui.go'
STATUS: Success
CONTENT:
🔍 Search Results for: 'tui.go'
📁 Path: .
🔴 ATTENTION LLM: FOUND 2 FILES MATCHING NAME 'tui.go'
📊 Summary: 2 matching files, 11 content matches in 2 files

──────────────────────────────────────────────────

📄 FOUND FILES MATCHING NAME:

📁 FILE EXISTS: /Users/tim.alexander/loom/tui/enhanced_tui.go
📁 FILE EXISTS: /Users/tim.alexander/loom/tui/tui.go

──────────────────────────────────────────────────

📝 Content Matches:
[Long content matches that might get truncated...]`

	// Create message with search results
	messages := []llm.Message{
		{
			Role:      "system",
			Content:   "System prompt",
			Timestamp: time.Now(),
		},
		{
			Role:      "user",
			Content:   "Find files named tui.go",
			Timestamp: time.Now(),
		},
		{
			Role:      "assistant",
			Content:   searchContent,
			Timestamp: time.Now(),
		},
	}

	// Optimize the messages
	optimized, err := contextManager.OptimizeMessages(messages)
	if err != nil {
		t.Errorf("Failed to optimize messages: %v", err)
		return
	}

	// Check that the search result is still present and contains filename matches
	foundSearchResults := false
	for _, msg := range optimized {
		if strings.Contains(msg.Content, "FOUND FILES MATCHING NAME") {
			foundSearchResults = true
			break
		}
	}

	if !foundSearchResults {
		t.Errorf("Search results with filename matches were lost during context optimization")
	}
}

// Helper function to check if a string contains another string
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
