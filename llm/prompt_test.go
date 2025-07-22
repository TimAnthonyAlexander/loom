package llm

import (
	"encoding/json"
	"loom/indexer"
	"loom/paths"
	"os"
	"path/filepath"
	"testing"
)

func TestProjectRules(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom-rules-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create .loom directory
	loomDir := filepath.Join(tempDir, ".loom")
	if err := os.MkdirAll(loomDir, 0755); err != nil {
		t.Fatalf("Failed to create .loom dir: %v", err)
	}

	// Create a simple index for testing
	idx := indexer.NewIndex(tempDir, 500*1024)
	enhancer := NewPromptEnhancer(tempDir, idx)

	// Test adding a rule
	err = enhancer.AddProjectRule("Always use interfaces for external dependencies", "This promotes testability")
	if err != nil {
		t.Fatalf("Failed to add project rule: %v", err)
	}

	// Test loading rules
	rules, err := enhancer.LoadProjectRules()
	if err != nil {
		t.Fatalf("Failed to load project rules: %v", err)
	}

	if len(rules.Rules) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(rules.Rules))
	}

	rule := rules.Rules[0]
	if rule.Text != "Always use interfaces for external dependencies" {
		t.Errorf("Expected rule text 'Always use interfaces for external dependencies', got '%s'", rule.Text)
	}

	if rule.Description != "This promotes testability" {
		t.Errorf("Expected description 'This promotes testability', got '%s'", rule.Description)
	}

	if rule.ID == "" {
		t.Error("Expected non-empty rule ID")
	}

	if rule.CreatedAt.IsZero() {
		t.Error("Expected non-zero CreatedAt time")
	}

	// Test adding another rule
	err = enhancer.AddProjectRule("Use dependency injection pattern", "")
	if err != nil {
		t.Fatalf("Failed to add second project rule: %v", err)
	}

	// Test listing rules
	rules, err = enhancer.ListProjectRules()
	if err != nil {
		t.Fatalf("Failed to list project rules: %v", err)
	}

	if len(rules.Rules) != 2 {
		t.Fatalf("Expected 2 rules, got %d", len(rules.Rules))
	}

	// Test removing a rule
	ruleIDToRemove := rules.Rules[0].ID
	err = enhancer.RemoveProjectRule(ruleIDToRemove)
	if err != nil {
		t.Fatalf("Failed to remove project rule: %v", err)
	}

	// Verify rule was removed
	rules, err = enhancer.ListProjectRules()
	if err != nil {
		t.Fatalf("Failed to list project rules after removal: %v", err)
	}

	if len(rules.Rules) != 1 {
		t.Fatalf("Expected 1 rule after removal, got %d", len(rules.Rules))
	}

	// Test removing non-existent rule
	err = enhancer.RemoveProjectRule("non-existent-id")
	if err == nil {
		t.Error("Expected error when removing non-existent rule")
	}
}

func TestProjectRulesIntegration(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom-rules-integration-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create .loom directory
	loomDir := filepath.Join(tempDir, ".loom")
	if err := os.MkdirAll(loomDir, 0755); err != nil {
		t.Fatalf("Failed to create .loom dir: %v", err)
	}

	// Create a simple index for testing
	idx := indexer.NewIndex(tempDir, 500*1024)
	enhancer := NewPromptEnhancer(tempDir, idx)

	// Add some project rules
	rules := []string{
		"Use structured logging with context",
		"Implement circuit breaker pattern for external calls",
		"Always validate input at API boundaries",
	}

	for _, rule := range rules {
		err = enhancer.AddProjectRule(rule, "")
		if err != nil {
			t.Fatalf("Failed to add project rule '%s': %v", rule, err)
		}
	}

	// Test that rules are included in project guidelines
	guidelines := enhancer.extractProjectGuidelines()

	// Check that each rule appears in the guidelines
	for _, rule := range rules {
		if !containsString(guidelines, rule) {
			t.Errorf("Expected guidelines to contain rule '%s', but it was not found", rule)
		}
	}

	// Check that the section header is present
	if !containsString(guidelines, "### Project-Specific Rules:") {
		t.Error("Expected guidelines to contain '### Project-Specific Rules:' section")
	}
}

func TestProjectRulesEmptyState(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom-rules-empty-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create .loom directory
	loomDir := filepath.Join(tempDir, ".loom")
	if err := os.MkdirAll(loomDir, 0755); err != nil {
		t.Fatalf("Failed to create .loom dir: %v", err)
	}

	// Create a simple index for testing
	idx := indexer.NewIndex(tempDir, 500*1024)
	enhancer := NewPromptEnhancer(tempDir, idx)

	// Test loading rules when no rules file exists
	rules, err := enhancer.LoadProjectRules()
	if err != nil {
		t.Fatalf("Failed to load project rules from empty state: %v", err)
	}

	if len(rules.Rules) != 0 {
		t.Fatalf("Expected 0 rules in empty state, got %d", len(rules.Rules))
	}

	// Test that guidelines don't include rules section when no rules exist
	guidelines := enhancer.extractProjectGuidelines()

	if containsString(guidelines, "### Project-Specific Rules:") {
		t.Error("Expected guidelines to NOT contain '### Project-Specific Rules:' section when no rules exist")
	}
}

func TestProjectRulesFileFormat(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom-rules-format-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a simple index for testing
	idx := indexer.NewIndex(tempDir, 500*1024)
	enhancer := NewPromptEnhancer(tempDir, idx)

	// Add a rule
	testRule := "Test rule with special characters: @#$%^&*()"
	testDescription := "Test description with unicode: 你好 🌟"

	err = enhancer.AddProjectRule(testRule, testDescription)
	if err != nil {
		t.Fatalf("Failed to add project rule: %v", err)
	}

	// Read the raw file and verify JSON format (using correct path via paths package)
	projectPaths, err := paths.NewProjectPaths(tempDir)
	if err != nil {
		t.Fatalf("Failed to create project paths: %v", err)
	}
	rulesPath := projectPaths.RulesPath()
	data, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatalf("Failed to read rules file: %v", err)
	}

	// Verify it's valid JSON
	var fileRules ProjectRules
	if err := json.Unmarshal(data, &fileRules); err != nil {
		t.Fatalf("Rules file is not valid JSON: %v", err)
	}

	if len(fileRules.Rules) != 1 {
		t.Fatalf("Expected 1 rule in file, got %d", len(fileRules.Rules))
	}

	rule := fileRules.Rules[0]
	if rule.Text != testRule {
		t.Errorf("Expected rule text '%s', got '%s'", testRule, rule.Text)
	}

	if rule.Description != testDescription {
		t.Errorf("Expected description '%s', got '%s'", testDescription, rule.Description)
	}
}

// Helper function to check if a string contains a substring
func containsString(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) &&
		indexOfString(haystack, needle) >= 0
}

// Helper function to find index of substring
func indexOfString(haystack, needle string) int {
	if len(needle) == 0 {
		return 0
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
