package task

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// InteractiveSession represents an interactive shell session
type InteractiveSession struct {
	cmd           *exec.Cmd
	stdin         io.WriteCloser
	stdout        io.ReadCloser
	stderr        io.ReadCloser
	outputBuffer  *bytes.Buffer
	errorBuffer   *bytes.Buffer
	inputChannel  chan string
	outputChannel chan string
	errorChannel  chan error
	done          chan bool
	mutex         sync.Mutex
	isRunning     bool
}

// InteractiveExecutor handles interactive shell command execution
type InteractiveExecutor struct {
	workspacePath string
	enableShell   bool
}

// NewInteractiveExecutor creates a new interactive executor
func NewInteractiveExecutor(workspacePath string, enableShell bool) *InteractiveExecutor {
	return &InteractiveExecutor{
		workspacePath: workspacePath,
		enableShell:   enableShell,
	}
}

// ExecuteInteractiveCommand executes a shell command with interactive capabilities
func (ie *InteractiveExecutor) ExecuteInteractiveCommand(task *Task) *TaskResponse {
	response := &TaskResponse{Task: *task}

	if !ie.enableShell {
		response.Error = "shell execution is disabled (set enable_shell: true in config)"
		return response
	}

	// Determine execution mode based on task configuration
	switch task.InputMode {
	case "auto":
		return ie.executeAutoInteractive(task)
	case "prompt":
		return ie.executeWithUserPrompts(task)
	case "predefined":
		return ie.executeWithPredefinedInput(task)
	default:
		// For backward compatibility, detect if command is likely interactive
		if ie.isLikelyInteractive(task.Command) {
			task.Interactive = true
			task.InputMode = "prompt"
			return ie.executeWithUserPrompts(task)
		}
		// Fall back to regular execution for non-interactive commands
		return ie.executeRegularCommand(task)
	}
}

// executeAutoInteractive automatically handles known interactive patterns
func (ie *InteractiveExecutor) executeAutoInteractive(task *Task) *TaskResponse {
	response := &TaskResponse{Task: *task}

	// Create timeout context
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(task.Timeout)*time.Second)
	defer cancel()

	session, err := ie.startInteractiveSession(ctx, task.Command)
	if err != nil {
		response.Error = fmt.Sprintf("failed to start interactive session: %v", err)
		return response
	}
	defer session.cleanup()

	// Handle known interactive patterns automatically
	autoResponses := ie.getAutoResponses(task.Command)

	// Start output monitoring
	go session.monitorOutput()

	// Handle automatic responses based on detected prompts
	for {
		select {
		case output := <-session.outputChannel:
			// Check if output contains a known prompt
			if autoResponse := ie.findAutoResponse(output, autoResponses); autoResponse != "" {
				session.sendInput(autoResponse)
			}
		case err := <-session.errorChannel:
			if err != nil {
				response.Error = fmt.Sprintf("command execution error: %v", err)
				return response
			}
		case <-session.done:
			// Command completed
			response.Success = true
			response.Output = ie.formatInteractiveOutput(session)
			return response
		case <-ctx.Done():
			response.Error = fmt.Sprintf("command timed out after %d seconds", task.Timeout)
			return response
		}
	}
}

// executeWithUserPrompts executes command and prompts user for input when needed
func (ie *InteractiveExecutor) executeWithUserPrompts(task *Task) *TaskResponse {
	response := &TaskResponse{Task: *task}

	// For user prompts, we need to provide a preview and explanation
	preview := ie.buildInteractivePreview(task)

	response.Success = true
	response.Output = preview
	response.ActualContent = fmt.Sprintf("Interactive command ready: %s\n\nThis command requires user interaction. When approved, you'll be prompted for input as needed.\n\nExpected interactions:\n%s",
		task.Command, ie.describeExpectedInteractions(task))

	// The actual execution will happen in ApplyInteractiveTask after user confirmation
	return response
}

// executeWithPredefinedInput executes with predefined responses
func (ie *InteractiveExecutor) executeWithPredefinedInput(task *Task) *TaskResponse {
	response := &TaskResponse{Task: *task}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(task.Timeout)*time.Second)
	defer cancel()

	session, err := ie.startInteractiveSession(ctx, task.Command)
	if err != nil {
		response.Error = fmt.Sprintf("failed to start interactive session: %v", err)
		return response
	}
	defer session.cleanup()

	go session.monitorOutput()

	// Process predefined inputs
	inputIndex := 0

	for {
		select {
		case output := <-session.outputChannel:
			// Check if output matches any expected prompts
			if promptMatch := ie.findMatchingPrompt(output, task.ExpectedPrompts); promptMatch != nil {
				session.sendInput(promptMatch.Response)
			} else if inputIndex < len(task.PredefinedInput) {
				// Use predefined input in order
				session.sendInput(task.PredefinedInput[inputIndex])
				inputIndex++
			}
		case err := <-session.errorChannel:
			if err != nil {
				response.Error = fmt.Sprintf("command execution error: %v", err)
				return response
			}
		case <-session.done:
			response.Success = true
			response.Output = ie.formatInteractiveOutput(session)
			return response
		case <-ctx.Done():
			response.Error = fmt.Sprintf("command timed out after %d seconds", task.Timeout)
			return response
		}
	}
}

