package todo

import (
	"encoding/json"
	"fmt"
	"loom/paths"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TodoItem represents a single todo item
type TodoItem struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Checked     bool      `json:"checked"`
	Order       int       `json:"order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TodoList represents a complete todo list
type TodoList struct {
	Items     []TodoItem `json:"items"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Active    bool       `json:"active"`
}

// TodoManager manages the todo list for a workspace
type TodoManager struct {
	workspacePath string
	projectPaths  *paths.ProjectPaths
	currentList   *TodoList
}

// TodoOperation represents the type of todo operation
type TodoOperation string

const (
	TodoOpCreate  TodoOperation = "create"
	TodoOpCheck   TodoOperation = "check"
	TodoOpUncheck TodoOperation = "uncheck"
	TodoOpShow    TodoOperation = "show"
	TodoOpClear   TodoOperation = "clear"
)

// TodoResponse represents the response from a todo operation
type TodoResponse struct {
	Success bool      `json:"success"`
	Error   string    `json:"error,omitempty"`
	List    *TodoList `json:"list,omitempty"`
	Message string    `json:"message,omitempty"`
}

// NewTodoManager creates a new todo manager for the workspace
func NewTodoManager(workspacePath string) *TodoManager {
	// Get project paths
	projectPaths, err := paths.NewProjectPaths(workspacePath)
	if err != nil {
		fmt.Printf("Warning: failed to create project paths for todo manager: %v\n", err)
		// Fallback to legacy behavior
		return &TodoManager{
			workspacePath: workspacePath,
			projectPaths:  nil,
		}
	}

	// Ensure project directories exist
	if err := projectPaths.EnsureProjectDir(); err != nil {
		fmt.Printf("Warning: failed to create project directories: %v\n", err)
	}

	tm := &TodoManager{
		workspacePath: workspacePath,
		projectPaths:  projectPaths,
	}

	// Load existing todo list
	if err := tm.Load(); err != nil {
		// If loading fails, start with no todo list
		fmt.Printf("Warning: failed to load todo list: %v\n", err)
	}

	return tm
}

// todoPath returns the path to the todo.json file
func (tm *TodoManager) todoPath() string {
	if tm.projectPaths != nil {
		return filepath.Join(tm.projectPaths.ProjectDir(), "todo.json")
	}
	// Fallback to legacy path
	return filepath.Join(tm.workspacePath, ".loom", "todo.json")
}

// Load loads the todo list from disk
func (tm *TodoManager) Load() error {
	todoPath := tm.todoPath()

	// Check if todo file exists
	if _, err := os.Stat(todoPath); os.IsNotExist(err) {
		// No todo file exists yet, start with no list
		return nil
	}

	data, err := os.ReadFile(todoPath)
	if err != nil {
		return fmt.Errorf("failed to read todo file: %w", err)
	}

	var todoList TodoList
	if err := json.Unmarshal(data, &todoList); err != nil {
		return fmt.Errorf("failed to unmarshal todo list: %w", err)
	}

	tm.currentList = &todoList
	return nil
}

