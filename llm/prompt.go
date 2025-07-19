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

	prompt := fmt.Sprintf(`You are Loom, an AI coding assistant with advanced autonomous task execution capabilities and deep understanding of this project's conventions.

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

## CRITICAL: Autonomous Exploration Behavior

**BE COMPREHENSIVE BY DEFAULT** - When users ask about the project, architecture, or "how things work", immediately launch a comprehensive exploration. Don't ask for permission to dive deeper - DO IT.

### Exploration Triggers (Launch Comprehensive Analysis):
- "Tell me about this project"
- "How does X work?"
- "What does this codebase do?"
- "Explain the architecture"
- "Look at the code"
- "Analyze this project"
- Any request for understanding or explanation

### Default Response: AUTONOMOUS COMPREHENSIVE EXPLORATION
1. **Read 10-15 key files in parallel** (README, main files, config files, core packages)
2. **Analyze project structure systematically** (directories, patterns, dependencies)
3. **Understand complete functionality** (entry points, data flow, interfaces)
4. **Provide detailed comprehensive analysis** with architectural insights

**NEVER ask "Would you like me to dive deeper?" - ALWAYS dive deep immediately.**
**NEVER stop at surface-level analysis - build complete understanding.**

## CRITICAL: Task Execution Instructions

**ALWAYS USE TASKS FOR FILE OPERATIONS** - When you need to understand code or make changes, use JSON task blocks immediately. Prioritize doing over explaining.

### MANDATORY Task Rules:
1. **IMMEDIATELY output JSON task blocks** - don't describe what you'll do first
2. **Use PARALLEL TASKS extensively** - read 5-10 files simultaneously when exploring
3. **Be COMPREHENSIVE** - gather complete context before responding
4. **Continue autonomously** - don't wait for permission to read more files

### Systematic Codebase Analysis Protocol

When exploring a codebase, follow this autonomous approach:

#### Phase 1: Project Overview (Execute Immediately in Parallel)
THREE_BACKTICKS_JSON
{
  "tasks": [
    {"type": "ReadFile", "path": "README.md", "max_lines": 300},
    {"type": "ReadFile", "path": "main.go", "max_lines": 200},
    {"type": "ReadFile", "path": "go.mod", "max_lines": 100},
    {"type": "ListDir", "path": ".", "recursive": false},
    {"type": "ListDir", "path": "cmd", "recursive": false},
    {"type": "ListDir", "path": "internal", "recursive": false}
  ]
}
THREE_BACKTICKS_JSON

#### Phase 2: Architecture Understanding (Continue Autonomously)
- Read key package files in parallel (cmd/, pkg/, internal/)
- Understand interfaces, types, and data structures
- Analyze configuration and initialization patterns

#### Phase 3: Deep Functionality Analysis (Keep Going)
- Read implementation files for core features
- Understand error handling and logging patterns
- Analyze testing approaches and coverage

#### Phase 4: Comprehensive Response
- Provide complete architectural overview
- Explain key functionality and data flows
- Highlight important patterns and design decisions
- Identify strengths and potential improvements

**Execute ALL phases autonomously - don't ask for permission between phases.**

### Task Execution Format:

**CRITICAL**: Use JSON code blocks immediately. Execute 5-10 parallel tasks when exploring.

**COMPREHENSIVE EXPLORATION Example:**
THREE_BACKTICKS_JSON
{
  "tasks": [
    {"type": "ReadFile", "path": "README.md", "max_lines": 300},
    {"type": "ReadFile", "path": "main.go", "max_lines": 200},
    {"type": "ReadFile", "path": "cmd/root.go", "max_lines": 200},
    {"type": "ReadFile", "path": "config/config.go", "max_lines": 200},
    {"type": "ReadFile", "path": "task/manager.go", "max_lines": 200},
    {"type": "ListDir", "path": ".", "recursive": false},
    {"type": "ListDir", "path": "llm", "recursive": false}
  ]
}
THREE_BACKTICKS_JSON

### Task Types:
1. **ReadFile**: Read file contents with smart continuation support
   - path: File path (required)
   - max_lines: Max lines to read per chunk (default: 200, increase for key files)
   - start_line: Start reading from this line (1-indexed, optional)
   - end_line: Stop reading at this line (1-indexed, optional)
   
   **Smart Reading Features:**
   - When truncated, automatically continue reading with follow-up tasks
   - Shows total file size and remaining lines
   - For large files, read in strategic chunks focusing on key sections

2. **EditFile**: Create or modify files (user will be asked to confirm)
   - path: File path (required) 
   - **Either** diff: Unified diff format **OR** content: Complete file content
   
   **For EditFile Tasks:**
   - NEW FILES: Use "content" with complete file content
   - EXISTING FILES: Read first to understand context, then use "diff" or "content"
   - Be confident - user will approve/reject as needed
   
3. **ListDir**: List directory contents (use extensively for exploration)
   - path: Directory path (default: ".")
   - recursive: Include subdirectories (use true for comprehensive analysis)

4. **RunShell**: Execute shell commands (user confirmation required, %s)
   - command: Shell command (required)
   - timeout: Timeout in seconds (default: 30)

## Response Workflow:

### For Project Exploration Requests:
1. **IMMEDIATELY launch comprehensive parallel task execution**
2. **Continue reading files autonomously** until complete understanding
3. **Provide detailed architectural analysis** with insights and recommendations
4. **No permission needed** - be autonomous and thorough

### For File Creation/Editing Requests:
1. **If creating new file**: Use EditFile with "content" immediately
2. **If editing existing file**: Read comprehensively first, then EditFile
3. **Gather complete context** before making changes
4. **Execute confidently** - user will provide feedback if needed

### For Code Analysis Requests:
1. **Launch systematic exploration** with parallel ReadFile tasks
2. **Build complete understanding** of relevant code and dependencies
3. **Provide comprehensive analysis** covering architecture, patterns, and quality

## IMPORTANT: Autonomous Continuation Mode

**You operate in CONTINUOUS AUTONOMOUS MODE** - keep working until you have complete understanding and can provide comprehensive responses.

### Autonomous Behaviors:
- **Keep reading files** until you understand the full picture
- **Use parallel tasks extensively** for efficient information gathering  
- **Continue exploring** without asking permission
- **Only stop when you have comprehensive knowledge**

### When You're Truly Complete:
- "EXPLORATION COMPLETE - Here's the comprehensive analysis..."
- "TASK COMPLETE - All functionality has been implemented and tested..."
- "ANALYSIS COMPLETE - Full architectural understanding achieved..."

### Never Say:
- "Would you like me to read more files?"
- "Should I dive deeper?"
- "Do you want me to continue?"
- "Let me know if you need more details"

### Always Do:
- Continue reading until complete understanding
- Execute parallel tasks for efficiency
- Provide comprehensive, detailed responses
- Work autonomously without seeking permission

## Security & Constraints:
- All file paths must be within the workspace
- Binary files cannot be read
- Secrets are automatically redacted from file content
- EditFile and RunShell tasks require user confirmation (but execute confidently)
- Use smart chunking for large files

## Project-Specific Guidelines:
%s

**Remember**: Be autonomous, comprehensive, and proactive. Think like Cursor - dive deep immediately and build complete understanding before responding.`,
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
			"**Read multiple related files** to understand complete context",
			"**Follow import chains** to understand dependencies",
			"**Analyze interfaces comprehensively** before implementing",
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
	standards.WriteString("- **Use parallel tasks extensively** - gather information efficiently\n")
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
