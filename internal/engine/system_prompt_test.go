package engine

import (
	"strings"
	"testing"

	"github.com/loom/loom/internal/tool"
)

func TestGenerateSystemPrompt_IncludesToolsAndSafety(t *testing.T) {
	tools := []tool.Schema{
		{Name: "read_file", Description: "Reads a file", Safe: true},
		{Name: "edit_file", Description: "Edit", Safe: false},
	}
	prompt := GenerateSystemPrompt(tools)
	if !strings.Contains(prompt, "Loom System Prompt v") {
		t.Fatalf("missing version header: %q", prompt[:80])
	}
	if !strings.Contains(prompt, "- read_file:") || !strings.Contains(prompt, "- edit_file:") {
		t.Fatalf("tools not listed: %q", prompt)
	}
	if !strings.Contains(prompt, "requires approval") {
		t.Fatalf("missing safety marker for unsafe tool: %q", prompt)
	}
}

func TestGenerateSystemPromptWithRules_IncludesMemoriesBlock(t *testing.T) {
	tools := []tool.Schema{}
	mems := []MemoryEntry{{ID: "pref", Text: "User prefers tabs"}}
	prompt := GenerateSystemPromptWithRules(tools, nil, nil, mems, "")
	if !strings.Contains(prompt, "Memories:") {
		t.Fatalf("missing Memories block")
	}
	if !strings.Contains(prompt, "pref: User prefers tabs") {
		t.Fatalf("missing formatted memory entry")
	}
}
