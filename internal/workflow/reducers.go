package workflow

import "time"

var validTransitions = map[Phase]map[Phase]bool{
	PhaseIdle:     {PhaseAnalyze: true},
	PhaseAnalyze:  {PhasePlan: true},
	PhasePlan:     {PhaseToolCall: true},
	PhaseToolCall: {PhaseValidate: true},
	PhaseValidate: {PhaseApply: true, PhasePlan: true},
	PhaseApply:    {PhaseReview: true},
	PhaseReview:   {PhaseCommit: true, PhasePlan: true},
	PhaseCommit:   {PhaseIdle: true},
}

func Reduce(evt map[string]any, st *WorkflowState) *WorkflowState {
	if st == nil {
		now := time.Now().Unix()
		st = &WorkflowState{Version: 1, CreatedAt: now, UpdatedAt: now, Phase: PhaseIdle}
	}
	typ, _ := evt["type"].(string)

	switch typ {
	case "phase":
		from := st.Phase
		if toStr, ok := evt["to"].(string); ok {
			to := Phase(toStr)
			if validTransitions[from][to] {
				st.Phase = to
			}
		}
	case "tool_use":
		st.LastAction.Tool, _ = evt["tool"].(string)
		if args, ok := evt["args"].(map[string]any); ok {
			st.LastAction.Args = args
		}
		if ts, ok := asInt64(evt["ts"]); ok {
			st.LastAction.At = ts
		}
	case "tool_result":
		if sum, ok := evt["summary"].(string); ok {
			st.LastResultSummary = sum
		}
	case "approval":
		switch evt["status"] {
		case "granted":
			st.Approvals.Granted = append(st.Approvals.Granted, asString(evt["tool"]))
		case "pending":
			st.Approvals.Pending = append(st.Approvals.Pending, asString(evt["tool"]))
		}
	}

	if st.Budget.MaxTokensStateSlice == 0 {
		st.Budget.MaxTokensStateSlice = 1200
	}
	if st.Budget.MaxEventsInPrompt == 0 {
		st.Budget.MaxEventsInPrompt = 8
	}
	if st.Budget.TruncateRulesMDChars == 0 {
		st.Budget.TruncateRulesMDChars = 600
	}
	return st
}

func asInt64(v any) (int64, bool) {
	switch t := v.(type) {
	case float64:
		return int64(t), true
	case int64:
		return t, true
	case int:
		return int64(t), true
	default:
		return 0, false
	}
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
