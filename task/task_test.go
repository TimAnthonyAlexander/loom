package task

import (
	"strings"
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

func TestParseTasksFallbackRawJSON(t *testing.T) {
	// Test raw JSON without backticks (fallback parsing)
	llmResponse := "OBJECTIVE: Read the main file to understand the project structure\n\n" +
		"{\"type\": \"ReadFile\", \"path\": \"main.go\", \"max_lines\": 200}\n\n" +
		"Starting exploration now."

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

	// Test the parsed task
	task := taskList.Tasks[0]
	if task.Type != TaskTypeReadFile {
		t.Errorf("Expected ReadFile, got %s", task.Type)
	}
	if task.Path != "main.go" {
		t.Errorf("Expected main.go, got %s", task.Path)
	}
	if task.MaxLines != 200 {
		t.Errorf("Expected 200, got %d", task.MaxLines)
	}
}

func TestParseTasksFallbackRawJSONMultiple(t *testing.T) {
	// Test multiple raw JSON lines (should pick the first valid one)
	llmResponse := "OBJECTIVE: Understand the project structure\n\n" +
		"This is some explanatory text.\n" +
		"{\"type\": \"ListDir\", \"path\": \".\", \"recursive\": false}\n" +
		"Some more text.\n" +
		"{\"type\": \"ReadFile\", \"path\": \"README.md\", \"max_lines\": 100}\n" +
		"End of response."

	taskList, err := ParseTasks(llmResponse)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if taskList == nil {
		t.Fatal("Expected task list, got nil")
	}

	if len(taskList.Tasks) != 1 {
		t.Fatalf("Expected 1 task (first valid one), got %d", len(taskList.Tasks))
	}

	// Should pick the first valid task
	task := taskList.Tasks[0]
	if task.Type != TaskTypeListDir {
		t.Errorf("Expected ListDir, got %s", task.Type)
	}
	if task.Path != "." {
		t.Errorf("Expected '.', got %s", task.Path)
	}
}

func TestParseTasksPreferBackticksOverRaw(t *testing.T) {
	// Test that backtick-wrapped JSON is preferred over raw JSON
	llmResponse := "I'll start by reading the file.\n\n" +
		"{\"type\": \"ReadFile\", \"path\": \"wrong.go\", \"max_lines\": 50}\n\n" +
		"```json\n" +
		"{\"type\": \"ReadFile\", \"path\": \"correct.go\", \"max_lines\": 100}\n" +
		"```\n\n" +
		"This should use the backtick version."

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

	// Should prefer the backtick-wrapped version
	task := taskList.Tasks[0]
	if task.Path != "correct.go" {
		t.Errorf("Expected correct.go (from backticks), got %s", task.Path)
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

func TestParseSearchTask(t *testing.T) {
	// Test basic search
	task := parseSearchTask("main")
	if task == nil {
		t.Fatal("Expected search task, got nil")
	}
	if task.Type != TaskTypeSearch {
		t.Errorf("Expected TaskTypeSearch, got %s", task.Type)
	}
	if task.Query != "main" {
		t.Errorf("Expected query 'main', got '%s'", task.Query)
	}
	if task.MaxResults != 100 {
		t.Errorf("Expected default MaxResults 100, got %d", task.MaxResults)
	}

	// Test search with file types
	task = parseSearchTask("func type:go,js")
	if task.Query != "func" {
		t.Errorf("Expected query 'func', got '%s'", task.Query)
	}
	if len(task.FileTypes) != 2 || task.FileTypes[0] != "go" || task.FileTypes[1] != "js" {
		t.Errorf("Expected FileTypes [go, js], got %v", task.FileTypes)
	}

	// Test search with options
	task = parseSearchTask("error case-insensitive whole-word context:3 max:50")
	if task.Query != "error" {
		t.Errorf("Expected query 'error', got '%s'", task.Query)
	}
	if !task.IgnoreCase {
		t.Error("Expected IgnoreCase to be true")
	}
	if !task.WholeWord {
		t.Error("Expected WholeWord to be true")
	}
	if task.ContextBefore != 3 || task.ContextAfter != 3 {
		t.Errorf("Expected context 3/3, got %d/%d", task.ContextBefore, task.ContextAfter)
	}
	if task.MaxResults != 50 {
		t.Errorf("Expected MaxResults 50, got %d", task.MaxResults)
	}

	// Test search with directory
	task = parseSearchTask("TODO in:src/")
	if task.Query != "TODO" {
		t.Errorf("Expected query 'TODO', got '%s'", task.Query)
	}
	if task.Path != "src/" {
		t.Errorf("Expected path 'src/', got '%s'", task.Path)
	}
}

func TestValidateSearchTask(t *testing.T) {
	// Test valid search task
	task := &Task{
		Type:       TaskTypeSearch,
		Query:      "pattern",
		Path:       ".",
		MaxResults: 100,
	}
	if err := validateTask(task); err != nil {
		t.Errorf("Expected valid search task, got error: %v", err)
	}

	// Test search task without query
	task = &Task{
		Type: TaskTypeSearch,
		Path: ".",
	}
	if err := validateTask(task); err == nil {
		t.Error("Expected error for search task without query")
	}

	// Test search task gets defaults
	task = &Task{
		Type:  TaskTypeSearch,
		Query: "pattern",
	}
	if err := validateTask(task); err != nil {
		t.Errorf("Expected valid task, got error: %v", err)
	}
	if task.Path != "." {
		t.Errorf("Expected default path '.', got '%s'", task.Path)
	}
	if task.MaxResults != 100 {
		t.Errorf("Expected default MaxResults 100, got %d", task.MaxResults)
	}
}

func TestSearchTaskNaturalLanguageParsing(t *testing.T) {
	// Test emoji format
	llmResponse1 := "I'll search for that pattern.\n\n🔧 SEARCH \"IndexStats\"\n\nLet me find where it's defined."

	taskList1, err := ParseTasks(llmResponse1)
	if err != nil {
		t.Fatalf("Failed to parse tasks: %v", err)
	}

	if taskList1 == nil || len(taskList1.Tasks) == 0 {
		t.Fatal("Expected to find SEARCH task in emoji format")
	}

	task1 := taskList1.Tasks[0]
	if task1.Type != TaskTypeSearch {
		t.Errorf("Expected TaskTypeSearch, got %s", task1.Type)
	}

	if task1.Query != "IndexStats" {
		t.Errorf("Expected query 'IndexStats', got '%s'", task1.Query)
	}

	// Test simple format
	llmResponse2 := "SEARCH \"IndexStats\""

	taskList2, err := ParseTasks(llmResponse2)
	if err != nil {
		t.Fatalf("Failed to parse tasks: %v", err)
	}

	if taskList2 == nil || len(taskList2.Tasks) == 0 {
		t.Fatal("Expected to find SEARCH task in simple format")
	}

	task2 := taskList2.Tasks[0]
	if task2.Type != TaskTypeSearch {
		t.Errorf("Expected TaskTypeSearch, got %s", task2.Type)
	}

	// Test with options
	llmResponse3 := "🔧 SEARCH \"pattern\" type:go context:2"

	taskList3, err := ParseTasks(llmResponse3)
	if err != nil {
		t.Fatalf("Failed to parse tasks: %v", err)
	}

	if taskList3 == nil || len(taskList3.Tasks) == 0 {
		t.Fatal("Expected to find SEARCH task with options")
	}

	task3 := taskList3.Tasks[0]
	if task3.Type != TaskTypeSearch {
		t.Errorf("Expected TaskTypeSearch, got %s", task3.Type)
	}

	if task3.Query != "pattern" {
		t.Errorf("Expected query 'pattern', got '%s'", task3.Query)
	}

	if len(task3.FileTypes) == 0 || task3.FileTypes[0] != "go" {
		t.Errorf("Expected FileTypes to contain 'go', got %v", task3.FileTypes)
	}

	if task3.ContextBefore != 2 || task3.ContextAfter != 2 {
		t.Errorf("Expected context 2/2, got %d/%d", task3.ContextBefore, task3.ContextAfter)
	}
}

// TestTaskDescription tests the Description method for various task types
func TestTaskDescription(t *testing.T) {
	tests := []struct {
		task     Task
		expected string
	}{
		{
			task:     Task{Type: TaskTypeReadFile, Path: "main.go", MaxLines: 100},
			expected: "Read main.go (max 100 lines)",
		},
		{
			task:     Task{Type: TaskTypeReadFile, Path: "main.go", StartLine: 10, EndLine: 20},
			expected: "Read main.go (lines 10-20)",
		},
		{
			task:     Task{Type: TaskTypeEditFile, Path: "main.go", LoomEditCommand: true},
			expected: "Edit main.go (LOOM_EDIT format)",
		},
		{
			task:     Task{Type: TaskTypeListDir, Path: "src", Recursive: true},
			expected: "List directory src (recursive)",
		},
		{
			task:     Task{Type: TaskTypeRunShell, Command: "go build"},
			expected: "Run command: go build",
		},
	}

	for i, tc := range tests {
		desc := tc.task.Description()
		if desc != tc.expected {
			t.Errorf("Test %d: Expected '%s', got '%s'", i, tc.expected, desc)
		}
	}
}

func TestValidateTaskLineRanges(t *testing.T) {
	// Test valid line range
	task := &Task{
		Type:      TaskTypeReadFile,
		Path:      "test.go",
		StartLine: 10,
		EndLine:   20,
		MaxLines:  50,
	}

	if err := validateTask(task); err != nil {
		t.Errorf("Expected valid task, got error: %v", err)
	}

	// Test invalid line range (start > end)
	task = &Task{
		Type:      TaskTypeReadFile,
		Path:      "test.go",
		StartLine: 20,
		EndLine:   10,
	}

	if err := validateTask(task); err == nil {
		t.Error("Expected error for invalid line range (start > end)")
	}

	// Test negative start line
	task = &Task{
		Type:      TaskTypeReadFile,
		Path:      "test.go",
		StartLine: -5,
	}

	if err := validateTask(task); err == nil {
		t.Error("Expected error for negative start line")
	}

	// Test negative end line
	task = &Task{
		Type:    TaskTypeReadFile,
		Path:    "test.go",
		EndLine: -10,
	}

	if err := validateTask(task); err == nil {
		t.Error("Expected error for negative end line")
	}
}

func TestParseTasksWithLineRanges(t *testing.T) {
	// Test parsing tasks with line range parameters
	llmResponse := "I'll read the specific section of the file.\n\n" +
		"```json\n" +
		"{\n" +
		"  \"tasks\": [\n" +
		"    {\"type\": \"ReadFile\", \"path\": \"main.go\", \"start_line\": 50, \"end_line\": 100, \"max_lines\": 200}\n" +
		"  ]\n" +
		"}\n" +
		"```"

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
	if task.Type != TaskTypeReadFile {
		t.Errorf("Expected ReadFile, got %s", task.Type)
	}
	if task.Path != "main.go" {
		t.Errorf("Expected main.go, got %s", task.Path)
	}
	if task.StartLine != 50 {
		t.Errorf("Expected start_line 50, got %d", task.StartLine)
	}
	if task.EndLine != 100 {
		t.Errorf("Expected end_line 100, got %d", task.EndLine)
	}
	if task.MaxLines != 200 {
		t.Errorf("Expected max_lines 200, got %d", task.MaxLines)
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

	// UPDATED: EditFile no longer requires confirmation - executes immediately
	task = &Task{Type: TaskTypeEditFile}
	if task.RequiresConfirmation() {
		t.Error("EditFile should NOT require confirmation after removing confirmation system")
	}

	// UPDATED: RunShell no longer requires confirmation - executes immediately
	task = &Task{Type: TaskTypeRunShell}
	if task.RequiresConfirmation() {
		t.Error("RunShell should NOT require confirmation after removing confirmation system")
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

func TestParseNaturalLanguageTasks(t *testing.T) {
	// Test natural language task parsing
	llmResponse := `I'll help you read the main.go file and understand the project structure.

🔧 READ main.go (max: 100 lines)

Let me start by examining the main entry point.`

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
	if task.Type != TaskTypeReadFile {
		t.Errorf("Expected ReadFile, got %s", task.Type)
	}
	if task.Path != "main.go" {
		t.Errorf("Expected main.go, got %s", task.Path)
	}
	if task.MaxLines != 100 {
		t.Errorf("Expected 100, got %d", task.MaxLines)
	}
}

func TestParseNaturalLanguageTasksMultiple(t *testing.T) {
	// Test multiple natural language tasks
	llmResponse := `I'll explore the project structure comprehensively.

🔧 READ README.md (max: 300 lines)
🔧 LIST . recursive
🔧 READ main.go

This will give us a complete overview.`

	taskList, err := ParseTasks(llmResponse)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if taskList == nil {
		t.Fatal("Expected task list, got nil")
	}

	if len(taskList.Tasks) != 3 {
		t.Fatalf("Expected 3 tasks, got %d", len(taskList.Tasks))
	}

	// Test first task
	task1 := taskList.Tasks[0]
	if task1.Type != TaskTypeReadFile {
		t.Errorf("Expected ReadFile, got %s", task1.Type)
	}
	if task1.Path != "README.md" {
		t.Errorf("Expected README.md, got %s", task1.Path)
	}
	if task1.MaxLines != 300 {
		t.Errorf("Expected 300, got %d", task1.MaxLines)
	}

	// Test second task
	task2 := taskList.Tasks[1]
	if task2.Type != TaskTypeListDir {
		t.Errorf("Expected ListDir, got %s", task2.Type)
	}
	if task2.Path != "." {
		t.Errorf("Expected '.', got %s", task2.Path)
	}
	if !task2.Recursive {
		t.Error("Expected recursive to be true")
	}

	// Test third task
	task3 := taskList.Tasks[2]
	if task3.Type != TaskTypeReadFile {
		t.Errorf("Expected ReadFile, got %s", task3.Type)
	}
	if task3.Path != "main.go" {
		t.Errorf("Expected main.go, got %s", task3.Path)
	}
	if task3.MaxLines != DefaultMaxLines { // Default value
		t.Errorf("Expected %d (default), got %d", DefaultMaxLines, task3.MaxLines)
	}
}

func TestParseNaturalLanguageTasksSimpleFormat(t *testing.T) {
	// Test simple format without emoji
	llmResponse := `Let me read the configuration file.

READ config.go (lines 50-100)

This will show us the configuration structure.`

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
	if task.Type != TaskTypeReadFile {
		t.Errorf("Expected ReadFile, got %s", task.Type)
	}
	if task.Path != "config.go" {
		t.Errorf("Expected config.go, got %s", task.Path)
	}
	if task.StartLine != 50 {
		t.Errorf("Expected start line 50, got %d", task.StartLine)
	}
	if task.EndLine != 100 {
		t.Errorf("Expected end line 100, got %d", task.EndLine)
	}
}

func TestParseNaturalLanguageRunTask(t *testing.T) {
	// Test run task with timeout
	llmResponse := `Let me run the tests to verify everything works.

🔧 RUN go test ./... (timeout: 60)

This will execute all tests in the project.`

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
	if task.Type != TaskTypeRunShell {
		t.Errorf("Expected RunShell, got %s", task.Type)
	}
	if task.Command != "go test ./..." {
		t.Errorf("Expected 'go test ./...', got %s", task.Command)
	}
	if task.Timeout != 60 {
		t.Errorf("Expected timeout 60, got %d", task.Timeout)
	}
}

func TestParseNaturalLanguagePreferredOverJSON(t *testing.T) {
	// Test that natural language is preferred over JSON when both are present
	llmResponse := `I'll read the main file.

🔧 READ main.go (max: 150 lines)

` + "```json\n" + `{"type": "ReadFile", "path": "wrong.go", "max_lines": 50}` + "\n```"

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

	// Should prefer natural language over JSON
	task := taskList.Tasks[0]
	if task.Path != "main.go" {
		t.Errorf("Expected main.go (from natural language), got %s", task.Path)
	}
	if task.MaxLines != 150 {
		t.Errorf("Expected 150 (from natural language), got %d", task.MaxLines)
	}
}

// Test line-based editing parsing
func TestParseLineBasedEditTasks(t *testing.T) {
	tests := []struct {
		name           string
		llmResponse    string
		expectedPath   string
		expectedTarget int
		expectedStart  int
		expectedEnd    int
		expectedIntent string
	}{
		{
			name:           "Single line edit",
			llmResponse:    "🔧 EDIT main.go:15 -> add error handling",
			expectedPath:   "main.go",
			expectedTarget: 15,
			expectedIntent: "add error handling",
		},
		{
			name:           "Line range edit",
			llmResponse:    "🔧 EDIT config.go:10-20 -> replace database settings",
			expectedPath:   "config.go",
			expectedStart:  10,
			expectedEnd:    20,
			expectedIntent: "replace database settings",
		},
		{
			name:           "Simple format single line",
			llmResponse:    "EDIT utils.js:42 -> fix bug",
			expectedPath:   "utils.js",
			expectedTarget: 42,
			expectedIntent: "fix bug",
		},
		{
			name:           "Simple format range",
			llmResponse:    "EDIT styles.css:5-8 -> update colors",
			expectedPath:   "styles.css",
			expectedStart:  5,
			expectedEnd:    8,
			expectedIntent: "update colors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskList, err := ParseTasks(tt.llmResponse)
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
			if task.Path != tt.expectedPath {
				t.Errorf("Expected path %s, got %s", tt.expectedPath, task.Path)
			}
			if task.TargetLine != tt.expectedTarget {
				t.Errorf("Expected target line %d, got %d", tt.expectedTarget, task.TargetLine)
			}
			if task.TargetStartLine != tt.expectedStart {
				t.Errorf("Expected start line %d, got %d", tt.expectedStart, task.TargetStartLine)
			}
			if task.TargetEndLine != tt.expectedEnd {
				t.Errorf("Expected end line %d, got %d", tt.expectedEnd, task.TargetEndLine)
			}
			if task.Intent != tt.expectedIntent {
				t.Errorf("Expected intent '%s', got '%s'", tt.expectedIntent, task.Intent)
			}
		})
	}
}

// Test READ with line numbers parsing
func TestParseReadWithLineNumbers(t *testing.T) {
	tests := []struct {
		name              string
		llmResponse       string
		expectedPath      string
		expectedShowLines bool
		expectedMaxLines  int
	}{
		{
			name:              "Simple line numbers request",
			llmResponse:       "🔧 READ main.go with line numbers",
			expectedPath:      "main.go",
			expectedShowLines: true,
		},
		{
			name:              "Line numbers with max lines",
			llmResponse:       "🔧 READ config.go (max: 50 lines, with line numbers)",
			expectedPath:      "config.go",
			expectedShowLines: true,
			expectedMaxLines:  50,
		},
		{
			name:              "Simple format with numbers",
			llmResponse:       "READ utils.js with numbers",
			expectedPath:      "utils.js",
			expectedShowLines: true,
		},
		{
			name:              "Numbered variant",
			llmResponse:       "🔧 READ styles.css numbered",
			expectedPath:      "styles.css",
			expectedShowLines: true,
		},
		{
			name:              "Without line numbers",
			llmResponse:       "🔧 READ normal.go",
			expectedPath:      "normal.go",
			expectedShowLines: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskList, err := ParseTasks(tt.llmResponse)
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
			if task.Type != TaskTypeReadFile {
				t.Errorf("Expected ReadFile, got %s", task.Type)
			}
			if task.Path != tt.expectedPath {
				t.Errorf("Expected path %s, got %s", tt.expectedPath, task.Path)
			}
			if task.ShowLineNumbers != tt.expectedShowLines {
				t.Errorf("Expected ShowLineNumbers %v, got %v", tt.expectedShowLines, task.ShowLineNumbers)
			}
			if tt.expectedMaxLines > 0 && task.MaxLines != tt.expectedMaxLines {
				t.Errorf("Expected MaxLines %d, got %d", tt.expectedMaxLines, task.MaxLines)
			}
		})
	}
}

// Test backward compatibility - ensure legacy context-based editing still works
func TestBackwardCompatibilityContextEditing(t *testing.T) {
	llmResponse := `🔧 EDIT README.md -> add Rules section after "## Quick Start"

` + "```" + `markdown
## Rules

These are the project rules.
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
	if task.Path != "README.md" {
		t.Errorf("Expected README.md, got %s", task.Path)
	}

	// Should NOT have line numbers set (legacy mode)
	if task.TargetLine > 0 {
		t.Errorf("Expected no target line for legacy task, got %d", task.TargetLine)
	}

	// Should have context information
	if task.Intent != "add Rules section after \"## Quick Start\"" {
		t.Errorf("Expected intent with context, got %s", task.Intent)
	}

	// Should have content
	if !strings.Contains(task.Content, "## Rules") {
		t.Errorf("Expected content with Rules section, got %s", task.Content)
	}
}

// TestParseEditTaskWithDirectContent - Removing this test since EDIT is forbidden

// TestParseEditTaskWithCodeBlockContent - Removing this test since EDIT is forbidden

func TestParseMemoryTaskBugWithQuotedID(t *testing.T) {
	// Test the specific bug reported by the user
	testCases := []struct {
		name              string
		input             string
		expectedOperation string
		expectedID        string
		expectedContent   string
		shouldSucceed     bool
		description       string
	}{
		{
			name:              "Bug reproduction - quoted ID with colon and content",
			input:             `"CHECK24 Hotelapi App Middleware":\n- Purpose: PHP-based API gateway`,
			expectedOperation: "create", // Should default to create when no operation specified
			expectedID:        "CHECK24 Hotelapi App Middleware",
			expectedContent:   "- Purpose: PHP-based API gateway",
			shouldSucceed:     true,
			description:       "Should handle quoted ID followed by colon and content, defaulting to create operation",
		},
		{
			name:              "Explicit create with quoted ID",
			input:             `create "CHECK24 Hotelapi App Middleware" content:"- Purpose: PHP-based API gateway"`,
			expectedOperation: "create",
			expectedID:        "CHECK24 Hotelapi App Middleware",
			expectedContent:   "- Purpose: PHP-based API gateway",
			shouldSucceed:     true,
			description:       "Should handle explicit create operation with quoted ID",
		},
		{
			name:              "Simple quoted ID without operation",
			input:             `"my-memory-id" content:"some content here"`,
			expectedOperation: "create",
			expectedID:        "my-memory-id",
			expectedContent:   "some content here",
			shouldSucceed:     true,
			description:       "Should default to create when first arg is quoted ID",
		},
		{
			name:              "Quoted ID with colon format - simple",
			input:             `"simple-id": some content here`,
			expectedOperation: "create",
			expectedID:        "simple-id",
			expectedContent:   "some content here",
			shouldSucceed:     true,
			description:       "Should handle simple colon format",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			task := parseMemoryTask(tc.input)

			if tc.shouldSucceed {
				if task == nil {
					t.Fatalf("Expected task to be parsed successfully, got nil. Input: %s", tc.input)
				}

				if task.MemoryOperation != tc.expectedOperation {
					t.Errorf("Expected operation '%s', got '%s'", tc.expectedOperation, task.MemoryOperation)
				}

				if task.MemoryID != tc.expectedID {
					t.Errorf("Expected ID '%s', got '%s'", tc.expectedID, task.MemoryID)
				}

				if task.MemoryContent != tc.expectedContent {
					t.Errorf("Expected content '%s', got '%s'", tc.expectedContent, task.MemoryContent)
				}

				// Test validation
				if err := validateTask(task); err != nil {
					t.Errorf("Task validation failed: %v", err)
				}
			} else {
				if task != nil {
					t.Errorf("Expected parsing to fail, but got task: %+v", task)
				}
			}
		})
	}
}

func TestParseMemoryTaskEndToEnd(t *testing.T) {
	// Test the full parsing pipeline with the problematic input
	llmResponse := `I'll create a memory for this project information.

🔧 MEMORY "CHECK24 Hotelapi App Middleware":\n- Purpose: PHP-based API gateway

This will help remember key details about the project.`

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
	if task.Type != TaskTypeMemory {
		t.Errorf("Expected TaskTypeMemory, got %s", task.Type)
	}

	// Should default to create operation
	if task.MemoryOperation != "create" {
		t.Errorf("Expected operation 'create', got '%s'", task.MemoryOperation)
	}

	// Should extract the quoted ID correctly
	if task.MemoryID != "CHECK24 Hotelapi App Middleware" {
		t.Errorf("Expected ID 'CHECK24 Hotelapi App Middleware', got '%s'", task.MemoryID)
	}

	// Should extract content after colon
	expectedContent := "- Purpose: PHP-based API gateway"
	if task.MemoryContent != expectedContent {
		t.Errorf("Expected content '%s', got '%s'", expectedContent, task.MemoryContent)
	}
}

func TestMemoryParsingNotTooAggressive(t *testing.T) {
	// Test that conversational text mentioning "memory" doesn't get parsed as commands
	testCases := []struct {
		name            string
		llmResponse     string
		shouldParseTask bool
		description     string
	}{
		{
			name:            "Conversational memory mention should not parse",
			llmResponse:     "Memory saved! I'll remember that this is the CHECK24 Hotel App middleware project.",
			shouldParseTask: false,
			description:     "Conversational text starting with 'Memory saved!' should not be treated as a MEMORY command",
		},
		{
			name:            "Task completion message should not parse",
			llmResponse:     "Memory created successfully! The information has been stored.",
			shouldParseTask: false,
			description:     "Success messages should not be parsed as commands",
		},
		{
			name:            "Actual memory command should parse",
			llmResponse:     "🔧 MEMORY create \"project-info\" content:\"CHECK24 Hotel App middleware\"",
			shouldParseTask: true,
			description:     "Actual MEMORY commands with emoji should still work",
		},
		{
			name:            "Simple memory command should parse",
			llmResponse:     "MEMORY create \"test-id\" content:\"test content\"",
			shouldParseTask: true,
			description:     "Simple MEMORY commands should still work",
		},
		{
			name:            "Memory colon format should parse",
			llmResponse:     "MEMORY \"test-id\": test content here",
			shouldParseTask: true,
			description:     "Colon format MEMORY commands should still work",
		},
		{
			name:            "General memory discussion should not parse",
			llmResponse:     "The memory usage of this application is quite high. We should optimize it.",
			shouldParseTask: false,
			description:     "General discussion mentioning memory should not be parsed",
		},
		{
			name:            "Edit command discussion should not parse",
			llmResponse:     "Edit completed successfully! The file has been updated.",
			shouldParseTask: false,
			description:     "Edit completion messages should not be parsed as commands",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			taskList, err := ParseTasks(tc.llmResponse)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			hasTasks := taskList != nil && len(taskList.Tasks) > 0

			if tc.shouldParseTask && !hasTasks {
				t.Errorf("Expected to parse task but none found. Input: %q", tc.llmResponse)
			}

			if !tc.shouldParseTask && hasTasks {
				t.Errorf("Expected no tasks but found %d task(s). Input: %q", len(taskList.Tasks), tc.llmResponse)
				if len(taskList.Tasks) > 0 {
					task := taskList.Tasks[0]
					t.Errorf("Incorrectly parsed task: Type=%s, Operation=%s, ID=%s",
						task.Type, task.MemoryOperation, task.MemoryID)
				}
			}

			// Additional validation for memory tasks
			if hasTasks && taskList.Tasks[0].Type == TaskTypeMemory {
				task := taskList.Tasks[0]

				// Memory tasks should have reasonable IDs, not conversational fragments
				suspiciousIDs := []string{"saved!", "created", "completed", "successfully!", "usage", "high."}
				for _, suspicious := range suspiciousIDs {
					if strings.Contains(task.MemoryID, suspicious) {
						t.Errorf("Memory ID contains suspicious conversational fragment: '%s' in ID: '%s'",
							suspicious, task.MemoryID)
					}
				}
			}
		})
	}
}
