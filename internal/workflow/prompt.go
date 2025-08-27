package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/loom/loom/internal/profiler"
)

type PromptSlices struct {
	CoreConstraints string
	StateSlice      string
	RecentEvents    string
	RulesExcerpt    string
	Memories        string
	Hotlist         string
}

func BuildPromptFromStore(ctx context.Context, ws string, st *WorkflowState, recent []map[string]any) string {
	_ = ctx
	core := "Constraints: prefer high-importance files; do not modify generated paths; never leave the workspace.\n"
	state := BuildStateSlice(st)
	events := BuildRecentEvents(recent, st.Budget.MaxEventsInPrompt, 400)
	rules := buildRulesExcerpt(ws, st.Budget.TruncateRulesMDChars)
	mems := buildMemoriesExcerpt()
	hot := buildHotlist(ws, 10)

	var b strings.Builder
	if core != "" {
		b.WriteString(core + "\n")
	}
	if state != "" {
		b.WriteString(state + "\n")
	}
	if events != "" {
		b.WriteString(events + "\n")
	}
	if rules != "" {
		b.WriteString(rules + "\n")
	}
	if mems != "" {
		b.WriteString(mems + "\n")
	}
	if hot != "" {
		b.WriteString(hot + "\n")
	}
	return strings.TrimSpace(b.String())
}

func BuildStateSlice(st *WorkflowState) string {
	if st == nil {
		return ""
	}
	return fmt.Sprintf(
		"[workflow]\nphase: %s\ngoal: %s\nactive-plan: %s\napprovals.required: %v\nsafety.risk: %s\nlast_result: %s\n[/workflow]",
		st.Phase, st.Goal, activePlan(st.Plan), st.Approvals.Required, st.Safety.RiskLevel, st.LastResultSummary,
	)
}

func BuildRecentEvents(events []map[string]any, maxN int, _ int) string {
	if maxN <= 0 {
		maxN = 8
	}
	if len(events) > maxN {
		events = events[len(events)-maxN:]
	}
	var lines []string
	for _, e := range events {
		b, _ := json.Marshal(e)
		lines = append(lines, string(b))
	}
	if len(lines) == 0 {
		return ""
	}
	return "Recent events:\n" + strings.Join(lines, "\n")
}

func RecentEventsForPrompt(ws string, maxN int) []map[string]any {
	events := filepath.Join(ws, ".loom", "workflow_events.v1.ndjson")
	data, err := os.ReadFile(events)
	if err != nil || len(data) == 0 {
		return nil
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if maxN > 0 && len(lines) > maxN {
		lines = lines[len(lines)-maxN:]
	}
	out := make([]map[string]any, 0, len(lines))
	for _, ln := range lines {
		var m map[string]any
		if json.Unmarshal([]byte(ln), &m) == nil {
			out = append(out, m)
		}
	}
	return out
}

func activePlan(plan []PlanItem) string {
	for _, p := range plan {
		if p.Status == "active" {
			return p.Desc
		}
	}
	return ""
}

func buildRulesExcerpt(ws string, max int) string {
	cb := profiler.NewFileSystemProjectContextBuilder()
	text, err := cb.BuildRulesBlock(ws, max)
	if err != nil {
		return ""
	}
	return text
}

func buildHotlist(ws string, top int) string {
	loader := &profiler.FileSystemProfileLoader{}
	profile, err := loader.LoadProfile(ws)
	if err != nil || len(profile.ImportantFiles) == 0 {
		return ""
	}
	n := top
	if n <= 0 || n > len(profile.ImportantFiles) {
		n = len(profile.ImportantFiles)
	}
	lines := make([]string, 0, n)
	for i := 0; i < n; i++ {
		lines = append(lines, profile.ImportantFiles[i].Path)
	}
	return "Hotlist:\n- " + strings.Join(lines, "\n- ")
}

func buildMemoriesExcerpt() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	path := filepath.Join(home, ".loom", "memories.json")
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return ""
	}
	var list []struct{ ID, Text string }
	if json.Unmarshal(data, &list) == nil {
		return stringifyMems(list)
	}
	var wrap struct {
		Memories []struct{ ID, Text string } `json:"memories"`
	}
	if json.Unmarshal(data, &wrap) == nil && wrap.Memories != nil {
		return stringifyMems(wrap.Memories)
	}
	return ""
}

func stringifyMems(list []struct{ ID, Text string }) string {
	if len(list) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Memories:\n")
	max := 5
	if len(list) < max {
		max = len(list)
	}
	for i := 0; i < max; i++ {
		if strings.TrimSpace(list[i].ID) != "" {
			fmt.Fprintf(&b, "- %s: %s\n", list[i].ID, list[i].Text)
		} else {
			fmt.Fprintf(&b, "- %s\n", list[i].Text)
		}
	}
	return strings.TrimSpace(b.String())
}
