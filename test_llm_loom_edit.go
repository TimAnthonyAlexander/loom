package main

import (
	"fmt"
	"loom/chat"
	"loom/llm"
	"loom/task"
	"strings"
	"time"
)

func main() {
	fmt.Println("Testing LLM response with LOOM_EDIT command processing...")
	fmt.Println("============================================================")

	// Simulate the LLM response from the user's example
	llmResponse := `>>LOOM_EDIT file=sample.json v=95407b2ce2f0b6776208c72cd1485fc67d1b0d85 REPLACE 3
#OLD_HASH:d50e9b3f036c9a3e4d60d55f0a3e753d8a297038
    "name": "Chili",
<<LOOM_EDIT

OBJECTIVE_COMPLETE: Renamed "Sample Item" to "Chili" in sample.json.`

	fmt.Printf("LLM Response:\n%s\n\n", llmResponse)

	// Test 1: Task parsing
	fmt.Println("1. Testing task parsing...")
	taskList, err := task.ParseTasks(llmResponse)
	if err != nil {
		fmt.Printf("❌ Task parsing failed: %v\n", err)
		return
	}

	if taskList == nil || len(taskList.Tasks) == 0 {
		fmt.Printf("❌ No tasks found in LLM response\n")
		return
	}

	fmt.Printf("✅ Found %d task(s)\n", len(taskList.Tasks))
	for i, t := range taskList.Tasks {
		fmt.Printf("   Task %d: %s -> %s (LoomEditCommand: %t)\n", 
			i+1, t.Type, t.Path, t.LoomEditCommand)
	}

	// Test 2: Chat session handling
	fmt.Println("\n2. Testing chat session display...")
	session := chat.NewSession(".", 100)

	// Add the LLM response to chat
	assistantMessage := llm.Message{
		Role:      "assistant",
		Content:   llmResponse,
		Timestamp: time.Now(),
	}

	err = session.AddMessage(assistantMessage)
	if err != nil {
		fmt.Printf("❌ Failed to add message to chat: %v\n", err)
		return
	}

	// Get all messages (including system messages)
	allMessages := session.GetMessages()
	fmt.Printf("✅ Chat session contains %d total messages\n", len(allMessages))
	
	// Get display messages
	displayMessages := session.GetDisplayMessages()
	fmt.Printf("   Display messages: %d\n", len(displayMessages))
	
	for i, msg := range allMessages {
		fmt.Printf("   All Message %d (%s): %s\n", i+1, msg.Role, 
			func() string {
				if len(msg.Content) > 80 {
					return msg.Content[:80] + "..."
				}
				return msg.Content
			}())
	}
	
	for i, msg := range displayMessages {
		fmt.Printf("   Display Message %d: %s\n", i+1, 
			func() string {
				if len(msg) > 80 {
					return msg[:80] + "..."
				}
				return msg
			}())
	}

	// Test 3: Check if LOOM_EDIT is in the display
	fmt.Println("\n3. Testing LOOM_EDIT visibility in display...")
	found := false
	for _, msg := range displayMessages {
		if strings.Contains(msg, ">>LOOM_EDIT") {
			fmt.Printf("✅ LOOM_EDIT command found in display messages\n")
			found = true
			break
		}
	}
	
	if !found {
		fmt.Printf("❌ LOOM_EDIT command NOT found in display messages\n")
		fmt.Println("   This explains why the command disappears from chat!")
	}

	fmt.Println("\n4. Conclusion:")
	if taskList != nil && len(taskList.Tasks) > 0 && found {
		fmt.Println("✅ Task parsing works AND message displays properly")
		fmt.Println("   The issue must be in task execution")
	} else if taskList != nil && len(taskList.Tasks) > 0 && !found {
		fmt.Println("❌ Task parsing works BUT message is filtered from display")
		fmt.Println("   This explains the disappearing message")
	} else {
		fmt.Println("❌ Task parsing failed")
		fmt.Println("   This explains why nothing gets executed")
	}
} 