// executeRegularCommand falls back to regular non-interactive execution
func (ie *InteractiveExecutor) executeRegularCommand(task *Task) *TaskResponse {
	response := &TaskResponse{Task: *task}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(task.Timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", task.Command)
	cmd.Dir = ie.workspacePath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Command: %s\n", task.Command))
	output.WriteString(fmt.Sprintf("Exit code: %d\n\n", cmd.ProcessState.ExitCode()))

	if stdout.Len() > 0 {
		output.WriteString("STDOUT:\n")
		output.WriteString(stdout.String())
		output.WriteString("\n")
	}

	if stderr.Len() > 0 {
		output.WriteString("STDERR:\n")
		output.WriteString(stderr.String())
		output.WriteString("\n")
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			response.Error = fmt.Sprintf("command timed out after %d seconds", task.Timeout)
		} else {
			response.Error = fmt.Sprintf("command failed: %v", err)
		}
	} else {
		response.Success = true
	}

	response.Output = output.String()
	return response
}

// ApplyInteractiveTask executes the interactive task after user confirmation
func (ie *InteractiveExecutor) ApplyInteractiveTask(task *Task, userInputChannel chan string) error {
	if !task.Interactive || !task.AllowUserInput {
		return fmt.Errorf("task is not configured for user interaction")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(task.Timeout)*time.Second)
	defer cancel()

	session, err := ie.startInteractiveSession(ctx, task.Command)
	if err != nil {
		return fmt.Errorf("failed to start interactive session: %v", err)
	}
	defer session.cleanup()

	go session.monitorOutput()

	// Handle real-time user interaction
	for {
		select {
		case output := <-session.outputChannel:
			// Display output to user and wait for input if it looks like a prompt
			if ie.looksLikePrompt(output) {
				fmt.Printf("Command output: %s\n", output)
				fmt.Print("Your input: ")

				// Wait for user input
				select {
				case userInput := <-userInputChannel:
					session.sendInput(userInput)
				case <-ctx.Done():
					return fmt.Errorf("user input timed out")
				}
			}
		case err := <-session.errorChannel:
			if err != nil {
				return fmt.Errorf("command execution error: %v", err)
			}
		case <-session.done:
			return nil
		case <-ctx.Done():
			return fmt.Errorf("command timed out after %d seconds", task.Timeout)
		}
	}
}

// Helper methods

