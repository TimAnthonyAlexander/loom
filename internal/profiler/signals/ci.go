package signals

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/loom/loom/internal/profiler/shared"
)

// CIExtractor extracts information from CI/CD configuration files
type CIExtractor struct {
	root string
}

// NewCIExtractor creates a new CI extractor
func NewCIExtractor(root string) *CIExtractor {
	return &CIExtractor{root: root}
}

// Extract processes CI files and returns signals
func (c *CIExtractor) Extract(files []*shared.FileInfo, existing *shared.SignalData) {
	if existing.CIRefs == nil {
		existing.CIRefs = make(map[string][]string)
	}

	for _, file := range files {
		if c.isCIFile(file.Path) {
			c.extractCI(file.Path, existing)
		}
	}
}

// isCIFile checks if a file is a CI configuration file
func (c *CIExtractor) isCIFile(path string) bool {
	lowerPath := strings.ToLower(path)

	ciPatterns := []string{
		".github/workflows/",
		".gitlab-ci.yml",
		"gitlab-ci.yml",
		".circleci/config.yml",
		"circleci/config.yml",
		"azure-pipelines.yml",
		".azure-pipelines.yml",
		"appveyor.yml",
		".appveyor.yml",
		"bitbucket-pipelines.yml",
		".bitbucket-pipelines.yml",
		"buildkite.yml",
		".buildkite.yml",
		"drone.yml",
		".drone.yml",
		"jenkins.yml",
		"jenkinsfile",
	}

	for _, pattern := range ciPatterns {
		if strings.Contains(lowerPath, pattern) {
			return true
		}
	}

	return false
}

// extractCI processes a CI configuration file
func (c *CIExtractor) extractCI(path string, signals *shared.SignalData) {
	lowerPath := strings.ToLower(path)

	switch {
	case strings.Contains(lowerPath, ".github/workflows/"):
		c.extractGitHubActions(path, signals)
	case strings.Contains(lowerPath, "gitlab-ci.yml"):
		c.extractGitLabCI(path, signals)
	case strings.Contains(lowerPath, ".circleci/config.yml"):
		c.extractCircleCI(path, signals)
	case strings.Contains(lowerPath, "azure-pipelines.yml"):
		c.extractAzurePipelines(path, signals)
	default:
		// Generic CI file processing
		c.extractGenericCI(path, signals)
	}
}

// extractGitHubActions processes GitHub Actions workflow files
func (c *CIExtractor) extractGitHubActions(path string, signals *shared.SignalData) {
	file, err := os.Open(filepath.Join(c.root, path))
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()

	var jobs []string
	var currentJob string
	var inRun bool
	var runCommands []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Skip comments and empty lines
		if strings.HasPrefix(trimmed, "#") || trimmed == "" {
			continue
		}

		// Look for job definitions
		if strings.HasPrefix(line, "  ") && strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(line, "    ") {
			// Save previous job
			if currentJob != "" {
				jobs = append(jobs, currentJob)
				if len(runCommands) > 0 {
					signals.CIRefs[currentJob] = c.extractPathsFromCommands(runCommands)
				}
			}

			currentJob = strings.TrimSuffix(trimmed, ":")
			runCommands = nil
			inRun = false
		} else if strings.Contains(trimmed, "run:") {
			inRun = true
			// Extract command from same line if present
			if strings.Contains(trimmed, "run:") {
				parts := strings.SplitN(trimmed, "run:", 2)
				if len(parts) > 1 {
					cmd := strings.TrimSpace(parts[1])
					if cmd != "" && cmd != "|" {
						runCommands = append(runCommands, cmd)
					}
				}
			}
		} else if inRun && (strings.HasPrefix(line, "        ") || strings.HasPrefix(line, "\t\t")) {
			// Multi-line run command
			runCommands = append(runCommands, trimmed)
		} else if !strings.HasPrefix(line, "      ") && inRun {
			inRun = false
		}
	}

	// Don't forget the last job
	if currentJob != "" {
		jobs = append(jobs, currentJob)
		if len(runCommands) > 0 {
			signals.CIRefs[currentJob] = c.extractPathsFromCommands(runCommands)
		}
	}

	if len(jobs) > 0 {
		signals.CI = append(signals.CI, shared.CIConfig{
			Path: path,
			Jobs: jobs,
		})
	}
}

