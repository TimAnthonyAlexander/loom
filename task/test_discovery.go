package task

import (
	"fmt"
	"loom/indexer"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// TestFile represents a discovered test file
type TestFile struct {
	Path         string   `json:"path"`
	Language     string   `json:"language"`
	Framework    string   `json:"framework"`
	TestCount    int      `json:"test_count"`
	TestNames    []string `json:"test_names"`
	RelatedFiles []string `json:"related_files"` // Files this test might be testing
}

// TestResult represents the result of running tests
type TestResult struct {
	Command      string        `json:"command"`
	Success      bool          `json:"success"`
	Output       string        `json:"output"`
	ErrorOutput  string        `json:"error_output"`
	Duration     time.Duration `json:"duration"`
	TestsPassed  int           `json:"tests_passed"`
	TestsFailed  int           `json:"tests_failed"`
	TestsSkipped int           `json:"tests_skipped"`
	FailedTests  []string      `json:"failed_tests"`
	Timestamp    time.Time     `json:"timestamp"`
}

// TestDiscovery handles test file discovery and test execution
type TestDiscovery struct {
	index           *indexer.Index
	testPatterns    map[string][]*regexp.Regexp
	frameworks      map[string]TestFramework
	discoveredTests map[string]*TestFile
}

// TestFramework defines how to run tests for a specific framework
type TestFramework struct {
	Name         string
	Language     string
	TestCommand  string           // Command template to run tests
	FilePatterns []string         // File patterns that indicate this framework
	TestPatterns []*regexp.Regexp // Regex patterns to find individual tests
}

// NewTestDiscovery creates a new test discovery instance
func NewTestDiscovery(index *indexer.Index) *TestDiscovery {
	td := &TestDiscovery{
		index:           index,
		testPatterns:    make(map[string][]*regexp.Regexp),
		frameworks:      make(map[string]TestFramework),
		discoveredTests: make(map[string]*TestFile),
	}

	td.initializePatterns()
	td.initializeFrameworks()
	return td
}

// initializePatterns sets up test file detection patterns
func (td *TestDiscovery) initializePatterns() {
	// Go test patterns
	td.testPatterns["Go"] = []*regexp.Regexp{
		regexp.MustCompile(`_test\.go$`),
	}

	// JavaScript/TypeScript test patterns
	jsPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\.test\.(js|ts)$`),
		regexp.MustCompile(`\.spec\.(js|ts)$`),
		regexp.MustCompile(`__tests__/.*\.(js|ts)$`),
		regexp.MustCompile(`test/.*\.(js|ts)$`),
		regexp.MustCompile(`tests/.*\.(js|ts)$`),
	}
	td.testPatterns["JavaScript"] = jsPatterns
	td.testPatterns["TypeScript"] = jsPatterns

	// Python test patterns
	td.testPatterns["Python"] = []*regexp.Regexp{
		regexp.MustCompile(`test_.*\.py$`),
		regexp.MustCompile(`.*_test\.py$`),
		regexp.MustCompile(`test/.*\.py$`),
		regexp.MustCompile(`tests/.*\.py$`),
	}

	// Rust test patterns
	td.testPatterns["Rust"] = []*regexp.Regexp{
		regexp.MustCompile(`tests/.*\.rs$`),
		regexp.MustCompile(`.*_test\.rs$`),
	}

	// Java test patterns
	td.testPatterns["Java"] = []*regexp.Regexp{
		regexp.MustCompile(`.*Test\.java$`),
		regexp.MustCompile(`test/.*\.java$`),
		regexp.MustCompile(`src/test/.*\.java$`),
	}
}

// initializeFrameworks sets up test framework configurations
func (td *TestDiscovery) initializeFrameworks() {
	// Go testing
	td.frameworks["go"] = TestFramework{
		Name:         "Go Testing",
		Language:     "Go",
		TestCommand:  "go test ./...",
		FilePatterns: []string{"*_test.go"},
		TestPatterns: []*regexp.Regexp{
			regexp.MustCompile(`func\s+(Test\w+)\s*\(`),
			regexp.MustCompile(`func\s+(Benchmark\w+)\s*\(`),
			regexp.MustCompile(`func\s+(Example\w+)\s*\(`),
		},
	}

	// Jest (JavaScript/TypeScript)
	td.frameworks["jest"] = TestFramework{
		Name:         "Jest",
		Language:     "JavaScript",
		TestCommand:  "npm test",
		FilePatterns: []string{"*.test.js", "*.spec.js", "*.test.ts", "*.spec.ts"},
		TestPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?:test|it)\s*\(\s*['"]([^'"]+)['"]`),
			regexp.MustCompile(`describe\s*\(\s*['"]([^'"]+)['"]`),
		},
	}

	// Pytest (Python)
	td.frameworks["pytest"] = TestFramework{
		Name:         "Pytest",
		Language:     "Python",
		TestCommand:  "pytest",
		FilePatterns: []string{"test_*.py", "*_test.py"},
		TestPatterns: []*regexp.Regexp{
			regexp.MustCompile(`def\s+(test_\w+)\s*\(`),
		},
	}

	// Unittest (Python)
	td.frameworks["unittest"] = TestFramework{
		Name:         "Python unittest",
		Language:     "Python",
		TestCommand:  "python -m unittest discover",
		FilePatterns: []string{"test_*.py", "*_test.py"},
		TestPatterns: []*regexp.Regexp{
			regexp.MustCompile(`def\s+(test_\w+)\s*\(`),
		},
	}

	// Cargo test (Rust)
	td.frameworks["cargo"] = TestFramework{
		Name:         "Cargo Test",
		Language:     "Rust",
		TestCommand:  "cargo test",
		FilePatterns: []string{"*_test.rs", "tests/*.rs"},
		TestPatterns: []*regexp.Regexp{
			regexp.MustCompile(`#\[test\]\s*fn\s+(\w+)\s*\(`),
		},
	}
}

// DiscoverTests scans the workspace for test files
func (td *TestDiscovery) DiscoverTests() error {
	td.discoveredTests = make(map[string]*TestFile)

	// Scan all files in the index
	for filePath, fileMeta := range td.index.Files {
		// Check if this file matches any test patterns
		if td.isTestFile(filePath, fileMeta.Language) {
			testFile := &TestFile{
				Path:      filePath,
				Language:  fileMeta.Language,
				Framework: td.detectFramework(filePath, fileMeta.Language),
				TestNames: []string{},
			}

			// Extract test names from the file
			testNames, err := td.extractTestNames(filePath, fileMeta.Language)
			if err == nil {
				testFile.TestNames = testNames
				testFile.TestCount = len(testNames)
			}

			// Find related files (files this test might be testing)
			testFile.RelatedFiles = td.findRelatedFiles(filePath)

			td.discoveredTests[filePath] = testFile
		}
	}

	return nil
}

// isTestFile checks if a file is a test file based on patterns
func (td *TestDiscovery) isTestFile(filePath, language string) bool {
	patterns, exists := td.testPatterns[language]
	if !exists {
		return false
	}

	for _, pattern := range patterns {
		if pattern.MatchString(filePath) {
			return true
		}
	}

	return false
}

// detectFramework detects which test framework is used based on file patterns
func (td *TestDiscovery) detectFramework(filePath, language string) string {
	for name, framework := range td.frameworks {
		if framework.Language == language {
			for _, pattern := range framework.FilePatterns {
				if matched, _ := filepath.Match(pattern, filepath.Base(filePath)); matched {
					return name
				}
			}
		}
	}
	return "unknown"
}

// extractTestNames extracts individual test names from a test file
func (td *TestDiscovery) extractTestNames(filePath, language string) ([]string, error) {
	// Find the appropriate framework
	var framework *TestFramework
	for _, fw := range td.frameworks {
		if fw.Language == language {
			framework = &fw
			break
		}
	}

	if framework == nil {
		return []string{}, nil
	}

	// Read the file content
	fullPath := filepath.Join(td.index.WorkspacePath, filePath)
	content, err := td.readFileContent(fullPath)
	if err != nil {
		return nil, err
	}

	var testNames []string

	// Extract test names using framework patterns
	for _, pattern := range framework.TestPatterns {
		matches := pattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				testNames = append(testNames, match[1])
			}
		}
	}

	return testNames, nil
}

