package workflow

type Phase string
type Risk string

const (
	PhaseIdle     Phase = "idle"
	PhaseAnalyze  Phase = "analyze"
	PhasePlan     Phase = "plan"
	PhaseToolCall Phase = "tool_call"
	PhaseValidate Phase = "validate"
	PhaseApply    Phase = "apply"
	PhaseReview   Phase = "review"
	PhaseCommit   Phase = "commit"
)

const (
	RiskLow    Risk = "low"
	RiskMedium Risk = "medium"
	RiskHigh   Risk = "high"
)

type PlanItem struct {
	ID     string `json:"id"`
	Desc   string `json:"desc"`
	Status string `json:"status"`
}

type ToolQueueItem struct {
	Name   string         `json:"name"`
	Args   map[string]any `json:"args"`
	Status string         `json:"status"`
}

type WorkflowState struct {
	Version   int    `json:"version"`
	SessionID string `json:"session_id"`
	CreatedAt int64  `json:"created_at_unix"`
	UpdatedAt int64  `json:"updated_at_unix"`

	Phase    Phase    `json:"phase"`
	Goal     string   `json:"goal"`
	Subgoals []string `json:"subgoals"`

	Plan      []PlanItem      `json:"plan"`
	ToolQueue []ToolQueueItem `json:"tool_queue"`

	LastAction struct {
		Tool string         `json:"tool"`
		Args map[string]any `json:"args"`
		At   int64          `json:"at_unix"`
	} `json:"last_action"`

	LastResultSummary string `json:"last_result_summary"`

	Approvals struct {
		Required bool     `json:"required"`
		Pending  []string `json:"pending"`
		Granted  []string `json:"granted"`
	} `json:"approvals"`

	Safety struct {
		RiskLevel     Risk     `json:"risk_level"`
		Flags         []string `json:"flags"`
		MassEditGuard struct {
			FilesPending   int `json:"files_pending"`
			LinesPending   int `json:"lines_pending"`
			ThresholdFiles int `json:"threshold_files"`
			ThresholdLines int `json:"threshold_lines"`
		} `json:"mass_edit_guard"`
	} `json:"safety"`

	Idempotency struct {
		LastEditFingerprint string `json:"last_edit_fingerprint"`
		LastAppliedCommit   string `json:"last_applied_commit"`
		WorkspaceHash       string `json:"workspace_hash"`
	} `json:"idempotency"`

	Checkpoints []struct {
		ID     string `json:"id"`
		Commit string `json:"commit"`
		Desc   string `json:"desc"`
		At     int64  `json:"at_unix"`
	} `json:"checkpoints"`

	Context struct {
		ActiveFiles     []string `json:"active_files"`
		Hotlist         []string `json:"hotlist"`
		RulesExcerpt    string   `json:"rules_excerpt"`
		MemoriesExcerpt string   `json:"memories_excerpt"`
		ProfileVersion  string   `json:"profile_version"`
	} `json:"context"`

	Budget struct {
		MaxTokensStateSlice  int `json:"max_tokens_state_slice"`
		MaxEventsInPrompt    int `json:"max_events_in_prompt"`
		TruncateRulesMDChars int `json:"truncate_rules_md_chars"`
	} `json:"budget"`

	Summaries struct {
		ConversationRollup string `json:"conversation_rollup"`
		CodeRollup         string `json:"code_rollup"`
	} `json:"summaries"`
}