// extractGitLabCI processes GitLab CI configuration
func (c *CIExtractor) extractGitLabCI(path string, signals *shared.SignalData) {
	file, err := os.Open(filepath.Join(c.root, path))
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()

	var jobs []string
	var currentJob string
	var inScript bool
	var scriptCommands []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Skip comments and empty lines
		if strings.HasPrefix(trimmed, "#") || trimmed == "" {
			continue
		}

		// Look for job definitions (not starting with .)
		if !strings.HasPrefix(line, " ") && strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(trimmed, ".") {
			// Save previous job
			if currentJob != "" {
				jobs = append(jobs, currentJob)
				if len(scriptCommands) > 0 {
					signals.CIRefs[currentJob] = c.extractPathsFromCommands(scriptCommands)
				}
			}

			currentJob = strings.TrimSuffix(trimmed, ":")
			scriptCommands = nil
			inScript = false
		} else if strings.Contains(trimmed, "script:") {
			inScript = true
		} else if inScript && strings.HasPrefix(line, "    - ") {
			cmd := strings.TrimPrefix(trimmed, "- ")
			scriptCommands = append(scriptCommands, cmd)
		} else if !strings.HasPrefix(line, "  ") && inScript {
			inScript = false
		}
	}

	// Don't forget the last job
	if currentJob != "" {
		jobs = append(jobs, currentJob)
		if len(scriptCommands) > 0 {
			signals.CIRefs[currentJob] = c.extractPathsFromCommands(scriptCommands)
		}
	}

	if len(jobs) > 0 {
		signals.CI = append(signals.CI, shared.CIConfig{
			Path: path,
			Jobs: jobs,
		})
	}
}

// extractCircleCI processes CircleCI configuration
func (c *CIExtractor) extractCircleCI(path string, signals *shared.SignalData) {
	file, err := os.Open(filepath.Join(c.root, path))
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()

	var jobs []string
	var currentJob string
	var inRun bool
	var runCommands []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Skip comments and empty lines
		if strings.HasPrefix(trimmed, "#") || trimmed == "" {
			continue
		}

		// Look for job definitions under jobs:
		if strings.HasPrefix(line, "    ") && strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(line, "      ") {
			// Save previous job
			if currentJob != "" {
				jobs = append(jobs, currentJob)
				if len(runCommands) > 0 {
					signals.CIRefs[currentJob] = c.extractPathsFromCommands(runCommands)
				}
			}

			currentJob = strings.TrimSuffix(trimmed, ":")
			runCommands = nil
			inRun = false
		} else if strings.Contains(trimmed, "run:") {
			inRun = true
			// Extract command from same line if present
			if strings.Contains(trimmed, "run:") {
				parts := strings.SplitN(trimmed, "run:", 2)
				if len(parts) > 1 {
					cmd := strings.TrimSpace(parts[1])
					if cmd != "" && cmd != "|" {
						runCommands = append(runCommands, cmd)
					}
				}
			}
		} else if inRun && (strings.HasPrefix(line, "          ") || strings.HasPrefix(line, "\t\t\t")) {
			// Multi-line run command
			runCommands = append(runCommands, trimmed)
		} else if !strings.HasPrefix(line, "        ") && inRun {
			inRun = false
		}
	}

	// Don't forget the last job
	if currentJob != "" {
		jobs = append(jobs, currentJob)
		if len(runCommands) > 0 {
			signals.CIRefs[currentJob] = c.extractPathsFromCommands(runCommands)
		}
	}

	if len(jobs) > 0 {
		signals.CI = append(signals.CI, shared.CIConfig{
			Path: path,
			Jobs: jobs,
		})
	}
}

