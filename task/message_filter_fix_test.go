package task

import (
	"loom/chat"
	"loom/llm"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMessageFilteringFix tests that internal TASK_RESULT messages are properly filtered out
func TestMessageFilteringFix(t *testing.T) {
	// Create a chat session
	session := chat.NewSession("test_session", 100)

	// Add a user message
	userMsg := llm.Message{
		Role:      "user",
		Content:   "Please edit the file",
		Timestamp: time.Now(),
	}
	session.AddMessage(userMsg)

	// Add an internal TASK_RESULT message (this should be hidden)
	internalMsg := llm.Message{
		Role:      "system",
		Content:   "TASK_RESULT: Edit public/index.html (replace content)\nSTATUS: Success\nCONTENT:\nContent replacement preview for public/index.html:\n\n{\"content\":\"<html>...</html>\"}",
		Timestamp: time.Now(),
	}
	session.AddMessage(internalMsg)

	// Add a proper assistant response
	assistantMsg := llm.Message{
		Role:      "assistant",
		Content:   "I'll help you edit the file. Please confirm the changes.",
		Timestamp: time.Now(),
	}
	session.AddMessage(assistantMsg)

	// Get display messages
	displayMessages := session.GetDisplayMessages()

	// Verify the internal message is filtered out
	foundInternalMessage := false
	for _, msg := range displayMessages {
		if strings.Contains(msg, "TASK_RESULT:") || strings.Contains(msg, "STATUS: Success") {
			foundInternalMessage = true
			t.Errorf("CRITICAL BUG: Internal TASK_RESULT message leaked to display: %s", msg)
		}
	}

	if foundInternalMessage {
		t.Errorf("Message filtering FAILED - internal messages are still leaking")
	} else {
		t.Logf("SUCCESS: Internal messages properly filtered out")
	}

	// Verify we still see the user and assistant messages
	if len(displayMessages) != 2 {
		t.Errorf("Expected 2 display messages (user + assistant), got %d", len(displayMessages))
	}

	// Verify the messages are properly formatted
	expectedMessages := []string{
		"You: Please edit the file",
		"Loom: I'll help you edit the file. Please confirm the changes.",
	}

	for i, expected := range expectedMessages {
		if i >= len(displayMessages) {
			t.Errorf("Missing expected message: %s", expected)
			continue
		}

		if displayMessages[i] != expected {
			t.Errorf("Message %d mismatch.\nExpected: %s\nGot:      %s", i, expected, displayMessages[i])
		}
	}
}

// TestRealWorldScenario tests a real-world scenario with a multi-part LLM response
func TestRealWorldScenario(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create executor and manager
	executor := NewExecutor(tempDir, false, 1024*1024)
	mockChat := &MockChatSession{}
	manager := NewManager(executor, nil, mockChat)

	// Real-world LLM response with explanation and task - now using LOOM_EDIT format
	llmResponse := `I'll create a simple React app for a dentist office. Let's start with setting up the basic HTML structure.

>>LOOM_EDIT file=public/index.html REPLACE 1-1
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Fatih Secilmis Dentist Office</title>
    <link href="https://fonts.googleapis.com/css2?family=Open+Sans:wght@400;600&display=swap" rel="stylesheet">
  </head>
  <body>
    <div id="root"></div>
  </body>
</html>
<<LOOM_EDIT

Next, I'll create the main App.js component that will serve as the entry point for our application.

>>LOOM_EDIT file=src/App.js REPLACE 1-1
import React from 'react';
import './App.css';

function App() {
  return (
    <div className="App">
      <header>
        <h1>Fatih Secilmis Dentist Office</h1>
        <nav>
          <ul>
            <li><a href="#services">Services</a></li>
            <li><a href="#about">About</a></li>
            <li><a href="#contact">Contact</a></li>
          </ul>
        </nav>
      </header>
      <main>
        <section id="hero">
          <h2>Your Smile, Our Passion</h2>
          <p>Professional dental care with a personal touch</p>
          <button className="cta">Book Appointment</button>
        </section>
      </main>
    </div>
  );
}

export default App;
<<LOOM_EDIT`

	// Execute through manager
	eventChan := make(chan TaskExecutionEvent, 10)
	execution, err := manager.HandleLLMResponse(llmResponse, eventChan)
	if err != nil {
		t.Fatalf("Manager failed to handle LLM response: %v", err)
	}

	if execution == nil {
		t.Fatal("No execution or tasks created")
	}

	// Should have found both tasks
	if len(execution.Tasks) != 2 {
		t.Fatalf("Expected 2 tasks, got %d", len(execution.Tasks))
	}

	// Verify all tasks are LOOM_EDIT commands
	for i, task := range execution.Tasks {
		if !task.LoomEditCommand {
			t.Errorf("Task %d not recognized as LOOM_EDIT command", i+1)
		}
	}

	// Verify all tasks succeeded
	for i, response := range execution.Responses {
		if !response.Success {
			t.Errorf("Task %d failed: %s", i+1, response.Error)
		}
	}

	// Verify files were created
	files := []string{
		"public/index.html",
		"src/App.js",
	}

	for i, file := range files {
		fullPath := filepath.Join(tempDir, file)
		if !fileExists(fullPath) {
			t.Errorf("File %d missing: %s", i+1, fullPath)
		}
	}
}
