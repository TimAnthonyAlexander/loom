package task

import (
	"testing"
)

func TestParseTasks(t *testing.T) {
	// Test valid task JSON
	llmResponse := "I'll help you read the main.go file and make some improvements.\n\n" +
		"```json\n" +
		"{\n" +
		"  \"tasks\": [\n" +
		"    {\"type\": \"ReadFile\", \"path\": \"main.go\", \"max_lines\": 100},\n" +
		"    {\"type\": \"EditFile\", \"path\": \"main.go\", \"content\": \"package main\\n\\nfunc main() {\\n\\tprintln(\\\"Hello, World!\\\")\\n}\"}\n" +
		"  ]\n" +
		"}\n" +
		"```\n\n" +
		"Let me start by reading the current file to understand its structure."

	taskList, err := ParseTasks(llmResponse)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if taskList == nil {
		t.Fatal("Expected task list, got nil")
	}

	if len(taskList.Tasks) != 2 {
		t.Fatalf("Expected 2 tasks, got %d", len(taskList.Tasks))
	}

	// Test first task
	task1 := taskList.Tasks[0]
	if task1.Type != TaskTypeReadFile {
		t.Errorf("Expected ReadFile, got %s", task1.Type)
	}
	if task1.Path != "main.go" {
		t.Errorf("Expected main.go, got %s", task1.Path)
	}
	if task1.MaxLines != 100 {
		t.Errorf("Expected 100, got %d", task1.MaxLines)
	}

	// Test second task
	task2 := taskList.Tasks[1]
	if task2.Type != TaskTypeEditFile {
		t.Errorf("Expected EditFile, got %s", task2.Type)
	}
	if task2.Path != "main.go" {
		t.Errorf("Expected main.go, got %s", task2.Path)
	}
}

func TestParseTasksNoJSON(t *testing.T) {
	// Test response with no JSON blocks
	llmResponse := "This is a regular chat response without any tasks.\n" +
		"I can help you understand the code structure and answer questions."

	taskList, err := ParseTasks(llmResponse)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if taskList != nil {
		t.Fatal("Expected nil task list for response without tasks")
	}
}

func TestParseTasksInvalidJSON(t *testing.T) {
	// Test response with malformed JSON
	llmResponse := "I'll help you with that.\n\n" +
		"```json\n" +
		"{\n" +
		"  \"tasks\": [\n" +
		"    {\"type\": \"ReadFile\", \"path\": }  // malformed JSON\n" +
		"  ]\n" +
		"}\n" +
		"```\n\n" +
		"This should fail to parse."

	taskList, err := ParseTasks(llmResponse)
	if err != nil {
		// This is expected for malformed JSON
		return
	}

	if taskList != nil {
		t.Fatal("Expected nil task list for malformed JSON")
	}
}

func TestValidateTask(t *testing.T) {
	// Test valid ReadFile task
	task := &Task{
		Type:     TaskTypeReadFile,
		Path:     "test.go",
		MaxLines: 50,
	}

	if err := validateTask(task); err != nil {
		t.Errorf("Expected valid task, got error: %v", err)
	}

	// Test invalid ReadFile task (no path)
	task = &Task{
		Type:     TaskTypeReadFile,
		MaxLines: 50,
	}

	if err := validateTask(task); err == nil {
		t.Error("Expected error for ReadFile task without path")
	}

	// Test valid EditFile task
	task = &Task{
		Type:    TaskTypeEditFile,
		Path:    "test.go",
		Content: "package main\n\nfunc main() {}",
	}

	if err := validateTask(task); err != nil {
		t.Errorf("Expected valid task, got error: %v", err)
	}

	// Test invalid EditFile task (no content or diff)
	task = &Task{
		Type: TaskTypeEditFile,
		Path: "test.go",
	}

	if err := validateTask(task); err == nil {
		t.Error("Expected error for EditFile task without content or diff")
	}

	// Test valid RunShell task
	task = &Task{
		Type:    TaskTypeRunShell,
		Command: "go build",
		Timeout: 10,
	}

	if err := validateTask(task); err != nil {
		t.Errorf("Expected valid task, got error: %v", err)
	}

	// Test RunShell task with default timeout
	task = &Task{
		Type:    TaskTypeRunShell,
		Command: "go test",
	}

	if err := validateTask(task); err != nil {
		t.Errorf("Expected valid task with default timeout, got error: %v", err)
	}
	if task.Timeout != 3 {
		t.Errorf("Expected default timeout of 3, got %d", task.Timeout)
	}
}

