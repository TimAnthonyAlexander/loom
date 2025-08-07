package engine

import (
	"fmt"
	"strings"
	"time"

	"github.com/loom/loom/internal/tool"
)

// GenerateSystemPrompt builds the system prompt shown to the model as the first message.
// It includes a dated header and a rendered list of currently registered tools.
func GenerateSystemPrompt(tools []tool.Schema) string {
	var b strings.Builder

	// Header with rolling date version
	today := time.Now().Format("2006-01-02")
	fmt.Fprintf(&b, "# Loom v2 System Prompt v%s\n\n", today)

	// Tools section
	b.WriteString("Core tools provided:\n")
	if len(tools) == 0 {
		b.WriteString("(none registered)\n\n")
	} else {
		for _, t := range tools {
			safety := "safe"
			if !t.Safe {
				safety = "requires approval"
			}
			fmt.Fprintf(&b, "- %s: %s (%s)\n", t.Name, t.Description, safety)
		}
		b.WriteString("\n")
	}

	// Rules and guidance
	b.WriteString("Rules:\n")
	b.WriteString("• When you need code or file contents, call read_file or search_code. Do not answer from memory.\n")
	b.WriteString("• For edits, prefer the smallest safe change. Read first to determine exact lines.\n")
	b.WriteString("• Destructive actions require user approval. The system will pause and ask the user.\n")
	b.WriteString("• Only call tools that were provided to you. Do not invent tool names or schemas.\n\n")

	b.WriteString("4 . Typical Workflows\n")
	b.WriteString("Exploration: list_dir or search_code → read_file (as needed) → final summary.\n")
	b.WriteString("Editing: read_file to locate lines → edit_file with minimal, precise changes → wait for confirmation → then call apply_edit after approval → final summary.\n\n")

	b.WriteString("7 . Error-Prevention Checklist\n")
	b.WriteString("☑ Prefer relative paths within the workspace (no path escapes).\n")
	b.WriteString("☑ Do not fabricate tool results.\n")
	b.WriteString("☑ One tool call per turn — no commentary alongside.\n")
	b.WriteString("☑ For edits:\n")
	b.WriteString("   • Read first to confirm the target lines and surrounding context.\n")
	b.WriteString("   • Provide the smallest precise change.\n")
	b.WriteString("   • Expect a diff preview and user approval before the edit applies.\n")
	b.WriteString("☑ If a tool returns an error, adjust your plan (read, search, ask for clarification) rather than guessing.\n\n")

	b.WriteString("9 . Memory and Context\n")
	b.WriteString("• Consider any conversation summary and project overview that may be included.\n")
	b.WriteString("• Respect TODOs or decisions from earlier turns.\n")
	b.WriteString("• Avoid echoing secrets verbatim. If you encounter credentials, treat them as redacted.\n\n")

	b.WriteString("10 . Final Message Policy\n")
	b.WriteString("• Final answers should be concise: at most 3 paragraphs. Use bullet points if you must expand.\n")
	b.WriteString("• Do not include raw tool JSON or internal orchestration details in the final answer.\n")
	b.WriteString("• Start your final message with \"Perfect!\" or \"I found the issue\" when you complete a task.\n")
	b.WriteString("• Start with \"Here is what I found\" when you are summarizing findings without a specific fix.\n\n")

	b.WriteString("Follow these rules and the tools provided in the current request. When code access or changes are required, use the tools. Do not guess outputs, do not mix actions, and wait for results before proceeding.\n")

	return b.String()
}
