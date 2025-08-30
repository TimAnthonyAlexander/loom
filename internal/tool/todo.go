package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// TodoTask represents a single task in the todo list
type TodoTask struct {
	ID          string     `json:"id"`
	Task        string     `json:"task"`
	Completed   bool       `json:"completed"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// TodoList represents the entire todo list
type TodoList struct {
	Tasks   []TodoTask `json:"tasks"`
	Created time.Time  `json:"created"`
}

// TodoArgs represents the arguments for todo operations
type TodoArgs struct {
	Action string `json:"action"` // "create", "add", "complete", "list", "clear", "remove"
	Task   string `json:"task,omitempty"`
	TaskID string `json:"task_id,omitempty"`
}

// TodoResponse represents the response from todo operations
type TodoResponse struct {
	Success bool      `json:"success"`
	Message string    `json:"message"`
	List    *TodoList `json:"list,omitempty"`
}

// In-memory todo list storage (per session)
var (
	todoListMutex   sync.RWMutex
	currentTodoList *TodoList
	taskIDCounter   int
)

// generateTaskID generates a simple sequential task ID
func generateTaskID() string {
	taskIDCounter++
	return fmt.Sprintf("task_%d", taskIDCounter)
}

// RegisterTodoList registers the todo_list tool which manages todo lists for the LLM.
func RegisterTodoList(registry *Registry) error {
	return registry.Register(Definition{
		Name:        "todo_list",
		Description: "Create and manage a todo list to track tasks and progress during complex workflows.",
		Safe:        true,
		JSONSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"create", "add", "complete", "list", "clear", "remove"},
					"description": "Action to perform: 'create' new list, 'add' task, 'complete' task, 'list' show all, 'clear' all tasks, 'remove' specific task",
				},
				"task": map[string]interface{}{
					"type":        "string",
					"description": "Task description (required for 'add' action)",
				},
				"task_id": map[string]interface{}{
					"type":        "string",
					"description": "Task ID (required for 'complete' and 'remove' actions)",
				},
			},
			"required": []string{"action"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			var args TodoArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, fmt.Errorf("failed to parse arguments: %w", err)
			}
			return handleTodoAction(ctx, args)
		},
	})
}

func handleTodoAction(ctx context.Context, args TodoArgs) (*TodoResponse, error) {
	todoListMutex.Lock()
	defer todoListMutex.Unlock()

	action := strings.ToLower(strings.TrimSpace(args.Action))

	switch action {
	case "create":
		return createTodoList()
	case "add":
		return addTodoTask(args.Task)
	case "complete":
		return completeTodoTask(args.TaskID)
	case "list":
		return listTodoTasks()
	case "clear":
		return clearTodoList()
	case "remove":
		return removeTodoTask(args.TaskID)
	default:
		return &TodoResponse{
			Success: false,
			Message: fmt.Sprintf("## ❌ Error\n\n*Unknown action: **%s***\n\nValid actions: `create`, `add`, `complete`, `list`, `clear`, `remove`", args.Action),
		}, nil
	}
}

func createTodoList() (*TodoResponse, error) {
	currentTodoList = &TodoList{
		Tasks:   make([]TodoTask, 0),
		Created: time.Now(),
	}
	taskIDCounter = 0

	return &TodoResponse{
		Success: true,
		Message: "## 📋 Todo List Created\n\n*Ready to add tasks! Use the todo_list tool to add, complete, and manage tasks.*",
		List:    currentTodoList,
	}, nil
}

func addTodoTask(taskDesc string) (*TodoResponse, error) {
	if currentTodoList == nil {
		_, err := createTodoList()

		if err != nil {
			return nil, err
		}
	}

	taskDesc = strings.TrimSpace(taskDesc)
	if taskDesc == "" {
		return &TodoResponse{
			Success: false,
			Message: "## ❌ Error\n\n*Task description cannot be empty*",
		}, nil
	}

	task := TodoTask{
		ID:        generateTaskID(),
		Task:      taskDesc,
		Completed: false,
		CreatedAt: time.Now(),
	}

	currentTodoList.Tasks = append(currentTodoList.Tasks, task)

	return &TodoResponse{
		Success: true,
		Message: fmt.Sprintf("## ➕ Task Added\n\n**%s** *(ID: %s)*", taskDesc, task.ID),
		List:    currentTodoList,
	}, nil
}

func completeTodoTask(taskID string) (*TodoResponse, error) {
	if currentTodoList == nil {
		return &TodoResponse{
			Success: false,
			Message: "## ❌ Error\n\n*No todo list exists. Create one first.*",
		}, nil
	}

	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return &TodoResponse{
			Success: false,
			Message: "## ❌ Error\n\n*Task ID cannot be empty*",
		}, nil
	}

	for i := range currentTodoList.Tasks {
		if currentTodoList.Tasks[i].ID == taskID {
			if currentTodoList.Tasks[i].Completed {
				return &TodoResponse{
					Success: false,
					Message: fmt.Sprintf("## ⚠️ Warning\n\n*Task **%s** is already completed*", currentTodoList.Tasks[i].Task),
					List:    currentTodoList,
				}, nil
			}

			now := time.Now()
			currentTodoList.Tasks[i].Completed = true
			currentTodoList.Tasks[i].CompletedAt = &now

			return &TodoResponse{
				Success: true,
				Message: fmt.Sprintf("## ✅ Task Completed\n\n~~**%s**~~ *(ID: %s)*", currentTodoList.Tasks[i].Task, taskID),
				List:    currentTodoList,
			}, nil
		}
	}

	return &TodoResponse{
		Success: false,
		Message: fmt.Sprintf("## ❌ Error\n\n*Task with ID '%s' not found*", taskID),
		List:    currentTodoList,
	}, nil
}

func listTodoTasks() (*TodoResponse, error) {
	if currentTodoList == nil {
		return &TodoResponse{
			Success: true,
			Message: "## 📋 Todo List\n\n*No todo list exists yet. Create one to get started!*",
		}, nil
	}

	if len(currentTodoList.Tasks) == 0 {
		return &TodoResponse{
			Success: true,
			Message: "## 📋 Todo List\n\n*List is empty. Add some tasks to get started!*",
			List:    currentTodoList,
		}, nil
	}

	var message strings.Builder
	message.WriteString("## 📋 Todo List\n\n")

	completedCount := 0
	for _, task := range currentTodoList.Tasks {
		if task.Completed {
			message.WriteString(fmt.Sprintf("- [x] ~~%s~~ `%s`\n", task.Task, task.ID))
			completedCount++
		} else {
			message.WriteString(fmt.Sprintf("- [ ] **%s** `%s`\n", task.Task, task.ID))
		}
	}

	// Add progress bar visualization
	progressPercent := int(float64(completedCount) / float64(len(currentTodoList.Tasks)) * 100)
	progressBar := strings.Repeat("█", progressPercent/10) + strings.Repeat("░", 10-progressPercent/10)

	message.WriteString(fmt.Sprintf("\n**Progress:** %s `%d/%d tasks (%d%%)`",
		progressBar, completedCount, len(currentTodoList.Tasks), progressPercent))

	return &TodoResponse{
		Success: true,
		Message: message.String(),
		List:    currentTodoList,
	}, nil
}

func clearTodoList() (*TodoResponse, error) {
	if currentTodoList == nil {
		return &TodoResponse{
			Success: true,
			Message: "## 🗑️ Clear List\n\n*No todo list exists to clear.*",
		}, nil
	}

	clearedCount := len(currentTodoList.Tasks)
	currentTodoList.Tasks = make([]TodoTask, 0)
	taskIDCounter = 0

	return &TodoResponse{
		Success: true,
		Message: fmt.Sprintf("## 🗑️ List Cleared\n\n*Removed %d tasks. Starting fresh!*", clearedCount),
		List:    currentTodoList,
	}, nil
}

func removeTodoTask(taskID string) (*TodoResponse, error) {
	if currentTodoList == nil {
		return &TodoResponse{
			Success: false,
			Message: "## ❌ Error\n\n*No todo list exists.*",
		}, nil
	}

	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return &TodoResponse{
			Success: false,
			Message: "## ❌ Error\n\n*Task ID cannot be empty*",
		}, nil
	}

	for i, task := range currentTodoList.Tasks {
		if task.ID == taskID {
			// Remove task from slice
			currentTodoList.Tasks = append(currentTodoList.Tasks[:i], currentTodoList.Tasks[i+1:]...)

			return &TodoResponse{
				Success: true,
				Message: fmt.Sprintf("## 🗑️ Task Removed\n\n*Deleted:* **%s** *(ID: %s)*", task.Task, taskID),
				List:    currentTodoList,
			}, nil
		}
	}

	return &TodoResponse{
		Success: false,
		Message: fmt.Sprintf("## ❌ Error\n\n*Task with ID '%s' not found*", taskID),
		List:    currentTodoList,
	}, nil
}