func TestTaskDescription(t *testing.T) {
	// Test ReadFile description
	task := &Task{
		Type:     TaskTypeReadFile,
		Path:     "main.go",
		MaxLines: 100,
	}

	expected := "Read main.go (max 100 lines)"
	if desc := task.Description(); desc != expected {
		t.Errorf("Expected '%s', got '%s'", expected, desc)
	}

	// Test EditFile description
	task = &Task{
		Type:    TaskTypeEditFile,
		Path:    "main.go",
		Content: "new content",
	}

	expected = "Edit main.go (replace content)"
	if desc := task.Description(); desc != expected {
		t.Errorf("Expected '%s', got '%s'", expected, desc)
	}

	// Test ListDir description
	task = &Task{
		Type:      TaskTypeListDir,
		Path:      "src/",
		Recursive: true,
	}

	expected = "List directory src/ (recursive)"
	if desc := task.Description(); desc != expected {
		t.Errorf("Expected '%s', got '%s'", expected, desc)
	}

	// Test RunShell description
	task = &Task{
		Type:    TaskTypeRunShell,
		Command: "go build",
	}

	expected = "Run command: go build"
	if desc := task.Description(); desc != expected {
		t.Errorf("Expected '%s', got '%s'", expected, desc)
	}
}

func TestTaskDestructive(t *testing.T) {
	// ReadFile should not be destructive
	task := &Task{Type: TaskTypeReadFile}
	if task.IsDestructive() {
		t.Error("ReadFile should not be destructive")
	}

	// ListDir should not be destructive
	task = &Task{Type: TaskTypeListDir}
	if task.IsDestructive() {
		t.Error("ListDir should not be destructive")
	}

	// EditFile should be destructive
	task = &Task{Type: TaskTypeEditFile}
	if !task.IsDestructive() {
		t.Error("EditFile should be destructive")
	}

	// RunShell should be destructive
	task = &Task{Type: TaskTypeRunShell}
	if !task.IsDestructive() {
		t.Error("RunShell should be destructive")
	}
}

func TestTaskRequiresConfirmation(t *testing.T) {
	// ReadFile should not require confirmation
	task := &Task{Type: TaskTypeReadFile}
	if task.RequiresConfirmation() {
		t.Error("ReadFile should not require confirmation")
	}

	// EditFile should require confirmation
	task = &Task{Type: TaskTypeEditFile}
	if !task.RequiresConfirmation() {
		t.Error("EditFile should require confirmation")
	}

	// RunShell should require confirmation
	task = &Task{Type: TaskTypeRunShell}
	if !task.RequiresConfirmation() {
		t.Error("RunShell should require confirmation")
	}
}

func TestParseTasksDebugMode(t *testing.T) {
	// Test response that should trigger debug warning (action words but no tasks)
	llmResponse := "I'll create the LICENSE file for you. Let me write this file now."

	taskList, err := ParseTasks(llmResponse)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if taskList != nil {
		t.Fatal("Expected nil task list for response without JSON blocks")
	}

	// Note: Debug output will only appear if LOOM_DEBUG_TASKS=1 is set
}

func TestParseTasksWithRealEditFileTask(t *testing.T) {
	// Test the exact format the AI should use for creating a LICENSE file
	llmResponse := `I'll create the LICENSE file with the MIT License for you.

` + "```json\n" + `{
  "tasks": [
    {"type": "EditFile", "path": "LICENSE", "content": "MIT License\n\nCopyright (c) 2024\n\nPermission is hereby granted..."}
  ]
}
` + "```"

	taskList, err := ParseTasks(llmResponse)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if taskList == nil {
		t.Fatal("Expected task list, got nil")
	}

	if len(taskList.Tasks) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(taskList.Tasks))
	}

	task := taskList.Tasks[0]
	if task.Type != TaskTypeEditFile {
		t.Errorf("Expected EditFile, got %s", task.Type)
	}
	if task.Path != "LICENSE" {
		t.Errorf("Expected LICENSE, got %s", task.Path)
	}
	if task.Content == "" {
		t.Error("Expected content for LICENSE file")
	}
}

func TestParseTasksRejectsLiteralEmitText(t *testing.T) {
	// Test response where AI says "Then emit" instead of actually emitting JSON
	llmResponse := `I'll create the LICENSE file with the MIT License for you.
Then emit: {"tasks": [{"type": "EditFile", "path": "LICENSE", "content": "MIT License..."}]}`

	taskList, err := ParseTasks(llmResponse)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if taskList != nil {
		t.Fatal("Expected nil task list when AI says 'Then emit' instead of using proper JSON code blocks")
	}

	// This should not parse because "Then emit:" is not in a proper JSON code block
}
