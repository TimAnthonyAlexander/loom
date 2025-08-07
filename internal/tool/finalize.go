package tool

import (
	"context"
	"encoding/json"
	"fmt"
)

// FinalizeArgs represents the arguments for the finalize tool.
type FinalizeArgs struct {
	Summary string `json:"summary"`
}

// RegisterFinalize registers the finalize tool.
// The engine will treat this tool specially: when called, the loop ends and
// the summary is emitted to the user as the final assistant message.
func RegisterFinalize(registry *Registry) error {
	return registry.Register(Definition{
		Name:        "finalize",
		Description: "Signal that the objective is complete and provide a concise final summary for the user.",
		Safe:        true,
		JSONSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"summary": map[string]interface{}{
					"type":        "string",
					"description": "Concise final answer to present to the user",
				},
			},
			"required": []string{"summary"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			var args FinalizeArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, fmt.Errorf("failed to parse arguments: %w", err)
			}
			return &ExecutionResult{Content: args.Summary, Safe: true}, nil
		},
	})
}
