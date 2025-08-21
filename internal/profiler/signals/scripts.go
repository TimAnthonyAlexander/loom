package signals

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/loom/loom/internal/profiler/shared"
)

// ScriptExtractor extracts information from various script files
type ScriptExtractor struct {
	root string
}

// NewScriptExtractor creates a new script extractor
func NewScriptExtractor(root string) *ScriptExtractor {
	return &ScriptExtractor{root: root}
}

// Extract processes script files and returns signals
func (s *ScriptExtractor) Extract(files []*shared.FileInfo, existing *shared.SignalData) {
	if existing.ScriptRefs == nil {
		existing.ScriptRefs = make(map[string][]string)
	}

	for _, file := range files {
		switch strings.ToLower(file.Basename) {
		case "makefile":
			s.extractMakefile(file.Path, existing)
		case "justfile":
			s.extractJustfile(file.Path, existing)
		case "taskfile.yml", "taskfile.yaml":
			s.extractTaskfile(file.Path, existing)
		case "procfile":
			s.extractProcfile(file.Path, existing)
		default:
			// Check if it's a shell script
			if s.isShellScript(file.Extension) {
				s.extractShellScript(file.Path, existing)
			}
		}
	}
}

// isShellScript checks if the file is a shell script
func (s *ScriptExtractor) isShellScript(ext string) bool {
	shellExts := map[string]bool{
		".sh":   true,
		".bash": true,
		".zsh":  true,
		".fish": true,
		".bat":  true,
		".cmd":  true,
		".ps1":  true,
	}
	return shellExts[ext]
}

// extractMakefile processes Makefile
func (s *ScriptExtractor) extractMakefile(path string, signals *shared.SignalData) {
	file, err := os.Open(filepath.Join(s.root, path))
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	var currentTarget string
	var currentRecipe strings.Builder
	var inRecipe bool

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		// Check if this is a target line (contains :)
		if strings.Contains(line, ":") && !strings.HasPrefix(line, "\t") {
			// Save previous target if we have one
			if currentTarget != "" && currentRecipe.Len() > 0 {
				s.addMakeScript(currentTarget, currentRecipe.String(), signals)
			}

			// Start new target
			parts := strings.SplitN(line, ":", 2)
			currentTarget = strings.TrimSpace(parts[0])
			currentRecipe.Reset()
			inRecipe = true

			// If there's a command on the same line after :
			if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
				currentRecipe.WriteString(strings.TrimSpace(parts[1]))
			}
		} else if inRecipe && (strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "    ")) {
			// This is part of the recipe
			if currentRecipe.Len() > 0 {
				currentRecipe.WriteString(" && ")
			}
			currentRecipe.WriteString(strings.TrimSpace(line))
		} else {
			// End of recipe
			if currentTarget != "" && currentRecipe.Len() > 0 {
				s.addMakeScript(currentTarget, currentRecipe.String(), signals)
			}
			currentTarget = ""
			currentRecipe.Reset()
			inRecipe = false
		}
	}

	// Don't forget the last target
	if currentTarget != "" && currentRecipe.Len() > 0 {
		s.addMakeScript(currentTarget, currentRecipe.String(), signals)
	}
}

// addMakeScript adds a Makefile script to signals
func (s *ScriptExtractor) addMakeScript(target, recipe string, signals *shared.SignalData) {
	script := shared.Script{
		Name:   target,
		Source: "make",
		Cmd:    "make " + target,
		Paths:  s.extractPathsFromCommand(recipe),
	}
	signals.Scripts = append(signals.Scripts, script)
	signals.ScriptRefs[target] = script.Paths
}

// extractJustfile processes Justfile
func (s *ScriptExtractor) extractJustfile(path string, signals *shared.SignalData) {
	file, err := os.Open(filepath.Join(s.root, path))
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	var currentTarget string
	var currentRecipe strings.Builder

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		// Check if this is a recipe line (doesn't start with whitespace)
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && strings.Contains(line, ":") {
			// Save previous target if we have one
			if currentTarget != "" && currentRecipe.Len() > 0 {
				s.addJustScript(currentTarget, currentRecipe.String(), signals)
			}

			// Start new target
			parts := strings.SplitN(line, ":", 2)
			currentTarget = strings.TrimSpace(parts[0])
			currentRecipe.Reset()
		} else if currentTarget != "" && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) {
			// This is part of the recipe
			if currentRecipe.Len() > 0 {
				currentRecipe.WriteString(" && ")
			}
			currentRecipe.WriteString(strings.TrimSpace(line))
		}
	}

	// Don't forget the last target
	if currentTarget != "" && currentRecipe.Len() > 0 {
		s.addJustScript(currentTarget, currentRecipe.String(), signals)
	}
}

// addJustScript adds a Justfile script to signals
func (s *ScriptExtractor) addJustScript(target, recipe string, signals *shared.SignalData) {
	script := shared.Script{
		Name:   target,
		Source: "just",
		Cmd:    "just " + target,
		Paths:  s.extractPathsFromCommand(recipe),
	}
	signals.Scripts = append(signals.Scripts, script)
	signals.ScriptRefs[target] = script.Paths
}

