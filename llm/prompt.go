package llm

import (
	"encoding/json"
	"fmt"
	"loom/indexer"
	"loom/memory"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ProjectConventions represents detected project coding standards and patterns
type ProjectConventions struct {
	Language             string   `json:"language"`
	ProjectType          string   `json:"project_type"`
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

// ProjectRules represents user-defined project-specific rules
type ProjectRules struct {
	Rules []ProjectRule `json:"rules"`
}

// ProjectRule represents a single user-defined rule
type ProjectRule struct {
	ID          string    `json:"id"`
	Text        string    `json:"text"`
	CreatedAt   time.Time `json:"created_at"`
	Description string    `json:"description,omitempty"`
}

// PromptEnhancer generates enhanced system prompts with project-specific context
type PromptEnhancer struct {
	workspacePath string
	index         *indexer.Index
	conventions   *ProjectConventions
	memoryStore   *memory.MemoryStore
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

// SetMemoryStore sets the memory store for the prompt enhancer
func (pe *PromptEnhancer) SetMemoryStore(memoryStore *memory.MemoryStore) {
	pe.memoryStore = memoryStore
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
	memoriesSection := pe.generateMemoriesSection()

	prompt := fmt.Sprintf(`# Loom Prompt v2025-07-22

You are Loom, an AI coding assistant with advanced autonomous task execution capabilities and deep understanding of this project's conventions.

## 1. Workspace Snapshot
- **Total files**: %[1]d (%[2].2f MB)
- **Last updated**: %[3]s
- **Primary languages**: %[4]s
- **Shell execution**: %[5]s
- **Project type**: %[6]s
- **Testing framework**: %[7]s

## 2. Project-Specific Guidelines
%[12]s

%[8]s

%[9]s

%[10]s

%[11]s

## 3. Task Reference

| Task | Syntax | Purpose |
|------|--------|---------|
| READ | READ file.go (lines 40-80) | Inspect code with line numbers |
| SEARCH | SEARCH "pattern" type:go context:3 | Locate symbols/patterns |
| LIST | LIST src/ recursive | View directory structure |
| EDIT | EDIT file.go:15-17 -> description | Modify files (see §5.3) |
| RUN | RUN go test | Execute shell commands |
| MEMORY | MEMORY create key content:"text" | Persist information (see §7.B) |

**Basic syntax**: ACTION target [options] -> description

## 4. Workflow

### 4.1 Exploration Flow
**Triggers**: Any user request matching ("tell me about"|"explain"|"analyze"|"where is"|"find"|"search for") triggers comprehensive exploration.

**Process**:
1. Begin with one READ or LIST task (usually README.md)
2. Analyze results before proceeding
3. Plan next step based on findings
4. Continue sequentially until understanding is complete
5. Signal completion with EXPLORATION_COMPLETE: [analysis]

**Search-first strategy**: For "where is X?" queries, start with SEARCH to locate all occurrences, then READ specific files.

### 4.2 Editing Flow
**Mandatory sequence**:
1. READ file with line numbers to get current state
2. Identify exact line numbers for changes
3. Use SafeEdit format (see §5.3)
4. System validates context before applying
5. Edit confidently - validation ensures safety

## 5. Tool Details

### 5.1 SEARCH Rules
**Primary tool** for finding code patterns, functions, types, and symbols.

**Never use**: RUN grep or find commands - always use SEARCH instead.

**Common patterns**:
- Function definitions: SEARCH "func IndexStats" type:go
- Types/structs: SEARCH "type.*IndexStats" type:go
- Imports: SEARCH "import.*IndexStats" type:go
- TODOs: SEARCH "TODO|FIXME" case-insensitive

**Options**:
- type:go,js - file types to include
- -type:md - exclude file types
- context:3 - show surrounding lines
- case-insensitive - ignore case
- whole-word - exact word matches
- in:src/ - search specific directory
- max:50 - limit results

**Result handling**:
1. Read actual task result first
2. If "No matches found" - state pattern doesn't exist
3. If matches found - analyze actual file paths returned
4. Never hallucinate or invent results

### 5.2 LIST / READ
- LIST: Directory contents (LIST . recursive)
- READ: File contents with line numbers (READ file.go (lines 40-80))
- Always request line numbers before editing

### 5.3 EDIT (SafeEdit Specification)
**For existing files**, mandatory format:

EDIT file.go:15-17 -> description

--- BEFORE ---
[1-3 lines before edit for validation]
--- CHANGE ---
EDIT_LINES: 15-17
[new content for these exact lines]
--- AFTER ---
[1-3 lines after edit for validation]

**Rules**:
- Context lines (BEFORE/AFTER) are never modified
- Large ranges (>20 lines) require EDIT_OVERRIDE_CONFIRMED
- Line numbers counted after LF normalization
- System validates context before applying

**For new files**: Simple format with full content in code block.

### 5.4 RUN
Shell command execution.
- RUN go test
- RUN npm install (timeout: 60)
- RUN command --interactive for user input required
- RUN command --interactive auto for automatic responses

### 5.5 MEMORY
Store important information across conversations. Create memories proactively when encountering useful context, patterns, or user preferences.

Basic operations: create, update, get, delete, list
(Full API reference in §7.B)

## 6. Prohibited Actions
- ❌ Edit without fenced context validation for existing files
- ❌ Edit without reading file first to get line numbers
- ❌ Edit large ranges (>20 lines) without EDIT_OVERRIDE_CONFIRMED
- ❌ Use RUN+grep when SEARCH is available
- ❌ Use find+grep combinations (use SEARCH with filters)
- ❌ Provide partial file content without line ranges
- ❌ Hallucinate search results when "No matches found"

## 7. Appendices

### A. SafeEdit Examples

**Single line edit**:
EDIT main.go:42 -> fix variable name

--- BEFORE ---
func main() {
    userName := "john"
--- CHANGE ---
EDIT_LINES: 42
    username := "john"
--- AFTER ---
    fmt.Println(username)
}

**Multi-line edit**:
EDIT handler.go:28-31 -> improve error handling

--- BEFORE ---
func ProcessRequest(req *Request) error {
    if req == nil {
--- CHANGE ---
EDIT_LINES: 28-31
        return &ValidationError{
            Field:   "request",
            Message: "request cannot be nil",
        }
--- AFTER ---
    }
    return processData(req.Data)

### B. Memory API Reference

**Operations**:
- MEMORY create key content:"text" [description:"desc"] [tags:tag1,tag2] [active:true]
- MEMORY update key content:"new text"
- MEMORY get key
- MEMORY delete key
- MEMORY list [active:true]

**Options**:
- description: Human-readable description
- tags: Comma-separated tags for organization
- active: Whether memory is included in prompts (default: true)

## Security & Constraints
- All file paths must be within workspace
- Binary files cannot be read
- Secrets automatically redacted
- Context validation mandatory for existing file edits`,
		stats.TotalFiles,
		float64(stats.TotalSize)/1024/1024,
		pe.index.LastUpdated.Format("15:04:05"),
		strings.Join(langBreakdown, ", "),
		shellStatus,
		pe.conventions.ProjectType,
		pe.conventions.TestingFramework,
		pe.formatConventions(),
		qualityStandards,
		testingGuidance,
		memoriesSection,
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

	// Detect language-specific patterns
	switch conventions.Language {
	case "Go":
		conventions.TestingFramework = "Go standard testing"
		conventions.TestFilePatterns = []string{"*_test.go"}
		conventions.PackageStructure = "Go modules with clean package separation"
		conventions.ErrorHandlingPattern = "Explicit error returns with error wrapping"
		conventions.ConfigurationMethod = "JSON config with struct tags"
		conventions.BuildSystem = "Go modules (go.mod)"
		conventions.ProjectType = "Go application"

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
			"**Read multiple related files** to understand complete context",
			"**Follow import chains** to understand dependencies",
			"**Analyze interfaces comprehensively** before implementing",
		}
	case "JavaScript":
		conventions.ProjectType = "JavaScript application"
		conventions.TestingFramework = "Jest/Mocha"
		conventions.BuildSystem = "npm/yarn"
	case "TypeScript":
		conventions.ProjectType = "TypeScript application"
		conventions.TestingFramework = "Jest/Vitest"
		conventions.BuildSystem = "npm/yarn with TypeScript"
	case "Python":
		conventions.ProjectType = "Python application"
		conventions.TestingFramework = "pytest/unittest"
		conventions.BuildSystem = "pip/poetry"
	case "Rust":
		conventions.ProjectType = "Rust application"
		conventions.TestingFramework = "Rust built-in testing"
		conventions.BuildSystem = "Cargo"
	case "Java":
		conventions.ProjectType = "Java application"
		conventions.TestingFramework = "JUnit"
		conventions.BuildSystem = "Maven/Gradle"
	case "C++":
		conventions.ProjectType = "C++ application"
		conventions.TestingFramework = "Google Test/Catch2"
		conventions.BuildSystem = "CMake/Make"
	default:
		conventions.ProjectType = fmt.Sprintf("%s application", conventions.Language)
		conventions.TestingFramework = "Framework detection needed"
		conventions.BuildSystem = "Build system detection needed"
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

	guidelines.WriteString("### Autonomous Project Exploration:\n")
	guidelines.WriteString("- **Read comprehensive project documentation** (README, CONTRIBUTING, docs/)\n")
	guidelines.WriteString("- **Analyze dependency patterns** to understand architectural choices\n")
	guidelines.WriteString("- **Explore package structure** to understand code organization\n")
	guidelines.WriteString("- **Identify key interfaces and abstractions** through systematic reading\n")

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
		if strings.Contains(strings.ToLower(content), "task") || strings.Contains(strings.ToLower(content), "execution") {
			guidelines.WriteString("- Task execution is central - understand the complete task lifecycle\n")
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
		if strings.Contains(content, "testify") {
			guidelines.WriteString("- Use testify assertions for comprehensive test validation\n")
		}
	}

	guidelines.WriteString("\n### Comprehensive Analysis Requirements:\n")
	guidelines.WriteString("- **Explore all major packages** when asked about architecture\n")
	guidelines.WriteString("- **Read configuration files** to understand system setup\n")
	guidelines.WriteString("- **Analyze main entry points** to understand application flow\n")
	guidelines.WriteString("- **Follow import dependencies** to build complete understanding\n")

	// Add user-defined project rules
	if rules, err := pe.LoadProjectRules(); err == nil && len(rules.Rules) > 0 {
		guidelines.WriteString("\n### Project-Specific Rules:\n")
		for _, rule := range rules.Rules {
			guidelines.WriteString(fmt.Sprintf("- %s\n", rule.Text))
		}
	}

	if guidelines.Len() == 0 {
		guidelines.WriteString("- Follow established project patterns and maintain consistency through comprehensive analysis\n")
	}

	return guidelines.String()
}

// generateTestingGuidance creates testing-specific guidance
func (pe *PromptEnhancer) generateTestingGuidance() string {
	var guidance strings.Builder

	guidance.WriteString("### Testing Framework: " + pe.conventions.TestingFramework + "\n")
	guidance.WriteString("### Test File Patterns: " + strings.Join(pe.conventions.TestFilePatterns, ", ") + "\n")

	guidance.WriteString("### Autonomous Testing Analysis:\n")
	guidance.WriteString("- **Read all test files** when analyzing testing approaches\n")
	guidance.WriteString("- **Understand test patterns** by examining multiple test examples\n")
	guidance.WriteString("- **Analyze test coverage gaps** by comparing tests to implementation\n")
	guidance.WriteString("- **Explore testing utilities** and helper functions comprehensively\n")

	guidance.WriteString("### Testing Guidelines:\n")

	if pe.conventions.Language == "Go" {
		guidance.WriteString("- Write table-driven tests for complex scenarios\n")
		guidance.WriteString("- Use t.Fatalf for setup failures, t.Errorf for assertion failures\n")
		guidance.WriteString("- Test both success and error cases comprehensively\n")
		guidance.WriteString("- Use meaningful test names that describe the scenario\n")
		guidance.WriteString("- Consider using testify/assert for complex assertions\n")
		guidance.WriteString("- Test public interfaces, not implementation details\n")
		guidance.WriteString("- **Analyze existing test patterns** before writing new tests\n")
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
		guidance.WriteString("- **Follow existing test patterns** discovered through comprehensive analysis\n")
		guidance.WriteString("- **Read test files systematically** to understand testing approaches\n")
		guidance.WriteString("- Maintain test coverage for new functionality\n")
	} else {
		guidance.WriteString("- Consider adding comprehensive tests for new functionality\n")
		guidance.WriteString("- Start with critical path testing and expand systematically\n")
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

	standards.WriteString("\n### Autonomous Analysis Principles:\n")
	standards.WriteString("- **Be comprehensive by default** - read multiple files to understand full context\n")
	standards.WriteString("- **Use sequential exploration** - build understanding step by step\n")
	standards.WriteString("- **Explore systematically** - follow architectural patterns and dependencies\n")
	standards.WriteString("- **Provide detailed insights** - explain not just what, but why and how\n")

	standards.WriteString("\n### Project-Specific Patterns:\n")
	standards.WriteString("- Use the established adapter pattern for external integrations\n")
	standards.WriteString("- Maintain separation between TUI, business logic, and external services\n")
	standards.WriteString("- Follow the task execution pattern for file operations\n")
	standards.WriteString("- Ensure proper error handling and user confirmation for destructive operations\n")
	standards.WriteString("- **Explore the entire codebase** when asked about architecture or functionality\n")

	return standards.String()
}

// generateMemoriesSection creates the active memories section for the system prompt
func (pe *PromptEnhancer) generateMemoriesSection() string {
	if pe.memoryStore == nil {
		return "" // No memory store available
	}

	memoriesContent := pe.memoryStore.FormatMemoriesForPrompt()
	if memoriesContent == "" {
		return "" // No active memories
	}

	return memoriesContent
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

	formatted.WriteString("\n### Convention Analysis Approach:\n")
	formatted.WriteString("- **Systematically read code examples** to understand patterns\n")
	formatted.WriteString("- **Analyze multiple files** to identify consistent conventions\n")
	formatted.WriteString("- **Follow architectural decisions** through comprehensive exploration\n")
	formatted.WriteString("- **Understand the reasoning** behind established patterns\n")

	return formatted.String()
}

// LoadProjectRules loads user-defined project rules from .loom/rules.json
func (pe *PromptEnhancer) LoadProjectRules() (*ProjectRules, error) {
	rulesPath := filepath.Join(pe.workspacePath, ".loom", "rules.json")

	// Return empty rules if file doesn't exist
	if _, err := os.Stat(rulesPath); os.IsNotExist(err) {
		return &ProjectRules{Rules: []ProjectRule{}}, nil
	}

	data, err := os.ReadFile(rulesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read rules file: %w", err)
	}

	var rules ProjectRules
	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, fmt.Errorf("failed to parse rules file: %w", err)
	}

	return &rules, nil
}

// SaveProjectRules saves user-defined project rules to .loom/rules.json
func (pe *PromptEnhancer) SaveProjectRules(rules *ProjectRules) error {
	rulesPath := filepath.Join(pe.workspacePath, ".loom", "rules.json")

	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal rules: %w", err)
	}

	if err := os.WriteFile(rulesPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write rules file: %w", err)
	}

	return nil
}

// AddProjectRule adds a new user-defined rule to the project
func (pe *PromptEnhancer) AddProjectRule(text, description string) error {
	rules, err := pe.LoadProjectRules()
	if err != nil {
		return err
	}

	// Generate a simple ID based on current time
	id := fmt.Sprintf("rule_%d", time.Now().Unix())

	newRule := ProjectRule{
		ID:          id,
		Text:        text,
		CreatedAt:   time.Now(),
		Description: description,
	}

	rules.Rules = append(rules.Rules, newRule)
	return pe.SaveProjectRules(rules)
}

// RemoveProjectRule removes a user-defined rule by ID
func (pe *PromptEnhancer) RemoveProjectRule(id string) error {
	rules, err := pe.LoadProjectRules()
	if err != nil {
		return err
	}

	for i, rule := range rules.Rules {
		if rule.ID == id {
			rules.Rules = append(rules.Rules[:i], rules.Rules[i+1:]...)
			return pe.SaveProjectRules(rules)
		}
	}

	return fmt.Errorf("rule with ID %s not found", id)
}

// ListProjectRules returns all user-defined project rules
func (pe *PromptEnhancer) ListProjectRules() (*ProjectRules, error) {
	return pe.LoadProjectRules()
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
