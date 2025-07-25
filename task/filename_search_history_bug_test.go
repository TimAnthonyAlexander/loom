package task

import (
	"io/ioutil"
	"loom/llm"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// mockChatSession is a minimal ChatSession implementation for testing.
type mockChatSession struct {
	messages []llm.Message
}

func (m *mockChatSession) AddMessage(msg llm.Message) error {
	m.messages = append(m.messages, msg)
	return nil
}
func (m *mockChatSession) GetMessages() []llm.Message { return m.messages }

// TestFilenameSearchResultNotAdded reproduces the bug where performing a filename
// search ("names" mode) does not add the actual search results to the chat
// history delivered back to the LLM.
//
// EXPECTATION (desired behaviour):
//
//	After the search task executes successfully the assistant message that is
//	appended to the chat history should contain the phrase
//	"FOUND FILES MATCHING NAME" together with the file path so that the LLM can
//	read the results and decide what to do next.
//
// ACTUAL (buggy) behaviour at the time this test is written:
//
//	The assistant message only contains a generic task header and therefore the
//	assertion below fails – clearly demonstrating the regression.
func TestFilenameSearchResultNotAdded(t *testing.T) {
	// 1. Prepare a temporary workspace that contains a file we can search for.
	workspace := t.TempDir()
	targetFile := filepath.Join(workspace, "sample.json")
	if err := ioutil.WriteFile(targetFile, []byte(`{"name":"loom"}`), 0644); err != nil {
		t.Fatalf("failed to create sample file: %v", err)
	}

	// 2. Wire up the core components used by HandleLLMResponse.
	executor := NewExecutor(workspace /*enableShell*/, false /*maxFileSize*/, 1024*1024)
	chat := &mockChatSession{}
	manager := NewManager(executor /*llmAdapter*/, nil, chat)

	// 3. Craft an LLM response that instructs Loom to search for the filename
	//    (note the emoji + names flag triggers filename-search parsing).
	llmResponse := "🔧 SEARCH sample.json names"

	// Use a buffered channel so the manager can write events without blocking.
	events := make(chan TaskExecutionEvent, 10)
	defer close(events)

	// 4. Run the handler – this parses the request, executes the task and, in
	//    success cases, is expected to append an assistant message containing
	//    the search results to the chat session.
	if _, err := manager.HandleLLMResponse(llmResponse, events); err != nil {
		t.Fatalf("HandleLLMResponse returned error: %v", err)
	}

	// 5. Scan the chat history for the filename search result marker.
	foundMarker := false
	for _, msg := range chat.GetMessages() {
		if msg.Role == "assistant" &&
			(msg.Timestamp.After(time.Time{}) || true) { // ensure we iterate all
			if containsFoundMarker(msg.Content) {
				foundMarker = true
				break
			}
		}
	}

	// 6. We assert that the marker SHOULD be present – the current implementation
	//    misses it, so the test will fail and therefore reliably reproduces the
	//    bug for future debugging.
	if !foundMarker {
		t.Errorf("BUG REPRODUCED: assistant message lacks filename search results – no 'FOUND FILES MATCHING NAME' marker was found in chat history")
	}
}

// containsFoundMarker is a helper that checks for the presence of the result
// section that executeSearch adds when it discovers filename matches.
func containsFoundMarker(content string) bool {
	return strings.Contains(content, "FOUND FILES MATCHING NAME")
}
