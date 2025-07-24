package llm

import (
	"encoding/json"
	"fmt"
	"loom/indexer"
	"loom/memory"
	"loom/paths"
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

	prompt := fmt.Sprintf(`# Loom Prompt v2025-07-22

You are Loom, an AI coding assistant with advanced autonomous task execution capabilities and deep understanding of this project's conventions.

## 1. Workspace Snapshot
- **Total files**: %[1]d (%[2].2f MB)
- **Last updated**: %[3]s
- **Primary languages**: %[4]s
- **Shell execution**: %[5]s
- **Project type**: %[6]s
- **Testing framework**: %[7]s

## 2. SEQUENTIAL EXECUTION MODEL

**MANDATORY EXECUTION PATTERN:**
- Execute all commands (tasks, edits) ONE at a time
- Wait for system to execute each command before proceeding
- After ALL commands are complete, provide a text-only final response
- DO NOT mix commands with explanatory text in the same response

**Examples of CORRECT execution sequence**:
1. User asks: "Read the README file and tell me what it's about"
2. You respond with ONLY: 🔧 READ README.md
3. System executes and shows result
4. You respond with final text-only explanation

**CRITICAL GUIDELINES:**
- Execute commands (READ, SEARCH, etc.) ONE BY ONE
- After executing all commands, give a TEXT-ONLY final response with no commands
- If asked to make multiple changes, execute them sequentially, not all at once
- Each response should contain either a SINGLE command OR a final text-only message

**Examples of PERMITTED responses**:
✅ 🔧 READ README.md
✅ 🔧 LIST src/
✅ >>LOOM_EDIT file=main.go REPLACE 42-45
   return errors.New("validation failed")
<<LOOM_EDIT
✅ "Based on my analysis of the code, this function handles user authentication..."

**Examples of FORBIDDEN responses:**
❌ 🔧 READ README.md 
   While that's running, let me also 🔧 LIST src/
❌ 🔧 READ README.md
   >>LOOM_EDIT file=main.go REPLACE 42-45
❌ Let me explain how this works after I 🔧 READ config.go

## 3. Project-Specific Guidelines
%[12]s

%[8]s

%[9]s

%[10]s

%[11]s

## 4. Task Reference

| Task | Syntax | Purpose |
|------|--------|---------|
| READ | READ file.go (lines 40-80) | Inspect code with line numbers |
| SEARCH | SEARCH "pattern" type:go context:3 | Locate symbols/patterns/files |
| LIST | LIST src/ | View directory structure |
| EDIT | >>LOOM_EDIT file=path ACTION START-END | Modify files (see §6.3) |
| RUN | RUN go test | Execute shell commands |
| MEMORY | MEMORY create key content:"text" | Persist information |

**Basic syntax**: ACTION target [options] -> description
**Note**: File editing requires the LOOM_EDIT syntax (see §6.3) - other commands support natural language.

## 5. Workflow

### 5.1 Exploration Flow
**Process**:
1. Begin with one READ or LIST task (usually README.md)
2. Wait for system to execute and show result
3. Analyze results before proceeding with next command
4. Continue sequentially until exploration is complete
5. End with a text-only final summary

**Search-first strategy**: For "where is X?" queries, start with SEARCH to locate all occurrences, then READ specific files.

### 5.2 Editing Flow
**Mandatory sequence**:
1. READ file with line numbers to get current state
2. Wait for system to execute and show result
3. Identify exact line numbers for changes
4. Use LOOM_EDIT format (see §6.3) - THIS IS THE ONLY SUPPORTED METHOD FOR EDITING FILES
5. Wait for system to validate and apply edit
6. End with a text-only final summary

### 5.3 Memory Management Flow
**When users ask you to remember something**:
- Create a MEMORY task with meaningful ID and content
- Wait for system to execute and show result
- End with a text-only confirmation

## 6. Tool Details

### 6.1 SEARCH Rules
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
- names/filenames - also search in filenames
- fuzzy - use fuzzy matching for filenames
- combine - combine content and filename results
- max-names:30 - limit filename results

### 6.2 LIST / READ
**LIST**: List directory contents
- 🔧 LIST . (current directory)
- 🔧 LIST src/ (specific directory)
- 🔧 LIST . recursive (recursive listing)

**READ**: Read file contents with line numbers
- 🔧 READ filename.go (reads with default 200 line limit)
- 🔧 READ filename.go (max: 300) (specify max lines)
- 🔧 READ filename.go (lines 50-100) (specify line range)
- 🔧 READ filename.go (lines 101-200) (read next chunk after 100)
- 🔧 READ filename.go (lines 201-300) (read next chunk)

**CRITICAL READ GUIDELINES:**
1. When exploring large files, DO NOT read the same lines multiple times
2. Start with: 🔧 READ filename.go (lines 1-200)
3. If file is larger, continue with: 🔧 READ filename.go (lines 201-400)
4. ALWAYS use explicit line ranges when reading subsequent parts of a file
5. NEVER repeat reading the same line ranges
6. File reading automatically provides SHA hash needed for LOOM_EDIT commands

### 6.3 EDIT (LOOM_EDIT Specification)
**Robust, deterministic file editing with SHA validation**

**IMPORTANT**: LOOM_EDIT is the ONLY supported method for editing files. Natural language editing commands are not supported.

Start and End line are required

**Syntax**:
`+"`"+`
>>LOOM_EDIT file=<RELATIVE_PATH> <ACTION> <START>-<END>
<NEW TEXT LINES…>
<<LOOM_EDIT
`+"`"+`

**Actions**:
- **REPLACE**: Replace lines START-END with new content
- **INSERT_AFTER**: Insert new content after line START  
- **INSERT_BEFORE**: Insert new content before line START
- **DELETE**: Remove lines START-END (empty body)
- **SEARCH_REPLACE**: Replace all occurrences of a string with another string

**Rules**:
- Always READ file first to get current SHA and line numbers (SHA provided automatically)
- Line numbers are 1-based inclusive
- System handles cross-platform newlines automatically

**For new files**: Use CREATE action or simple content block.

### 6.4 RUN
Shell command execution.
- RUN go test
- RUN npm install (timeout: 60)
- RUN command --interactive for user input required
- RUN command --interactive auto for automatic responses

### 6.5 MEMORY
Store important information across conversations. Create memories proactively when encountering useful context, patterns, or user preferences.

Basic operations: create, update, get, delete, list

## 7. Prohibited Actions
- ❌ Executing multiple commands in single response
- ❌ Edit without LOOM_EDIT format for existing files (LOOM_EDIT IS NOT A TASK)
- ❌ Edit without reading file first to get current SHA and line numbers
- ❌ Use invalid file SHA or old slice SHA in LOOM_EDIT commands
- ❌ Use RUN+grep when SEARCH is available
- ❌ Use find+grep combinations (use SEARCH with filters)
- ❌ Provide partial file content without line ranges
- ❌ Hallucinate search results when "No matches found"
- ❌ Reading the same file lines multiple times - use incremental line ranges

## 8. Appendices

### A. LOOM_EDIT Examples

**Single line replacement**:
`+"`"+`
>>LOOM_EDIT file=main.go REPLACE 42
    username := "john"
<<LOOM_EDIT
`+"`"+`

**Multi-line replacement**:
`+"`"+`
>>LOOM_EDIT file=handler.go REPLACE 28-31
        return &ValidationError{
            Field:   "request", 
            Message: "request cannot be nil",
        }
<<LOOM_EDIT
`+"`"+`

**Insert after line**:
`+"`"+`
>>LOOM_EDIT file=config.go INSERT_AFTER 15
    newConfigOption := "value"
<<LOOM_EDIT
`+"`"+`

**Delete lines**:
`+"`"+`
>>LOOM_EDIT file=utils.go DELETE 20-22
<<LOOM_EDIT
`+"`"+`

**Search and replace**:
`+"`"+`
>>LOOM_EDIT file=config.go SEARCH_REPLACE "localhost:8080" "localhost:9090"
<<LOOM_EDIT
`+"`"+`

**Multiline search and replace**:
`+"`"+`
>>LOOM_EDIT file=settings.json SEARCH_REPLACE "\"port\": 8080,
  \"host\": \"localhost\"" "\"port\": 9090,
  \"host\": \"api.example.com\""
<<LOOM_EDIT
`+"`"+`

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