// extractAzurePipelines processes Azure Pipelines configuration
func (c *CIExtractor) extractAzurePipelines(path string, signals *shared.SignalData) {
	file, err := os.Open(filepath.Join(c.root, path))
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()

	var jobs []string
	var currentJob string
	var inScript bool
	var scriptCommands []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Skip comments and empty lines
		if strings.HasPrefix(trimmed, "#") || trimmed == "" {
			continue
		}

		// Look for job definitions
		if strings.HasPrefix(line, "- job:") || strings.HasPrefix(line, "- task:") {
			// Save previous job
			if currentJob != "" {
				jobs = append(jobs, currentJob)
				if len(scriptCommands) > 0 {
					signals.CIRefs[currentJob] = c.extractPathsFromCommands(scriptCommands)
				}
			}

			if strings.HasPrefix(line, "- job:") {
				currentJob = strings.TrimSpace(strings.TrimPrefix(trimmed, "- job:"))
			} else {
				currentJob = strings.TrimSpace(strings.TrimPrefix(trimmed, "- task:"))
			}
			scriptCommands = nil
			inScript = false
		} else if strings.Contains(trimmed, "script:") {
			inScript = true
			// Extract command from same line if present
			parts := strings.SplitN(trimmed, "script:", 2)
			if len(parts) > 1 {
				cmd := strings.TrimSpace(parts[1])
				if cmd != "" && cmd != "|" {
					scriptCommands = append(scriptCommands, cmd)
				}
			}
		} else if inScript && (strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t")) {
			scriptCommands = append(scriptCommands, trimmed)
		} else if !strings.HasPrefix(line, "  ") && inScript {
			inScript = false
		}
	}

	// Don't forget the last job
	if currentJob != "" {
		jobs = append(jobs, currentJob)
		if len(scriptCommands) > 0 {
			signals.CIRefs[currentJob] = c.extractPathsFromCommands(scriptCommands)
		}
	}

	if len(jobs) > 0 {
		signals.CI = append(signals.CI, shared.CIConfig{
			Path: path,
			Jobs: jobs,
		})
	}
}

// extractGenericCI processes generic CI files
func (c *CIExtractor) extractGenericCI(path string, signals *shared.SignalData) {
	file, err := os.Open(filepath.Join(c.root, path))
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()

	var allCommands []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		// Collect all lines that look like commands
		if c.looksLikeCommand(line) {
			allCommands = append(allCommands, line)
		}
	}

	if len(allCommands) > 0 {
		baseName := filepath.Base(path)
		signals.CI = append(signals.CI, shared.CIConfig{
			Path: path,
			Jobs: []string{baseName},
		})
		signals.CIRefs[baseName] = c.extractPathsFromCommands(allCommands)
	}
}

// looksLikeCommand checks if a line looks like a command
func (c *CIExtractor) looksLikeCommand(line string) bool {
	// Simple heuristics for command detection
	commands := []string{
		"npm", "yarn", "pnpm", "node",
		"go", "cargo", "rustc",
		"python", "pip", "poetry",
		"php", "composer",
		"make", "cmake",
		"docker", "kubectl",
		"git", "curl", "wget",
		"test", "build", "deploy",
		"lint", "format", "check",
	}

	lowerLine := strings.ToLower(line)
	for _, cmd := range commands {
		if strings.Contains(lowerLine, cmd) {
			return true
		}
	}

	return false
}

// extractPathsFromCommands extracts file paths from a list of commands
func (c *CIExtractor) extractPathsFromCommands(commands []string) []string {
	var allPaths []string

	for _, cmd := range commands {
		// Normalize environment variables to prevent fake paths
		normalizedCmd := c.normalizeEnvVars(cmd)

		// Extract paths from the normalized command
		paths := c.extractPathsFromCommand(normalizedCmd)
		allPaths = append(allPaths, paths...)
	}

	// Remove duplicates
	seen := make(map[string]bool)
	var unique []string
	for _, path := range allPaths {
		if !seen[path] {
			seen[path] = true
			unique = append(unique, path)
		}
	}

	return unique
}

// normalizeEnvVars replaces environment variable references with placeholder tokens
func (c *CIExtractor) normalizeEnvVars(cmd string) string {
	// Replace ${VAR} and $VAR patterns with placeholder tokens
	envVarPattern := regexp.MustCompile(`\$\{[A-Z_][A-Z0-9_]*\}|\$[A-Z_][A-Z0-9_]*`)
	return envVarPattern.ReplaceAllString(cmd, "ENV_VAR_TOKEN")
}

