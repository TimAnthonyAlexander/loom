package main_test

import (
	"fmt"
	"loom/task"
	"os"
	"testing"
)

func TestSearchFilenameAndContent(t *testing.T) {
	workspacePath, _ := os.Getwd()
	executor := task.NewExecutor(workspacePath, true, 10*1024*1024)

	searchTask := &task.Task{
		Type:           task.TaskTypeSearch,
		Path:           ".",
		Query:          "tui.go",
		SearchNames:    true,
		CombineResults: true,
	}

	fmt.Println("Executing search for 'tui.go'...")
	response := executor.Execute(searchTask)

	if !response.Success {
		t.Errorf("Search failed: %s", response.Error)
	}

	fmt.Println("Success:", response.Success)
	fmt.Println("Output:", response.Output)
	if response.ActualContent != "" {
		maxLen := 200
		if len(response.ActualContent) < maxLen {
			maxLen = len(response.ActualContent)
		}
		fmt.Println("ActualContent (truncated):", response.ActualContent[:maxLen]+"...")
	}
}