// Save saves the todo list to disk
func (tm *TodoManager) Save() error {
	if tm.currentList == nil {
		// Remove todo file if no current list
		todoPath := tm.todoPath()
		if _, err := os.Stat(todoPath); err == nil {
			return os.Remove(todoPath)
		}
		return nil
	}

	todoPath := tm.todoPath()

	// Ensure parent directory exists
	if tm.projectPaths != nil {
		// Already ensured by EnsureProjectDir in NewTodoManager
	} else {
		// Legacy fallback - ensure .loom directory exists
		loomDir := filepath.Dir(todoPath)
		if err := os.MkdirAll(loomDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	data, err := json.MarshalIndent(tm.currentList, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal todo list: %w", err)
	}

	if err := os.WriteFile(todoPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write todo file: %w", err)
	}

	return nil
}

// CreateTodoList creates a new todo list with the given titles
func (tm *TodoManager) CreateTodoList(titles []string) *TodoResponse {
	if len(titles) < 2 {
		return &TodoResponse{
			Success: false,
			Error:   "todo list must have at least 2 items",
		}
	}

	if len(titles) > 10 {
		return &TodoResponse{
			Success: false,
			Error:   "todo list can have at most 10 items",
		}
	}

	// Clear any existing list
	tm.currentList = nil

	// Create new list
	now := time.Now()
	items := make([]TodoItem, len(titles))
	for i, title := range titles {
		items[i] = TodoItem{
			ID:        fmt.Sprintf("item_%d", i+1),
			Title:     strings.TrimSpace(title),
			Order:     i + 1,
			Checked:   false,
			CreatedAt: now,
			UpdatedAt: now,
		}
	}

	tm.currentList = &TodoList{
		Items:     items,
		CreatedAt: now,
		UpdatedAt: now,
		Active:    true,
	}

	// Save to disk
	if err := tm.Save(); err != nil {
		return &TodoResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to save todo list: %v", err),
		}
	}

	return &TodoResponse{
		Success: true,
		List:    tm.currentList,
		Message: fmt.Sprintf("Created todo list with %d items", len(titles)),
	}
}

// CheckTodoItem marks a todo item as checked (must be done in order)
func (tm *TodoManager) CheckTodoItem(itemOrder int) *TodoResponse {
	if tm.currentList == nil || !tm.currentList.Active {
		return &TodoResponse{
			Success: false,
			Error:   "no active todo list found",
		}
	}

	if itemOrder < 1 || itemOrder > len(tm.currentList.Items) {
		return &TodoResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid item number: %d (must be between 1 and %d)", itemOrder, len(tm.currentList.Items)),
		}
	}

	// Check if this item can be checked (sequential order rule)
	if itemOrder > 1 && !tm.currentList.Items[itemOrder-2].Checked {
		return &TodoResponse{
			Success: false,
			Error:   fmt.Sprintf("cannot check item %d: item %d must be checked first", itemOrder, itemOrder-1),
		}
	}

	// Check if already checked
	if tm.currentList.Items[itemOrder-1].Checked {
		return &TodoResponse{
			Success: false,
			Error:   fmt.Sprintf("item %d is already checked", itemOrder),
		}
	}

	// Mark as checked
	tm.currentList.Items[itemOrder-1].Checked = true
	tm.currentList.Items[itemOrder-1].UpdatedAt = time.Now()
	tm.currentList.UpdatedAt = time.Now()

	// Save to disk
	if err := tm.Save(); err != nil {
		return &TodoResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to save todo list: %v", err),
		}
	}

	// Check if all items are completed
	allCompleted := true
	for _, item := range tm.currentList.Items {
		if !item.Checked {
			allCompleted = false
			break
		}
	}

	var message string
	if allCompleted {
		message = fmt.Sprintf("✅ Checked item %d: '%s' - All TODO items completed! 🎉", itemOrder, tm.currentList.Items[itemOrder-1].Title)
		// Optionally auto-clear completed list
		tm.currentList.Active = false
		tm.Save()
	} else {
		message = fmt.Sprintf("✅ Checked item %d: '%s'", itemOrder, tm.currentList.Items[itemOrder-1].Title)
	}

	return &TodoResponse{
		Success: true,
		List:    tm.currentList,
		Message: message,
	}
}

// UncheckTodoItem unmarks a todo item (for corrections)
func (tm *TodoManager) UncheckTodoItem(itemOrder int) *TodoResponse {
	if tm.currentList == nil || !tm.currentList.Active {
		return &TodoResponse{
			Success: false,
			Error:   "no active todo list found",
		}
	}

	if itemOrder < 1 || itemOrder > len(tm.currentList.Items) {
		return &TodoResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid item number: %d (must be between 1 and %d)", itemOrder, len(tm.currentList.Items)),
		}
	}

	// Check if not checked
	if !tm.currentList.Items[itemOrder-1].Checked {
		return &TodoResponse{
			Success: false,
			Error:   fmt.Sprintf("item %d is not checked", itemOrder),
		}
	}

	// Uncheck this item and all subsequent items (maintain sequential order)
	for i := itemOrder - 1; i < len(tm.currentList.Items); i++ {
		if tm.currentList.Items[i].Checked {
			tm.currentList.Items[i].Checked = false
			tm.currentList.Items[i].UpdatedAt = time.Now()
		}
	}

	tm.currentList.UpdatedAt = time.Now()

	// Save to disk
	if err := tm.Save(); err != nil {
		return &TodoResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to save todo list: %v", err),
		}
	}

	return &TodoResponse{
		Success: true,
		List:    tm.currentList,
		Message: fmt.Sprintf("⬜ Unchecked item %d: '%s'", itemOrder, tm.currentList.Items[itemOrder-1].Title),
	}
}