// extractPathsFromCommand extracts file paths from a single command with glob support
func (c *CIExtractor) extractPathsFromCommand(cmd string) []string {
	var paths []string

	// Regex patterns for common path patterns in CI commands
	patterns := []*regexp.Regexp{
		// Paths with wildcards: src/**/*.ts, test/*.go
		regexp.MustCompile(`[./]?(?:src|app|cmd|internal|ui|frontend|backend|lib|pkg|test|tests)/[a-zA-Z0-9_/*.-]+\.[a-zA-Z0-9*]+`),
		// Regular file paths with extensions
		regexp.MustCompile(`[./]?[a-zA-Z0-9_-]+\.(ts|tsx|js|jsx|go|php|py|rb|rs|c|cpp|h|hpp|yaml|yml|json|toml|md|txt)`),
		// Directory paths
		regexp.MustCompile(`(?:src|app|cmd|internal|ui)/[a-zA-Z0-9_/*.-]*`),
		// Relative paths
		regexp.MustCompile(`\./[a-zA-Z0-9_/*.-]+`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllString(cmd, -1)
		for _, match := range matches {
			// Clean up the match
			cleaned := strings.Trim(match, `"'`+"`"+`()[]{}`)
			if cleaned != "" && !c.isCommonNonPath(cleaned) {
				// Handle glob patterns
				if strings.Contains(cleaned, "*") {
					expandedPaths := c.expandGlobPath(cleaned)
					paths = append(paths, expandedPaths...)
				} else {
					paths = append(paths, cleaned)
				}
			}
		}
	}

	return paths
}

// expandGlobPath expands glob patterns to actual file paths with limits
func (c *CIExtractor) expandGlobPath(globPattern string) []string {
	const maxExpansion = 200 // Cap expansion to prevent graph blow-ups

	// For common patterns, return representative paths rather than expanding everything
	if strings.Contains(globPattern, "**/*") {
		return c.generateRepresentativePaths(globPattern)
	}

	// Try to expand the glob pattern
	matches, err := filepath.Glob(filepath.Join(c.root, globPattern))
	if err != nil {
		// If glob fails, return the pattern as-is (might still be useful)
		return []string{globPattern}
	}

	var result []string
	for i, match := range matches {
		if i >= maxExpansion {
			break // Cap the number of matches
		}

		// Convert back to relative path
		relPath, err := filepath.Rel(c.root, match)
		if err == nil {
			result = append(result, relPath)
		}
	}

	// If no matches found, still include the pattern for potential future matching
	if len(result) == 0 {
		result = append(result, globPattern)
	}

	return result
}

// generateRepresentativePaths creates representative paths for common glob patterns
func (c *CIExtractor) generateRepresentativePaths(globPattern string) []string {
	var paths []string

	// Extract the base directory and extension pattern
	if strings.Contains(globPattern, "src/**/*.ts") {
		paths = append(paths, "src/index.ts", "src/main.ts", "src/app.ts")
	} else if strings.Contains(globPattern, "test/**/*.go") {
		paths = append(paths, "test/main_test.go", "test/integration_test.go")
	} else if strings.Contains(globPattern, "**/*.js") {
		paths = append(paths, "src/index.js", "lib/main.js")
	} else if strings.Contains(globPattern, "**/*.yaml") || strings.Contains(globPattern, "**/*.yml") {
		paths = append(paths, "config/app.yaml", "k8s/deployment.yaml")
	} else {
		// Generic pattern - try to create a representative path
		pattern := strings.ReplaceAll(globPattern, "**/*", "main")
		pattern = strings.ReplaceAll(pattern, "*", "index")
		paths = append(paths, pattern)
	}

	return paths
}

// isCommonNonPath checks if a string is commonly found in CI but not a file path
func (c *CIExtractor) isCommonNonPath(str string) bool {
	nonPaths := map[string]bool{
		"bin":  true,
		"usr":  true,
		"etc":  true,
		"var":  true,
		"tmp":  true,
		"dev":  true,
		"run":  true,
		"home": true,
		"root": true,
		"opt":  true,
		"proc": true,
		"sys":  true,
	}

	return nonPaths[strings.ToLower(str)]
}
