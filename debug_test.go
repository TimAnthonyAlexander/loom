package main_test

import (
	"fmt"
	"loom/task"
	"os"
	"strings"
	"testing"
)

// This test verifies the behavior of search tasks and message formatting
func TestDebugMessages(t *testing.T) {
	// Enable debug mode
	task.EnableTaskDebug()

	fmt.Println("\n========== DEBUG TEST ==========")

	// Setup basic components
	workspacePath, _ := os.Getwd()
	executor := task.NewExecutor(workspacePath, true, 10*1024*1024)

	// Test direct executor search
	fmt.Println("\n--- TEST: Direct Executor Search ---")

	// 1. First try a filename search
	searchTask := &task.Task{
		Type:           task.TaskTypeSearch,
		Path:           ".",
		Query:          "tui.go",
		SearchNames:    true,
		CombineResults: true,
	}

	fmt.Println("Executing filename search task for 'tui.go'...")
	searchResponse := executor.Execute(searchTask)
	fmt.Println("Search response success:", searchResponse.Success)
	fmt.Println("ActualContent exists:", searchResponse.ActualContent != "")
	fmt.Println("ActualContent contains FOUND FILES:",
		searchResponse.ActualContent != "" &&
			strings.Contains(searchResponse.ActualContent, "FOUND FILES MATCHING NAME"))

	// 2. Then try a code content search
	contentTask := &task.Task{
		Type:           task.TaskTypeSearch,
		Path:           ".",
		Query:          "executeSearch",
		SearchNames:    false,
		CombineResults: false,
	}

	fmt.Println("\nExecuting content search task for 'executeSearch'...")
	contentResponse := executor.Execute(contentTask)
	fmt.Println("Content search response success:", contentResponse.Success)
	fmt.Println("ActualContent exists:", contentResponse.ActualContent != "")

	// 3. Try a sample.json search (which was problematic)
	sampleTask := &task.Task{
		Type:           task.TaskTypeSearch,
		Path:           ".",
		Query:          "sample.json",
		SearchNames:    true,
		CombineResults: false,
	}

	fmt.Println("\nExecuting filename search task for 'sample.json'...")
	sampleResponse := executor.Execute(sampleTask)
	fmt.Println("Sample search response success:", sampleResponse.Success)
	fmt.Println("ActualContent exists:", sampleResponse.ActualContent != "")
	fmt.Println("ActualContent contains FOUND FILES:",
		sampleResponse.ActualContent != "" &&
			strings.Contains(sampleResponse.ActualContent, "FOUND FILES MATCHING NAME"))

	// 4. Test sequential manager format method
	fmt.Println("\n--- TEST: Sequential Manager Formatting ---")
	manager := task.NewSequentialTaskManager(executor, nil, nil)
	messageContent := manager.FormatTaskResultForTest(searchTask, searchResponse)
	fmt.Println("Message content length:", len(messageContent))
	fmt.Println("Message content contains FOUND FILES:",
		strings.Contains(messageContent, "FOUND FILES MATCHING NAME"))

	fmt.Println("\n========== DEBUG TEST COMPLETE ==========")
}