// findRelatedFiles finds source files that might be tested by this test file
func (td *TestDiscovery) findRelatedFiles(testPath string) []string {
	var related []string

	// Basic heuristic: look for files with similar names
	dir := filepath.Dir(testPath)
	base := filepath.Base(testPath)

	// Remove test suffixes to find potential source files
	sourceBase := strings.ReplaceAll(base, "_test.", ".")
	sourceBase = strings.ReplaceAll(sourceBase, ".test.", ".")
	sourceBase = strings.ReplaceAll(sourceBase, ".spec.", ".")
	sourceBase = strings.ReplaceAll(sourceBase, "test_", "")

	// Look for files with the derived name
	for filePath := range td.index.Files {
		if filepath.Dir(filePath) == dir && filepath.Base(filePath) == sourceBase {
			related = append(related, filePath)
		}
	}

	return related
}

// GetDiscoveredTests returns all discovered test files
func (td *TestDiscovery) GetDiscoveredTests() map[string]*TestFile {
	return td.discoveredTests
}

// GetTestsByLanguage returns test files for a specific language
func (td *TestDiscovery) GetTestsByLanguage(language string) []*TestFile {
	var tests []*TestFile
	for _, test := range td.discoveredTests {
		if test.Language == language {
			tests = append(tests, test)
		}
	}
	return tests
}

// GetTestCount returns the total number of discovered tests
func (td *TestDiscovery) GetTestCount() int {
	total := 0
	for _, test := range td.discoveredTests {
		total += test.TestCount
	}
	return total
}

