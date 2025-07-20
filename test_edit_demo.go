package main

import (
	"fmt"
	"loom/task"
)

func main() {
	// Test the exact pattern that was broken
	llmResponse := `I'll create a sample JSON file with proper content.

🔧 EDIT sample.json → create a sample JSON file that demonstrates structured data

` + "```json\n" + `{
  "name": "Loom AI",
  "role": "Lead Developer",
  "config": {
    "mode": "autonomous",
    "shellExecution": true,
    "maxConcurrentTasks": 3
  },
  "success": true
}` + "\n```"

	taskList, err := task.ParseTasks(llmResponse)
	if err != nil {
		fmt.Printf("Error parsing tasks: %v\n", err)
		return
	}

	if taskList == nil || len(taskList.Tasks) == 0 {
		fmt.Println("No tasks found")
		return
	}

	editTask := &taskList.Tasks[0]
	fmt.Printf("Task Type: %s\n", editTask.Type)
	fmt.Printf("Path: %s\n", editTask.Path)
	fmt.Printf("Intent: %s\n", editTask.Intent)
	fmt.Printf("Content Length: %d\n", len(editTask.Content))
	fmt.Printf("Content Preview: %s...\n", editTask.Content[:min(len(editTask.Content), 50)])

	// Verify the content is actual JSON, not the description
	if editTask.Content == editTask.Intent {
		fmt.Println("❌ BUG: Content equals intent (description was used as content)")
	} else {
		fmt.Println("✅ FIXED: Content is different from intent (actual content provided)")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
} 