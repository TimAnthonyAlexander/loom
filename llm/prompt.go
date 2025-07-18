package llm

import (
	"fmt"
	"loom/indexer"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ProjectConventions represents detected project coding standards and patterns
type ProjectConventions struct {
	Language             string   `json:"language"`
	TestingFramework     string   `json:"testing_framework"`
	TestFilePatterns     []string `json:"test_file_patterns"`
	PackageStructure     string   `json:"package_structure"`
	DocumentationStyle   string   `json:"documentation_style"`
	ErrorHandlingPattern string   `json:"error_handling_pattern"`
	ConfigurationMethod  string   `json:"configuration_method"`
	BuildSystem          string   `json:"build_system"`
	CodingStandards      []string `json:"coding_standards"`
	BestPractices        []string `json:"best_practices"`
}

// PromptEnhancer generates enhanced system prompts with project-specific context
type PromptEnhancer struct {
	workspacePath string
	index         *indexer.Index
	conventions   *ProjectConventions
}

// NewPromptEnhancer creates a new prompt enhancer
func NewPromptEnhancer(workspacePath string, index *indexer.Index) *PromptEnhancer {
	enhancer := &PromptEnhancer{
		workspacePath: workspacePath,
		index:         index,
	}

	enhancer.conventions = enhancer.analyzeProjectConventions()
	return enhancer
}

// CreateEnhancedSystemPrompt generates a comprehensive system prompt with project conventions
func (pe *PromptEnhancer) CreateEnhancedSystemPrompt(enableShell bool) Message {
	stats := pe.index.GetStats()

	// Get language breakdown
	var langBreakdown []string
	type langPair struct {
		name    string
		percent float64
	}

	var langs []langPair
	for name, percent := range stats.LanguagePercent {
		if percent > 0 {
			langs = append(langs, langPair{name, percent})
		}
	}

	sort.Slice(langs, func(i, j int) bool {
		return langs[i].percent > langs[j].percent
	})

	for i, lang := range langs {
		if i >= 5 { // Show top 5 languages
			break
		}
		langBreakdown = append(langBreakdown, fmt.Sprintf("%s (%.1f%%)", lang.name, lang.percent))
	}

	shellStatus := "disabled"
	if enableShell {
		shellStatus = "enabled"
	}

	// Extract project-specific guidelines
	guidelines := pe.extractProjectGuidelines()
	testingGuidance := pe.generateTestingGuidance()
	qualityStandards := pe.generateQualityStandards()

	prompt := fmt.Sprintf(`You are Loom, an AI coding assistant with advanced task execution capabilities and deep understanding of this project's conventions.

## Current Workspace Analysis
- **Total files**: %d
- **Total size**: %.2f MB  
- **Last updated**: %s
- **Primary languages**: %s
- **Shell execution**: %s
- **Project type**: %s
- **Testing framework**: %s

## Project Conventions & Standards
%s

## Code Quality Guidelines
%s

## Testing Best Practices  
%s

## Available Task Types

Execute tasks using JSON code blocks:

`+"```"+`json
{
  "tasks": [
    {"type": "ReadFile", "path": "main.go", "max_lines": 150},
    {"type": "EditFile", "path": "main.go", "diff": "unified diff format"},
    {"type": "ListDir", "path": "src/", "recursive": false},
    {"type": "RunShell", "command": "go test ./...", "timeout": 30}
  ]
}
`+"```"+`

### Task Types:
1. **ReadFile**: Read file contents with optional line limits
   - path: File path (required)
   - max_lines: Max lines to read (default: 200)
   - start_line, end_line: Read specific line range

2. **EditFile**: Apply file changes (requires user confirmation)
   - path: File path (required) 
   - diff: Unified diff format, OR
   - content: Complete file replacement

3. **ListDir**: List directory contents
   - path: Directory path (default: ".")
   - recursive: Include subdirectories (default: false)

4. **RunShell**: Execute shell commands (requires user confirmation, %s)
   - command: Shell command (required)
   - timeout: Timeout in seconds (default: 30)

## Enhanced Interaction Guidelines

### Code Changes Must Include:
1. **Natural Language Summary**: Always explain what and why for every change
2. **Rationale**: Provide reasoning for architectural decisions
3. **Impact Assessment**: Describe what files/systems are affected
4. **Testing Recommendations**: Suggest appropriate tests when making changes

### Response Format:
When making code changes, structure responses as:
1. **Brief explanation** of what you're going to do
2. **Technical reasoning** for the approach
3. **Task execution** with clear descriptions
4. **Change summary** with rationale
5. **Testing suggestions** if applicable

### Quality Standards:
- Write idiomatic, well-documented code following project conventions
- Prefer minimal, focused changes over large refactors
- Always consider backward compatibility and existing patterns
- Include appropriate error handling following project patterns
- Follow the established testing framework and patterns

## Security & Constraints:
- All file paths must be within the workspace
- Binary files cannot be read
- Secrets are automatically redacted from file content
- EditFile and RunShell tasks require user confirmation
- File size limits apply (large files are truncated)
- Always validate inputs and handle edge cases

## Project-Specific Guidelines:
%s

Your role is to be a practical, insightful developer co-pilot that understands this codebase deeply and makes meaningful, well-reasoned improvements that align with the project's existing patterns and standards.`,
		stats.TotalFiles,
		float64(stats.TotalSize)/1024/1024,
		pe.index.LastUpdated.Format("15:04:05"),
		strings.Join(langBreakdown, ", "),
		shellStatus,
		pe.conventions.Language,
		pe.conventions.TestingFramework,
		pe.formatConventions(),
		qualityStandards,
		testingGuidance,
		shellStatus,
		guidelines)

	return Message{
		Role:      "system",
		Content:   prompt,
		Timestamp: time.Now(),
	}
}

// analyzeProjectConventions analyzes the project to detect conventions and patterns
func (pe *PromptEnhancer) analyzeProjectConventions() *ProjectConventions {
	conventions := &ProjectConventions{
		TestFilePatterns: []string{},
		CodingStandards:  []string{},
		BestPractices:    []string{},
	}

	// Analyze primary language and testing patterns
	stats := pe.index.GetStats()
	if len(stats.LanguageBreakdown) > 0 {
		// Find primary language
		maxCount := 0
		for lang, count := range stats.LanguageBreakdown {
			if count > maxCount {
				maxCount = count
				conventions.Language = lang
			}
		}
	}

	// Detect Go-specific patterns
	if conventions.Language == "Go" {
		conventions.TestingFramework = "Go standard testing"
		conventions.TestFilePatterns = []string{"*_test.go"}
		conventions.PackageStructure = "Go modules with clean package separation"
		conventions.ErrorHandlingPattern = "Explicit error returns with error wrapping"
		conventions.ConfigurationMethod = "JSON config with struct tags"
		conventions.BuildSystem = "Go modules (go.mod)"

		conventions.CodingStandards = []string{
			"Follow Go naming conventions (CamelCase for public, camelCase for private)",
			"Use interfaces for abstraction (e.g., LLMAdapter pattern)",
			"Prefer composition over inheritance",
			"Use struct embedding for extending functionality",
			"Return errors explicitly, don't panic",
			"Use context.Context for cancellation and timeouts",
		}

		conventions.BestPractices = []string{
			"Keep packages focused with single responsibility",
			"Use meaningful variable and function names",
			"Write tests for public API functions",
			"Handle errors at appropriate levels",
			"Use defer for cleanup operations",
			"Prefer small, composable functions",
		}
	}

	// Detect test files and patterns
	for filePath := range pe.index.Files {
		if strings.Contains(filePath, "_test.") {
			conventions.TestFilePatterns = append(conventions.TestFilePatterns, "*_test.go")
			break
		}
	}

	// Detect documentation style
	if pe.fileExists("README.md") {
		conventions.DocumentationStyle = "Markdown with comprehensive examples"
	}

	return conventions
}

// extractProjectGuidelines extracts guidelines from README, CONTRIBUTING, etc.
func (pe *PromptEnhancer) extractProjectGuidelines() string {
	var guidelines strings.Builder

	// Check for README.md insights
	readmePath := filepath.Join(pe.workspacePath, "README.md")
	if content := pe.readFileContent(readmePath); content != "" {
		if strings.Contains(strings.ToLower(content), "milestone") {
			guidelines.WriteString("- This is a milestone-based project with structured development phases\n")
		}
		if strings.Contains(strings.ToLower(content), "tui") || strings.Contains(strings.ToLower(content), "terminal") {
			guidelines.WriteString("- Focus on terminal/TUI user experience with Bubble Tea framework\n")
		}
		if strings.Contains(strings.ToLower(content), "llm") || strings.Contains(strings.ToLower(content), "ai") {
			guidelines.WriteString("- AI/LLM integration is a core feature - maintain adapter pattern\n")
		}
	}

	// Analyze go.mod for dependencies
	goModPath := filepath.Join(pe.workspacePath, "go.mod")
	if content := pe.readFileContent(goModPath); content != "" {
		if strings.Contains(content, "bubbletea") {
			guidelines.WriteString("- Use Bubble Tea patterns for TUI components and message handling\n")
		}
		if strings.Contains(content, "cobra") {
			guidelines.WriteString("- Follow Cobra CLI patterns for command structure\n")
		}
		if strings.Contains(content, "fsnotify") {
			guidelines.WriteString("- File watching and indexing is important - maintain performance\n")
		}
	}

	if guidelines.Len() == 0 {
		guidelines.WriteString("- Follow established project patterns and maintain consistency\n")
	}

	return guidelines.String()
}

// generateTestingGuidance creates testing-specific guidance
func (pe *PromptEnhancer) generateTestingGuidance() string {
	var guidance strings.Builder

	guidance.WriteString("### Testing Framework: " + pe.conventions.TestingFramework + "\n")
	guidance.WriteString("### Test File Patterns: " + strings.Join(pe.conventions.TestFilePatterns, ", ") + "\n")
	guidance.WriteString("### Testing Guidelines:\n")

	if pe.conventions.Language == "Go" {
		guidance.WriteString("- Write table-driven tests for complex scenarios\n")
		guidance.WriteString("- Use t.Fatalf for setup failures, t.Errorf for assertion failures\n")
		guidance.WriteString("- Test both success and error cases\n")
		guidance.WriteString("- Use meaningful test names that describe the scenario\n")
		guidance.WriteString("- Consider using testify/assert for complex assertions\n")
		guidance.WriteString("- Test public interfaces, not implementation details\n")
	}

	// Check if there are existing test files to understand patterns
	hasTests := false
	for filePath := range pe.index.Files {
		if strings.Contains(filePath, "_test.") {
			hasTests = true
			break
		}
	}

	if hasTests {
		guidance.WriteString("- Follow existing test patterns in the codebase\n")
		guidance.WriteString("- Maintain test coverage for new functionality\n")
	} else {
		guidance.WriteString("- Consider adding tests for new functionality\n")
		guidance.WriteString("- Start with critical path testing\n")
	}

	return guidance.String()
}

// generateQualityStandards creates code quality guidance
func (pe *PromptEnhancer) generateQualityStandards() string {
	var standards strings.Builder

	standards.WriteString("### Code Quality Standards:\n")
	for _, standard := range pe.conventions.CodingStandards {
		standards.WriteString("- " + standard + "\n")
	}

	standards.WriteString("\n### Best Practices:\n")
	for _, practice := range pe.conventions.BestPractices {
		standards.WriteString("- " + practice + "\n")
	}

	standards.WriteString("\n### Project-Specific Patterns:\n")
	standards.WriteString("- Use the established adapter pattern for external integrations\n")
	standards.WriteString("- Maintain separation between TUI, business logic, and external services\n")
	standards.WriteString("- Follow the task execution pattern for file operations\n")
	standards.WriteString("- Ensure proper error handling and user confirmation for destructive operations\n")

	return standards.String()
}

// formatConventions formats the detected conventions for display
func (pe *PromptEnhancer) formatConventions() string {
	var formatted strings.Builder

	formatted.WriteString("### Detected Project Patterns:\n")
	formatted.WriteString(fmt.Sprintf("- **Language**: %s\n", pe.conventions.Language))
	formatted.WriteString(fmt.Sprintf("- **Testing**: %s\n", pe.conventions.TestingFramework))
	formatted.WriteString(fmt.Sprintf("- **Package Structure**: %s\n", pe.conventions.PackageStructure))
	formatted.WriteString(fmt.Sprintf("- **Error Handling**: %s\n", pe.conventions.ErrorHandlingPattern))
	formatted.WriteString(fmt.Sprintf("- **Configuration**: %s\n", pe.conventions.ConfigurationMethod))
	formatted.WriteString(fmt.Sprintf("- **Build System**: %s\n", pe.conventions.BuildSystem))

	return formatted.String()
}

// Helper methods
func (pe *PromptEnhancer) fileExists(relativePath string) bool {
	_, exists := pe.index.Files[relativePath]
	return exists
}

func (pe *PromptEnhancer) readFileContent(filePath string) string {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	return string(content)
}

// GetConventions returns the detected project conventions
func (pe *PromptEnhancer) GetConventions() *ProjectConventions {
	return pe.conventions
}
