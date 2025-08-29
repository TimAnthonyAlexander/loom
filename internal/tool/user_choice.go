package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// UserChoiceArgs describes a user choice request.
type UserChoiceArgs struct {
	Question string   `json:"question"`
	Options  []string `json:"options"`
}

// UserChoiceResponse represents the response from a user choice.
type UserChoiceResponse struct {
	SelectedOption string `json:"selected_option"`
	SelectedIndex  int    `json:"selected_index"`
}

// RegisterUserChoice registers the user_choice tool which prompts user for a choice.
func RegisterUserChoice(registry *Registry) error {
	return registry.Register(Definition{
		Name:        "user_choice",
		Description: "Present the user with 2-4 options and wait for their selection",
		Safe:        false, // Requires user interaction
		JSONSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"question": map[string]interface{}{
					"type":        "string",
					"description": "The question or prompt to present to the user",
				},
				"options": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "string",
					},
					"description": "Array of 2-4 string options for the user to choose from",
					"minItems":    2,
					"maxItems":    4,
				},
			},
			"required": []string{"question", "options"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			var args UserChoiceArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, fmt.Errorf("failed to parse arguments: %w", err)
			}
			return handleUserChoice(ctx, args)
		},
	})
}

func handleUserChoice(ctx context.Context, args UserChoiceArgs) (*ExecutionResult, error) {
	// Validate arguments
	if strings.TrimSpace(args.Question) == "" {
		return nil, errors.New("question is required")
	}

	if len(args.Options) < 2 || len(args.Options) > 4 {
		return nil, errors.New("options must contain 2-4 items")
	}

	// Validate that all options are non-empty
	for i, option := range args.Options {
		if strings.TrimSpace(option) == "" {
			return nil, fmt.Errorf("option %d is empty", i+1)
		}
	}

	// Create a structured diff showing the choice being presented
	var diffBuilder strings.Builder
	diffBuilder.WriteString("User Choice Request:\n")
	diffBuilder.WriteString(fmt.Sprintf("Question: %s\n\n", args.Question))
	diffBuilder.WriteString("Options:\n")
	for i, option := range args.Options {
		diffBuilder.WriteString(fmt.Sprintf("%d. %s\n", i+1, option))
	}

	// Return a result that will trigger the approval flow
	// The actual user selection will be handled by the engine's approval system
	return &ExecutionResult{
		Content: fmt.Sprintf("Waiting for user to choose from %d options: %s", len(args.Options), args.Question),
		Diff:    diffBuilder.String(),
		Safe:    false, // This will trigger the approval/choice flow
	}, nil
}