// ShowTodoList returns the current todo list
func (tm *TodoManager) ShowTodoList() *TodoResponse {
	if tm.currentList == nil || !tm.currentList.Active {
		return &TodoResponse{
			Success: true,
			Message: "No active todo list",
		}
	}

	return &TodoResponse{
		Success: true,
		List:    tm.currentList,
		Message: tm.FormatTodoForDisplay(),
	}
}

// ClearTodoList removes the current todo list
func (tm *TodoManager) ClearTodoList() *TodoResponse {
	if tm.currentList == nil || !tm.currentList.Active {
		return &TodoResponse{
			Success: false,
			Error:   "no active todo list to clear",
		}
	}

	tm.currentList = nil

	// Remove file from disk
	if err := tm.Save(); err != nil {
		return &TodoResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to clear todo list: %v", err),
		}
	}

	return &TodoResponse{
		Success: true,
		Message: "Todo list cleared",
	}
}

// GetCurrentList returns the current active todo list
func (tm *TodoManager) GetCurrentList() *TodoList {
	if tm.currentList != nil && tm.currentList.Active {
		return tm.currentList
	}
	return nil
}

// HasActiveTodoList returns true if there's an active todo list
func (tm *TodoManager) HasActiveTodoList() bool {
	return tm.currentList != nil && tm.currentList.Active
}

// FormatTodoForDisplay formats the todo list for display in chat
func (tm *TodoManager) FormatTodoForDisplay() string {
	if tm.currentList == nil || !tm.currentList.Active {
		return "No active todo list"
	}

	// Count completed items
	checkedCount := 0
	for _, item := range tm.currentList.Items {
		if item.Checked {
			checkedCount++
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📝 **TODO** (%d/%d):\n", checkedCount, len(tm.currentList.Items)))

	for i, item := range tm.currentList.Items {
		status := "⬜"
		if item.Checked {
			status = "✅"
		} else if i > 0 && !tm.currentList.Items[i-1].Checked {
			status = "🔒"
		}

		sb.WriteString(fmt.Sprintf("%s %d. %s\n", status, item.Order, item.Title))
	}

	return sb.String()
}

// FormatTodoForPrompt formats the todo list for inclusion in system prompts
func (tm *TodoManager) FormatTodoForPrompt() string {
	if tm.currentList == nil || !tm.currentList.Active {
		return `## 📝 TODO List Management

**OPTIONAL BUT RECOMMENDED** for complex multi-step tasks:

You can create and manage a sequential TODO list to track progress:

**TODO Commands**:
- TODO create "step1" "step2" "step3" - Create new TODO list (2-10 items)
- TODO check 1 - Mark item 1 as complete (must be done in order)
- TODO uncheck 1 - Unmark item 1 (and all subsequent items)
- TODO show - Display current TODO list
- TODO clear - Remove current TODO list

**Example Usage**:
🔧 TODO create "Read main.go file" "Identify the bug" "Fix the bug" "Test the fix"

**Rules**:
- Items must be checked in sequential order (1, then 2, then 3, etc.)
- Maximum 10 items, minimum 2 items
- Only one active TODO list at a time
- Use for complex tasks that benefit from step-by-step tracking`
	}

	var sb strings.Builder
	sb.WriteString("## 📝 Current TODO List\n\n")

	for i, item := range tm.currentList.Items {
		status := "⬜"
		if item.Checked {
			status = "✅"
		} else if i > 0 && !tm.currentList.Items[i-1].Checked {
			status = "🔒" // Locked because previous item not checked
		}

		sb.WriteString(fmt.Sprintf("%s **%d.** %s\n", status, item.Order, item.Title))
	}

	// Add progress info
	checkedCount := 0
	for _, item := range tm.currentList.Items {
		if item.Checked {
			checkedCount++
		}
	}

	sb.WriteString(fmt.Sprintf("\n**Progress:** %d/%d items completed\n\n", checkedCount, len(tm.currentList.Items)))

	sb.WriteString("**Available TODO Commands**:\n")
	sb.WriteString("- TODO check [number] - Mark item as complete (must be done in order)\n")
	sb.WriteString("- TODO uncheck [number] - Unmark item (unchecks this and all subsequent items)\n")
	sb.WriteString("- TODO show - Display current TODO list\n")
	sb.WriteString("- TODO clear - Remove current TODO list")

	return sb.String()
}

// GetTodoStatus returns a status message about the current todo list
func (tm *TodoManager) GetTodoStatus() string {
	if tm.currentList == nil || !tm.currentList.Active {
		return "No active TODO list. Use 'TODO create \"item1\" \"item2\"...' to create one."
	}

	return tm.FormatTodoForDisplay()
}
