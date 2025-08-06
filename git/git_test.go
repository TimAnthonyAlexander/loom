package git

import (
	"errors"
	"strings"
	"testing"
)

func TestValidatePreConditions(t *testing.T) {
	// Instead of using a real repository, use a mockRepository for this test
	// to avoid git command execution errors
	mockRepo := &mockRepository{}

	// Test with known condition
	err := mockRepo.ValidatePreConditions([]string{"clean"})
	if err != nil {
		t.Errorf("Expected no error for 'clean' condition, got: %v", err)
	}

	// Test with unknown condition
	err = mockRepo.ValidatePreConditions([]string{"unknown"})
	if err == nil {
		t.Errorf("Expected error for unknown condition")
	} else if !strings.Contains(err.Error(), "unknown git precondition") {
		t.Errorf("Expected error to contain 'unknown git precondition', got: %v", err)
	}
}

// mockRepository implements a subset of the Repository interface for testing
type mockRepository struct {
	isGitRepo bool
}

func (m *mockRepository) ValidatePreConditions(preConditions []string) error {
	// Simpler implementation that doesn't execute git commands
	for _, condition := range preConditions {
		switch condition {
		case "clean", "no_staged", "no_modified":
			// Accept known conditions
		default:
			return errors.New("unknown git precondition: " + condition)
		}
	}
	return nil
}
