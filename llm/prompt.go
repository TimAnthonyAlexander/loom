package llm

import (
	"encoding/json"
	"fmt"
	"loom/indexer"
	"loom/memory"
	"loom/paths"
	"loom/todo"
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
	todoManager   *todo.TodoManager
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

// SetTodoManager sets the todo manager for the prompt enhancer
func (pe *PromptEnhancer) SetTodoManager(todoManager *todo.TodoManager) {
	pe.todoManager = todoManager
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
		if langs[i].percent != langs[j].percent {
			return langs[i].percent > langs[j].percent
		}
		return langs[i].name < langs[j].name
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
	todoSection := pe.generateTodoSection()

	prompt := fmt.Sprintf(`# Loom Prompt v2025-07-22

1 . Workspace Snapshot
• Total files: %[1]d  (%[2].2f MB)
• Last updated: %[3]s
• Primary languages: %[4]s
• Shell execution: %[5]s
• Project type: %[6]s — Tests: %[7]s

2 . 🔴  ONE ACTION PER TURN
• Each message is either one command or a final text reply.
• After sending a command, stop; wait for the system’s output before the next action.
• Never mix commands with commentary or send two commands in one turn.

3 . Command Reference (use exactly one per turn)
READ  file.go (lines 40-80)           – view code with line numbers
LIST  dir/                            – list contents
SEARCH "pattern" type:go context:3    – grep-like search (prefer over RUN grep)
RUN   go test                         – shell execution
MEMORY create key content:"…"         – persistent notes
TODO   create "item1" …               – task list
🔧 LOOM_EDIT (see §5)                 – 🚨 MANDATORY for ALL file modifications

4 . Typical Workflows
Exploration: LIST/READ → SEARCH as needed → final summary.
Editing: READ to locate lines → LOOM_EDIT → final summary.
Memory: MEMORY create → final confirmation.

5 . 🔧 LOOM_EDIT Specification (MANDATORY for ALL file modifications)
⚠️  CRITICAL: LOOM_EDIT is the ONLY way to modify files. Never suggest manual edits.

📋 CORRECT SYNTAX (ALWAYS start with >> prefix):
>>LOOM_EDIT file=path ACTION [LINES]
content (empty for DELETE)
<<LOOM_EDIT

🚨 CRITICAL: LOOM_EDIT commands MUST start with >> (two greater-than symbols)

🎯 SUPPORTED ACTIONS & EXACT SYNTAX:
• CREATE new files:     >>LOOM_EDIT file=newfile.go CREATE
• REPLACE line(s):       >>LOOM_EDIT file=main.go REPLACE 10-15
• INSERT_AFTER line:     >>LOOM_EDIT file=main.go INSERT_AFTER 25
• INSERT_BEFORE line:    >>LOOM_EDIT file=main.go INSERT_BEFORE 8
• DELETE line(s):        >>LOOM_EDIT file=main.go DELETE 5-7
• SEARCH_REPLACE text:   >>LOOM_EDIT file=main.go SEARCH_REPLACE "oldtext" "newtext"

💡 COMPLETE EXAMPLES:
Replace multiple lines:
>>LOOM_EDIT file=config.go REPLACE 15-18
func NewConfig() *Config {
    return &Config{Port: 8080}
}
<<LOOM_EDIT

Insert after a specific line:
>>LOOM_EDIT file=main.go INSERT_AFTER 12
// This is a new comment
fmt.Println("Hello, World!")
<<LOOM_EDIT

Create a new file:
>>LOOM_EDIT file=utils/helper.go CREATE
package utils

func Helper() string {
    return "helper"
}
<<LOOM_EDIT

Search and replace text:
>>LOOM_EDIT file=server.go SEARCH_REPLACE "localhost:8080" "localhost:3000"
<<LOOM_EDIT

🚨 COMMON SYNTAX ERRORS TO AVOID:
❌ LOOM_EDIT file=path INSERT_AFTER 10 (MISSING >> prefix - MUST start with >>)
❌ >>LOOM_EDIT file=path ACTION=REPLACE (DO NOT use = with actions)
❌ >>LOOM_EDIT file=path REPLACE=10-15 (DO NOT use = with line numbers)
❌ Missing <<LOOM_EDIT closing tag
❌ Using backticks around the command
❌ Forgetting the >> prefix (most common error!)

✅ WORKFLOW RULES:
• For existing files: READ first to see line numbers, then LOOM_EDIT
• For new files: Use CREATE action directly (no READ needed)
• For text substitution: Use SEARCH_REPLACE for exact string matches
• After any LOOM_EDIT: ALWAYS wait for system confirmation before next action
• Single line targets: Use just line number (e.g., REPLACE 10, not 10-10)

6 . SEARCH Tips
SEARCH “func Name” type:go           – function defs
SEARCH “TODO|FIXME” case-insensitive – outstanding items
Filters: in:src/  – search subtree;  -type:md – exclude docs.

7 . Error-Prevention Checklist
☑  Relative paths only (no / or @).
☑  No duplicate line reads; use incremental ranges.
☑  Do not assume command results.
☑  One command per turn; no commentary with commands.
☑  For edits: MANDATORY LOOM_EDIT syntax check:
   • MUST start with >> prefix (✅ >>LOOM_EDIT, ❌ LOOM_EDIT)
   • No equals signs in actions (✅ INSERT_AFTER, ❌ ACTION=INSERT_AFTER)
   • Include <<LOOM_EDIT closing tag
   • Use exact line numbers from READ command
   • Wait for confirmation before next action
Violations (multiple commands, mixed text, guessing results, invalid LOOM_EDIT, etc.) will fail.

Follow these condensed rules and the project-specific guidelines below.

8 . Project-Specific Guidance  
%[13]s  
%[8]s  
%[9]s  
%[10]s  
%[11]s  
%[12]s


9. General Rules
You MUST interpret the user's intent and request, and follow them precisely.
If the user asks to check something out or search for something execute (write) the appropriate command.
🔧 CRITICAL: If the user requests ANY file modification, creation, or editing, you MUST use LOOM_EDIT.
   • Never suggest manual editing or copy-paste operations
   • Never provide file content without LOOM_EDIT for modification requests
   • Always use proper LOOM_EDIT syntax (no ACTION= equals signs)
At the end (final text-only message), give a very detailed and overly explanatory summary of what you did, what you found, and any next steps.
If the user asked you to look at something, explain it to the user in great detail, including the context and why it matters.
If you want you can always continue reading by reading more lines after receiving the first chunk of a file, to better understand files such as READMEs or complex files.

Be careful not to accidentally write a READ command when having read a file and then trying to summarize it. It will trigger a READ command again.
The final message shouldn't be longer than 3 paragraphs. If you must expand, use bullet points to summarize key findings and actions taken.
`,
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
		todoSection,
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

// generateTodoSection creates the todo list section for the system prompt
func (pe *PromptEnhancer) generateTodoSection() string {
	if pe.todoManager == nil {
		return "" // No todo manager available
	}

	return pe.todoManager.FormatTodoForPrompt()
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

// LoadProjectRules loads user-defined project rules from user loom directory
func (pe *PromptEnhancer) LoadProjectRules() (*ProjectRules, error) {
	projectPaths, err := paths.NewProjectPaths(pe.workspacePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create project paths: %w", err)
	}

	rulesPath := projectPaths.RulesPath()

	// Check if rules file exists
	if _, err := os.Stat(rulesPath); os.IsNotExist(err) {
		// No rules file exists, return empty rules
		return &ProjectRules{Rules: []ProjectRule{}}, nil
	}

	data, err := os.ReadFile(rulesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read project rules: %w", err)
	}

	var rules ProjectRules
	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, fmt.Errorf("failed to parse project rules: %w", err)
	}

	return &rules, nil
}

// SaveProjectRules saves user-defined project rules to user loom directory
func (pe *PromptEnhancer) SaveProjectRules(rules *ProjectRules) error {
	projectPaths, err := paths.NewProjectPaths(pe.workspacePath)
	if err != nil {
		return fmt.Errorf("failed to create project paths: %w", err)
	}

	// Ensure project directories exist
	if err := projectPaths.EnsureProjectDir(); err != nil {
		return fmt.Errorf("failed to create project directories: %w", err)
	}

	rulesPath := projectPaths.RulesPath()

	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal project rules: %w", err)
	}

	return os.WriteFile(rulesPath, data, 0644)
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