// RunTests executes tests for a specific language or framework
func (td *TestDiscovery) RunTests(language string, workspacePath string, timeout time.Duration) (*TestResult, error) {
	// Find appropriate framework
	var framework *TestFramework
	for _, fw := range td.frameworks {
		if fw.Language == language {
			framework = &fw
			break
		}
	}

	if framework == nil {
		return nil, fmt.Errorf("no test framework configured for language: %s", language)
	}

	startTime := time.Now()

	// Execute the test command
	cmd := exec.Command("sh", "-c", framework.TestCommand)
	cmd.Dir = workspacePath

	output, err := cmd.CombinedOutput()
	duration := time.Since(startTime)

	result := &TestResult{
		Command:   framework.TestCommand,
		Success:   err == nil,
		Output:    string(output),
		Duration:  duration,
		Timestamp: time.Now(),
	}

	if err != nil {
		result.ErrorOutput = err.Error()
	}

	// Parse test results (basic implementation)
	result.parseTestOutput(language)

	return result, nil
}

// parseTestOutput parses test output to extract test counts and failed tests
func (tr *TestResult) parseTestOutput(language string) {
	output := tr.Output

	switch language {
	case "Go":
		tr.parseGoTestOutput(output)
	case "JavaScript", "TypeScript":
		tr.parseJestOutput(output)
	case "Python":
		tr.parsePythonTestOutput(output)
	}
}

// parseGoTestOutput parses Go test output
func (tr *TestResult) parseGoTestOutput(output string) {
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if strings.Contains(line, "PASS") || strings.Contains(line, "ok") {
			tr.TestsPassed++
		} else if strings.Contains(line, "FAIL") {
			tr.TestsFailed++
			// Extract failed test name
			parts := strings.Fields(line)
			if len(parts) > 1 {
				tr.FailedTests = append(tr.FailedTests, parts[1])
			}
		}
	}
}

// parseJestOutput parses Jest test output
func (tr *TestResult) parseJestOutput(output string) {
	// Look for Jest summary lines
	passedPattern := regexp.MustCompile(`(\d+) passing`)
	failedPattern := regexp.MustCompile(`(\d+) failing`)

	if matches := passedPattern.FindStringSubmatch(output); matches != nil {
		if count, err := parseIntSafe(matches[1]); err == nil {
			tr.TestsPassed = count
		}
	}

	if matches := failedPattern.FindStringSubmatch(output); matches != nil {
		if count, err := parseIntSafe(matches[1]); err == nil {
			tr.TestsFailed = count
		}
	}
}

// parsePythonTestOutput parses Python test output (pytest/unittest)
func (tr *TestResult) parsePythonTestOutput(output string) {
	// Look for pytest summary
	if strings.Contains(output, "passed") {
		passedPattern := regexp.MustCompile(`(\d+) passed`)
		if matches := passedPattern.FindStringSubmatch(output); matches != nil {
			if count, err := parseIntSafe(matches[1]); err == nil {
				tr.TestsPassed = count
			}
		}
	}

	if strings.Contains(output, "failed") {
		failedPattern := regexp.MustCompile(`(\d+) failed`)
		if matches := failedPattern.FindStringSubmatch(output); matches != nil {
			if count, err := parseIntSafe(matches[1]); err == nil {
				tr.TestsFailed = count
			}
		}
	}
}

// FormatSummary formats test discovery results for display
func (td *TestDiscovery) FormatSummary() string {
	var result strings.Builder

	result.WriteString("🧪 Test Discovery Summary\n")
	result.WriteString(strings.Repeat("=", 40) + "\n\n")

	if len(td.discoveredTests) == 0 {
		result.WriteString("No test files found.\n")
		return result.String()
	}

	// Group by language
	byLanguage := make(map[string][]*TestFile)
	for _, test := range td.discoveredTests {
		byLanguage[test.Language] = append(byLanguage[test.Language], test)
	}

	for language, tests := range byLanguage {
		result.WriteString(fmt.Sprintf("📋 %s Tests:\n", language))

		totalTests := 0
		for _, test := range tests {
			result.WriteString(fmt.Sprintf("  📄 %s (%d tests)\n", test.Path, test.TestCount))
			totalTests += test.TestCount

			// Show some test names
			if len(test.TestNames) > 0 {
				maxShow := 3
				if len(test.TestNames) < maxShow {
					maxShow = len(test.TestNames)
				}
				for i := 0; i < maxShow; i++ {
					result.WriteString(fmt.Sprintf("    • %s\n", test.TestNames[i]))
				}
				if len(test.TestNames) > maxShow {
					result.WriteString(fmt.Sprintf("    ... and %d more\n", len(test.TestNames)-maxShow))
				}
			}
		}

		result.WriteString(fmt.Sprintf("  Total: %d test files, %d individual tests\n\n", len(tests), totalTests))
	}

	return result.String()
}

// Helper functions

func (td *TestDiscovery) readFileContent(filePath string) (string, error) {
	fullPath := filepath.Join(td.index.WorkspacePath, filePath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func parseIntSafe(s string) (int, error) {
	return strconv.Atoi(s)
}
