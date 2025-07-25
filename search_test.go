package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"loom/llm"
	"loom/task"
)

// TestExecutionFlow focuses on directly tracing execution without mock LLM adapters
func TestExecutionFlow(t *testing.T) {
	fmt.Println("\n========== DEBUG: DIRECT EXECUTION FLOW TEST ==========")
	
	// Enable debug mode
	task.EnableTaskDebug()
	
	// Setup basic components
	workspacePath, _ := os.Getwd()
	executor := task.NewExecutor(workspacePath, true, 10*1024*1024)
	
	fmt.Println("\n--- Step 1: Direct search task execution ---")
	searchTask := &task.Task{
		Type:        task.TaskTypeSearch,
		Query:       "sample.json",
		Path:        ".",
		SearchNames: true,
	}
	
	fmt.Println("Executing search task directly...")
	searchResponse := executor.Execute(searchTask)
	fmt.Println("Search completed. Success:", searchResponse.Success)
	fmt.Println("Found filename matches:", strings.Contains(searchResponse.ActualContent, "FOUND FILES MATCHING NAME"))
	
	fmt.Println("\n--- Step 2: Sequential manager - direct function calls ---")
	manager := task.NewSequentialTaskManager(executor, nil, nil)
	
	fmt.Println("Step 2a: Calling FormatTaskResultForTest...")
	// This exposes the internal formatTaskResultForExploration function
	resultContent := manager.FormatTaskResultForTest(searchTask, searchResponse)
	fmt.Println("FormatTaskResultForTest returned message with length:", len(resultContent))
	fmt.Println("Formatted message contains filename matches:", strings.Contains(resultContent, "FOUND FILES MATCHING NAME"))
	
	// Create a mock message with the proper role
	fmt.Println("\nStep 2b: Creating a message with the formatted content...")
	mockMsg := llm.Message{
		Role:    "system",  // This is critical - must be system role!
		Content: resultContent,
	}
	
	// Verify the message content directly
	fmt.Printf("Message role: %s, length: %d\n", mockMsg.Role, len(mockMsg.Content))
	fmt.Println("Message contains filename matches:", strings.Contains(mockMsg.Content, "FOUND FILES MATCHING NAME"))
	
	// Get the exploration context
	fmt.Println("\nStep 3: Examining exploration context...")
	context := manager.GetExplorationContext()
	fmt.Printf("Initial exploration context has %d messages\n", len(context))
	
	// Add our formatted message to a new exploration context
	fmt.Println("\nStep 4: Let's call addToExplorationContext directly...")
	// NOTE: This doesn't work because addToExplorationContext is private
	// We can only trace this indirectly
	fmt.Println("Cannot call private method directly - would need to use a public method")
	
	fmt.Println("\n========== DEBUG TEST COMPLETE ==========")
}

func TestDirectFileSearch(t *testing.T) {
	fmt.Println("\n========== DEBUG: DIRECT FILE SEARCH TEST ==========")
	
	// Enable debug mode
	task.EnableTaskDebug()
	
	// Setup components
	workspacePath, _ := os.Getwd()
	executor := task.NewExecutor(workspacePath, true, 10*1024*1024)
	
	// 1. First try a filename-only search
	searchTask := &task.Task{
		Type:        task.TaskTypeSearch,
		Query:       "sample.json",
		Path:        ".",
		SearchNames: true,
		CombineResults: false,  // Don't include content matches - only filenames
	}
	
	fmt.Println("Executing filename-only search task...")
	searchResponse := executor.Execute(searchTask)
	fmt.Println("Search response success:", searchResponse.Success)
	fmt.Println("ActualContent exists:", searchResponse.ActualContent != "")
	hasFilenameMatches := strings.Contains(searchResponse.ActualContent, "FOUND FILES MATCHING NAME")
	fmt.Println("Has filename matches:", hasFilenameMatches)
	
	// 2. Try a combined search (filename + content)
	combinedTask := &task.Task{
		Type:        task.TaskTypeSearch,
		Query:       "sample.json",
		Path:        ".",
		SearchNames: true,
		CombineResults: true,  // Include content matches
	}
	
	fmt.Println("\nExecuting combined search task...")
	combinedResponse := executor.Execute(combinedTask)
	fmt.Println("Combined search response success:", combinedResponse.Success)
	fmt.Println("ActualContent exists:", combinedResponse.ActualContent != "")
	hasCombinedMatches := strings.Contains(combinedResponse.ActualContent, "FOUND FILES MATCHING NAME")
	fmt.Println("Has filename matches:", hasCombinedMatches)
	
	// 3. Let's check with sequential manager
	manager := task.NewSequentialTaskManager(executor, nil, nil)
	
	// Call formatTaskResultForExploration via the test wrapper
	fmt.Println("\nFormatting search result with sequential manager...")
	formattedContent := manager.FormatTaskResultForTest(searchTask, searchResponse)
	fmt.Println("Formatted content length:", len(formattedContent))
	fmt.Println("Formatted content has filename matches:", 
		strings.Contains(formattedContent, "FOUND FILES MATCHING NAME"))
	
	fmt.Println("\n========== DEBUG TEST COMPLETE ==========")
}