// extractTaskfile processes Taskfile.yml
func (s *ScriptExtractor) extractTaskfile(path string, signals *shared.SignalData) {
	// Simple YAML parsing for tasks - this is a basic implementation
	// In production, you might want to use a proper YAML parser
	file, err := os.Open(filepath.Join(s.root, path))
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	var currentTask string
	var inCmds bool
	var commands []string

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Skip comments and empty lines
		if strings.HasPrefix(trimmed, "#") || trimmed == "" {
			continue
		}

		// Check for task definition
		if strings.HasPrefix(line, "  ") && strings.HasSuffix(trimmed, ":") && !inCmds {
			// Save previous task
			if currentTask != "" && len(commands) > 0 {
				s.addTaskScript(currentTask, commands, signals)
			}

			currentTask = strings.TrimSuffix(trimmed, ":")
			commands = nil
			inCmds = false
		} else if strings.Contains(trimmed, "cmds:") {
			inCmds = true
		} else if inCmds && strings.HasPrefix(line, "      - ") {
			cmd := strings.TrimPrefix(trimmed, "- ")
			commands = append(commands, cmd)
		} else if !strings.HasPrefix(line, "    ") && currentTask != "" {
			// End of current task
			if len(commands) > 0 {
				s.addTaskScript(currentTask, commands, signals)
			}
			currentTask = ""
			commands = nil
			inCmds = false
		}
	}

	// Don't forget the last task
	if currentTask != "" && len(commands) > 0 {
		s.addTaskScript(currentTask, commands, signals)
	}
}

// addTaskScript adds a Taskfile script to signals
func (s *ScriptExtractor) addTaskScript(task string, commands []string, signals *shared.SignalData) {
	allCommands := strings.Join(commands, " && ")
	script := shared.Script{
		Name:   task,
		Source: "task",
		Cmd:    "task " + task,
		Paths:  s.extractPathsFromCommand(allCommands),
	}
	signals.Scripts = append(signals.Scripts, script)
	signals.ScriptRefs[task] = script.Paths
}

// extractProcfile processes Procfile
func (s *ScriptExtractor) extractProcfile(path string, signals *shared.SignalData) {
	file, err := os.Open(filepath.Join(s.root, path))
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		// Procfile format: process_name: command
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[0])
				cmd := strings.TrimSpace(parts[1])

				script := shared.Script{
					Name:   name,
					Source: "procfile",
					Cmd:    cmd,
					Paths:  s.extractPathsFromCommand(cmd),
				}
				signals.Scripts = append(signals.Scripts, script)
				signals.ScriptRefs[name] = script.Paths
			}
		}
	}
}

// extractShellScript processes shell scripts
func (s *ScriptExtractor) extractShellScript(path string, signals *shared.SignalData) {
	file, err := os.Open(filepath.Join(s.root, path))
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()

	var content strings.Builder
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		content.WriteString(scanner.Text())
		content.WriteString(" ")
	}

	// Extract paths from the entire script
	paths := s.extractPathsFromCommand(content.String())
	if len(paths) > 0 {
		scriptName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		signals.ScriptRefs[scriptName] = paths
	}
}

// extractPathsFromCommand extracts file paths from command strings
func (s *ScriptExtractor) extractPathsFromCommand(cmd string) []string {
	var paths []string

	// Regex patterns for common path patterns in scripts
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`[./]?(?:src|app|cmd|internal|ui|frontend|backend|lib|pkg|test|tests)/[a-zA-Z0-9_/.-]+\.[a-zA-Z0-9]+`),
		regexp.MustCompile(`[./]?[a-zA-Z0-9_-]+\.(ts|tsx|js|jsx|go|php|py|rb|rs|c|cpp|h|hpp|yaml|yml|json|toml|md|txt)`),
		regexp.MustCompile(`(?:src|app|cmd|internal|ui)/[a-zA-Z0-9_/.-]*`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllString(cmd, -1)
		for _, match := range matches {
			// Clean up the match
			cleaned := strings.Trim(match, `"'`+"`"+`()[]{}`)
			if cleaned != "" && !s.isCommonNonPath(cleaned) {
				paths = append(paths, cleaned)
			}
		}
	}

	// Remove duplicates
	seen := make(map[string]bool)
	var unique []string
	for _, path := range paths {
		if !seen[path] {
			seen[path] = true
			unique = append(unique, path)
		}
	}

	return unique
}

// isCommonNonPath checks if a string is commonly found in scripts but not a file path
func (s *ScriptExtractor) isCommonNonPath(str string) bool {
	nonPaths := map[string]bool{
		"src":   false, // Actually could be a path
		"app":   false, // Actually could be a path
		"test":  false, // Actually could be a path
		"bin":   true,
		"usr":   true,
		"etc":   true,
		"var":   true,
		"tmp":   true,
		"dev":   true,
		"run":   true,
		"build": false, // Could be a directory
		"dist":  false, // Could be a directory
		"out":   false, // Could be a directory
	}

	return nonPaths[strings.ToLower(str)]
}
