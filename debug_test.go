package main

import (
	"fmt"
	"loom/task"
)

func main() {
	task.EnableTaskDebug()

	testResponse := `Let me read the configuration file first.
📖 READ config.go
EDIT main.go`

	fmt.Println("Testing response:")
	fmt.Println(testResponse)
	fmt.Println("---")

	taskList, err := task.ParseTasks(testResponse)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if taskList == nil {
		fmt.Println("Result: No tasks (conversational)")
	} else {
		fmt.Printf("Result: Found %d tasks\n", len(taskList.Tasks))
		for i, t := range taskList.Tasks {
			fmt.Printf("  Task %d: %s - %s\n", i+1, t.Type, t.Path)
		}
	}
}