func (ie *InteractiveExecutor) startInteractiveSession(ctx context.Context, command string) (*InteractiveSession, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = ie.workspacePath

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	session := &InteractiveSession{
		cmd:           cmd,
		stdin:         stdin,
		stdout:        stdout,
		stderr:        stderr,
		outputBuffer:  &bytes.Buffer{},
		errorBuffer:   &bytes.Buffer{},
		inputChannel:  make(chan string, 10),
		outputChannel: make(chan string, 10),
		errorChannel:  make(chan error, 10),
		done:          make(chan bool, 1),
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	session.isRunning = true
	return session, nil
}

func (session *InteractiveSession) monitorOutput() {
	// Monitor stdout
	go func() {
		scanner := bufio.NewScanner(session.stdout)
		for scanner.Scan() {
			line := scanner.Text()
			session.outputBuffer.WriteString(line + "\n")
			session.outputChannel <- line
		}
		if err := scanner.Err(); err != nil {
			session.errorChannel <- err
		}
	}()

	// Monitor stderr
	go func() {
		scanner := bufio.NewScanner(session.stderr)
		for scanner.Scan() {
			line := scanner.Text()
			session.errorBuffer.WriteString(line + "\n")
			session.outputChannel <- line
		}
		if err := scanner.Err(); err != nil {
			session.errorChannel <- err
		}
	}()

	// Wait for command completion
	go func() {
		err := session.cmd.Wait()
		session.mutex.Lock()
		session.isRunning = false
		session.mutex.Unlock()

		if err != nil {
			session.errorChannel <- err
		}
		session.done <- true
	}()
}

func (session *InteractiveSession) sendInput(input string) error {
	session.mutex.Lock()
	defer session.mutex.Unlock()

	if !session.isRunning {
		return fmt.Errorf("session is not running")
	}

	_, err := session.stdin.Write([]byte(input + "\n"))
	return err
}

func (session *InteractiveSession) cleanup() {
	session.mutex.Lock()
	defer session.mutex.Unlock()

	if session.stdin != nil {
		session.stdin.Close()
	}
	if session.stdout != nil {
		session.stdout.Close()
	}
	if session.stderr != nil {
		session.stderr.Close()
	}

	if session.isRunning && session.cmd.Process != nil {
		session.cmd.Process.Kill()
	}
}

// Detection and analysis methods

func (ie *InteractiveExecutor) isLikelyInteractive(command string) bool {
	// Commands that typically require interaction
	interactivePatterns := []string{
		"npm init",
		"yarn init",
		"git config",
		"ssh-keygen",
		"openssl",
		"gpg",
		"sudo",
		"apt install",
		"yum install",
		"brew install",
		"pip install",
		"docker run.*-it",
		"mysql.*-p",
		"psql.*-W",
	}

	cmdLower := strings.ToLower(command)
	for _, pattern := range interactivePatterns {
		if matched, _ := regexp.MatchString(pattern, cmdLower); matched {
			return true
		}
	}

	return false
}

func (ie *InteractiveExecutor) getAutoResponses(command string) map[string]string {
	responses := make(map[string]string)

	cmdLower := strings.ToLower(command)

	// Auto-responses for common interactive commands
	if strings.Contains(cmdLower, "npm init") {
		responses["package name:"] = ""   // Use default
		responses["version:"] = ""        // Use default
		responses["description:"] = ""    // Use default
		responses["entry point:"] = ""    // Use default
		responses["test command:"] = ""   // Use default
		responses["git repository:"] = "" // Use default
		responses["keywords:"] = ""       // Use default
		responses["author:"] = ""         // Use default
		responses["license:"] = ""        // Use default
		responses["is this ok?"] = "yes"
	}

	if strings.Contains(cmdLower, "git config") {
		responses["username:"] = "loom-user"
		responses["email:"] = "loom@example.com"
	}

	// Add more patterns as needed

	return responses
}

func (ie *InteractiveExecutor) findAutoResponse(output string, responses map[string]string) string {
	outputLower := strings.ToLower(output)

	for prompt, response := range responses {
		if strings.Contains(outputLower, strings.ToLower(prompt)) {
			return response
		}
	}

	return ""
}

func (ie *InteractiveExecutor) findMatchingPrompt(output string, prompts []InteractivePrompt) *InteractivePrompt {
	for _, prompt := range prompts {
		if prompt.IsRegex {
			if matched, _ := regexp.MatchString(prompt.Prompt, output); matched {
				return &prompt
			}
		} else {
			if strings.Contains(strings.ToLower(output), strings.ToLower(prompt.Prompt)) {
				return &prompt
			}
		}
	}
	return nil
}

func (ie *InteractiveExecutor) looksLikePrompt(output string) bool {
	// Simple heuristics to detect prompts
	promptIndicators := []string{
		":",
		"?",
		"[y/n]",
		"[yes/no]",
		"password",
		"continue",
		"confirm",
		"enter",
		"input",
	}

	outputLower := strings.ToLower(strings.TrimSpace(output))

	for _, indicator := range promptIndicators {
		if strings.Contains(outputLower, indicator) {
			return true
		}
	}

	return false
}

func (ie *InteractiveExecutor) buildInteractivePreview(task *Task) string {
	var preview strings.Builder

	preview.WriteString(fmt.Sprintf("🔧 Interactive Command: %s\n\n", task.Command))
	preview.WriteString("This command requires user interaction during execution.\n\n")

	if len(task.ExpectedPrompts) > 0 {
		preview.WriteString("Expected interactions:\n")
		for i, prompt := range task.ExpectedPrompts {
			preview.WriteString(fmt.Sprintf("%d. %s\n", i+1, prompt.Description))
			if prompt.Response != "" {
				preview.WriteString(fmt.Sprintf("   → Response: %s\n", prompt.Response))
			}
		}
		preview.WriteString("\n")
	}

	preview.WriteString("When approved, you'll be prompted for input as needed.\n")
	preview.WriteString("Press 'y' to proceed with interactive execution.")

	return preview.String()
}

func (ie *InteractiveExecutor) describeExpectedInteractions(task *Task) string {
	if len(task.ExpectedPrompts) == 0 {
		return "- Interactive prompts will be handled as they appear"
	}

	var interactions strings.Builder
	for i, prompt := range task.ExpectedPrompts {
		interactions.WriteString(fmt.Sprintf("- %s", prompt.Description))
		if prompt.Response != "" {
			interactions.WriteString(fmt.Sprintf(" (auto-response: %s)", prompt.Response))
		}
		if i < len(task.ExpectedPrompts)-1 {
			interactions.WriteString("\n")
		}
	}

	return interactions.String()
}

func (ie *InteractiveExecutor) formatInteractiveOutput(session *InteractiveSession) string {
	var output strings.Builder

	output.WriteString("=== Interactive Command Output ===\n\n")

	if session.outputBuffer.Len() > 0 {
		output.WriteString("STDOUT:\n")
		output.WriteString(session.outputBuffer.String())
		output.WriteString("\n")
	}

	if session.errorBuffer.Len() > 0 {
		output.WriteString("STDERR:\n")
		output.WriteString(session.errorBuffer.String())
		output.WriteString("\n")
	}

	output.WriteString("=== End Output ===")

	return output.String()
}
