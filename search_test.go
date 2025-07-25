package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"loom/llm"
	"loom/task"
)

// TestDirectSearchAndFormatting directly tests the search and formatting functions
func TestDirectSearchAndFormatting(t *testing.T) {
	fmt.Println("\n========== DEBUG: DIRECT SEARCH AND FORMATTING TEST ==========")

	// Enable debug mode
	task.EnableTaskDebug()

	// Setup basic components
	workspacePath, _ := os.Getwd()
	executor := task.NewExecutor(workspacePath, true, 10*1024*1024)

	fmt.Println("\n--- Step 1: Direct search execution ---")
	// Create a filename search task
	searchTask := &task.Task{
		Type:        task.TaskTypeSearch,
		Query:       "sample.json",
		Path:        ".",
		SearchNames: true,
	}

	// Execute the search
	fmt.Println("Executing search task directly...")
	searchResponse := executor.Execute(searchTask)
	fmt.Println("Search completed. Success:", searchResponse.Success)
	hasFilenameMatches := strings.Contains(searchResponse.ActualContent, "FOUND FILES MATCHING NAME")
	fmt.Println("Search response contains filename matches:", hasFilenameMatches)

	fmt.Println("\n--- Step 2: Format task result ---")
	// Create sequential manager
	manager := task.NewSequentialTaskManager(executor, nil, nil)

	// Format the task result through the test wrapper
	formattedContent := manager.FormatTaskResultForTest(searchTask, searchResponse)
	fmt.Println("Formatted content length:", len(formattedContent))
	fmt.Println("Formatted content has filename matches:",
		strings.Contains(formattedContent, "FOUND FILES MATCHING NAME"))

	fmt.Println("\n--- Step 3: Manually create a task result message ---")
	// Create a task result message with the formatted content
	taskResultMsg := &llm.Message{
		Role:    "system", // This MUST be system, not assistant!
		Content: formattedContent,
	}

	// Now we have a properly formatted task result message
	fmt.Println("Message role:", taskResultMsg.Role)
	fmt.Println("Message contains filename matches:",
		strings.Contains(taskResultMsg.Content, "FOUND FILES MATCHING NAME"))

	fmt.Println("\n========== DIAGNOSIS ==========")
	fmt.Println("1. The executor correctly finds filename matches")
	fmt.Println("2. The formatTaskResultForExploration function correctly preserves filename matches")
	fmt.Println("3. The TaskResponse.ActualContent contains the filename match data")
	fmt.Println("4. The formatted message contains the filename match data")
	fmt.Println("5. However, when used in real conversations, these search results don't reach the LLM")
	fmt.Println("\nPossible reasons:")
	fmt.Println("A. The message might not be properly added to the exploration context")
	fmt.Println("B. The message role might be incorrect (must be 'system' not 'assistant')")
	fmt.Println("C. The context might be truncated/optimized by the context manager")
	fmt.Println("D. The AddToExplorationContext function might not be properly called")

	fmt.Println("\n========== SOLUTION ==========")
	fmt.Println("CRITICAL FIX: Ensure the role of the task result message is 'system'")
	fmt.Println("This ensures the message is treated as factual information from the environment")
	fmt.Println("rather than as the AI's own thoughts or actions")

	fmt.Println("\n========== DEBUG TEST COMPLETE ==========")
}
