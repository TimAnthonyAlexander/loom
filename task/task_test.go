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

	// Test ReadFile with line range
	task = &Task{
		Type:      TaskTypeReadFile,
		Path:      "main.go",
		StartLine: 50,
		EndLine:   150,
	}

	expected = "Read main.go (lines 50-150)"
	if desc := task.Description(); desc != expected {
		t.Errorf("Expected '%s', got '%s'", expected, desc)
	}

	// Test ReadFile with start line and max lines
	task = &Task{
		Type:      TaskTypeReadFile,
		Path:      "main.go",
		StartLine: 100,
		MaxLines:  50,
	}

	expected = "Read main.go (from line 100, max 50 lines)"
	if desc := task.Description(); desc != expected {
		t.Errorf("Expected '%s', got '%s'", expected, desc)
	}

	// Test ReadFile with just start line
	task = &Task{
		Type:      TaskTypeReadFile,
		Path:      "main.go",
		StartLine: 200,
	}

	expected = "Read main.go (from line 200)"
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
	if task3.MaxLines != 200 { // Default value
		t.Errorf("Expected 200 (default), got %d", task3.MaxLines)
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

func TestParseNaturalLanguageEditTask(t *testing.T) {
	// Test edit task with arrow notation
	llmResponse := `I'll update the main function with error handling.

🔧 EDIT main.go → add error handling and logging

This will improve the robustness of the application.`

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
	if task.Path != "main.go" {
		t.Errorf("Expected main.go, got %s", task.Path)
	}
	if task.Intent != "add error handling and logging" {
		t.Errorf("Expected 'add error handling and logging', got %s", task.Intent)
	}
	if task.Content != "" {
		t.Errorf("Expected empty content, got %s", task.Content)
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

func TestParseNaturalLanguageEditWithCodeBlock(t *testing.T) {
	// Test edit task with code block content
	llmResponse := `I'll create a sample JSON file with proper content.

🔧 EDIT sample.json → create a sample JSON file

` + "```json\n" + `{
  "name": "test",
  "version": "1.0.0",
  "description": "A test file"
}` + "\n```"

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
	if task.Path != "sample.json" {
		t.Errorf("Expected sample.json, got %s", task.Path)
	}
	if task.Intent != "create a sample JSON file" {
		t.Errorf("Expected intent 'create a sample JSON file', got %s", task.Intent)
	}

	expectedContent := `{
  "name": "test",
  "version": "1.0.0",
  "description": "A test file"
}`
	if task.Content != expectedContent {
		t.Errorf("Expected JSON content, got %s", task.Content)
	}
}

func TestParseNaturalLanguageEditWithoutCodeBlock(t *testing.T) {
	// Test edit task without code block content (should only have description)
	llmResponse := `I'll update the configuration file.

🔧 EDIT config.yaml → add database settings

This will add the necessary database configuration.`

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
	if task.Path != "config.yaml" {
		t.Errorf("Expected config.yaml, got %s", task.Path)
	}
	if task.Intent != "add database settings" {
		t.Errorf("Expected intent 'add database settings', got %s", task.Intent)
	}
	if task.Content != "" {
		t.Errorf("Expected empty content, got %s", task.Content)
	}
}
