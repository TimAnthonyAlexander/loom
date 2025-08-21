package write

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/loom/loom/internal/profiler/shared"
)

// RulesWriter writes the rules.md file
type RulesWriter struct {
	root string
}

// NewRulesWriter creates a new rules writer
func NewRulesWriter(root string) *RulesWriter {
	return &RulesWriter{root: root}
}

// Write writes the project rules to .loom/rules.md
func (w *RulesWriter) Write(profile *shared.Profile) error {
	// Ensure .loom directory exists
	loomDir := filepath.Join(w.root, ".loom")
	if err := os.MkdirAll(loomDir, 0755); err != nil {
		return err
	}

	content := w.generateRulesContent(profile)

	// Write to temporary file first for atomicity
	tempPath := filepath.Join(loomDir, "rules.md.tmp")
	if err := os.WriteFile(tempPath, []byte(content), 0644); err != nil {
		return err
	}

	// Atomic rename
	finalPath := filepath.Join(loomDir, "rules.md")
	return os.Rename(tempPath, finalPath)
}

// generateRulesContent generates the content for rules.md
func (w *RulesWriter) generateRulesContent(profile *shared.Profile) string {
	var content strings.Builder

	content.WriteString("# Project Rules (detected)\n\n")

	// Languages section
	if len(profile.Languages) > 0 {
		content.WriteString("**Languages:** ")
		titleCaser := cases.Title(language.English)
		for i, lang := range profile.Languages {
			if i > 0 {
				content.WriteString(", ")
			}
			content.WriteString(titleCaser.String(lang))
		}
		content.WriteString("\n\n")
	}

	// Entrypoints section
	w.writeEntrypoints(&content, profile.Entrypoints)

	// Commands section
	w.writeCommands(&content, profile.Scripts)

	// Tools section
	w.writeTools(&content, profile.Configs)

	// Code generation section
	w.writeCodegen(&content, profile.Codegen)

	// CI/CD section
	w.writeCICD(&content, profile.CI)

	// Generated files section
	w.writeGeneratedFiles(&content)

	// Important files section
	w.writeImportantFiles(&content, profile.ImportantFiles)

	content.WriteString("\n---\n")
	content.WriteString("*This file was generated automatically by Loom Project Profiler*\n")

	return content.String()
}

// writeEntrypoints writes the entrypoints section
func (w *RulesWriter) writeEntrypoints(content *strings.Builder, entrypoints []shared.EntryPoint) {
	if len(entrypoints) == 0 {
		return
	}

	content.WriteString("## Entrypoints\n\n")

	// Group by kind
	byKind := make(map[string][]shared.EntryPoint)
	for _, ep := range entrypoints {
		byKind[ep.Kind] = append(byKind[ep.Kind], ep)
	}

	titleCaser := cases.Title(language.English)
	for kind, eps := range byKind {
		if len(eps) == 1 {
			content.WriteString("**")
			content.WriteString(titleCaser.String(kind))
			content.WriteString(":** `")
			content.WriteString(eps[0].Path)
			content.WriteString("`")
			if len(eps[0].Hints) > 0 {
				content.WriteString(" (")
				content.WriteString(strings.Join(eps[0].Hints, ", "))
				content.WriteString(")")
			}
			content.WriteString("\n\n")
		} else {
			content.WriteString("**")
			content.WriteString(titleCaser.String(kind))
			content.WriteString(":**\n")
			for _, ep := range eps {
				content.WriteString("- `")
				content.WriteString(ep.Path)
				content.WriteString("`")
				if len(ep.Hints) > 0 {
					content.WriteString(" (")
					content.WriteString(strings.Join(ep.Hints, ", "))
					content.WriteString(")")
				}
				content.WriteString("\n")
			}
			content.WriteString("\n")
		}
	}
}

// writeCommands writes the commands section
func (w *RulesWriter) writeCommands(content *strings.Builder, scripts []shared.Script) {
	if len(scripts) == 0 {
		return
	}

	content.WriteString("## Commands\n\n")

	// Group by type and find common ones
	devCommands := w.findCommandsByPattern(scripts, []string{"dev", "start", "serve"})
	buildCommands := w.findCommandsByPattern(scripts, []string{"build", "compile"})
	testCommands := w.findCommandsByPattern(scripts, []string{"test", "spec", "check"})
	lintCommands := w.findCommandsByPattern(scripts, []string{"lint", "format", "fmt"})

	if len(devCommands) > 0 {
		content.WriteString("**Development:** `")
		content.WriteString(devCommands[0].Cmd)
		content.WriteString("`\n\n")
	}

	if len(buildCommands) > 0 {
		content.WriteString("**Build:** `")
		content.WriteString(buildCommands[0].Cmd)
		content.WriteString("`\n\n")
	}

	if len(testCommands) > 0 {
		content.WriteString("**Test:** `")
		content.WriteString(testCommands[0].Cmd)
		content.WriteString("`\n\n")
	}

	if len(lintCommands) > 0 {
		content.WriteString("**Lint/Format:** `")
		content.WriteString(lintCommands[0].Cmd)
		content.WriteString("`\n\n")
	}

	// Show other important commands
	otherCommands := w.findOtherImportantCommands(scripts, []string{"dev", "start", "serve", "build", "compile", "test", "spec", "check", "lint", "format", "fmt"})
	if len(otherCommands) > 0 {
		content.WriteString("**Other Commands:**\n")
		for _, cmd := range otherCommands {
			content.WriteString("- `")
			content.WriteString(cmd.Cmd)
			content.WriteString("` (")
			content.WriteString(cmd.Source)
			content.WriteString(")\n")
		}
		content.WriteString("\n")
	}
}

// writeTools writes the tools section
func (w *RulesWriter) writeTools(content *strings.Builder, configs []shared.ConfigFile) {
	if len(configs) == 0 {
		return
	}

	content.WriteString("## Tools & Configuration\n\n")

	// Group by category
	linters := w.findConfigsByTools(configs, []string{"eslint", "prettier", "phpstan", "pint"})
	bundlers := w.findConfigsByTools(configs, []string{"vite", "webpack", "rollup"})
	testing := w.findConfigsByTools(configs, []string{"jest", "vitest", "phpunit", "playwright", "cypress"})
	containers := w.findConfigsByTools(configs, []string{"docker", "docker-compose"})

	if len(linters) > 0 {
		content.WriteString("**Linting/Formatting:** ")
		tools := make([]string, len(linters))
		for i, config := range linters {
			tools[i] = config.Tool
		}
		content.WriteString(strings.Join(tools, ", "))
		content.WriteString("\n\n")
	}

	if len(bundlers) > 0 {
		content.WriteString("**Build Tools:** ")
		tools := make([]string, len(bundlers))
		for i, config := range bundlers {
			tools[i] = config.Tool
		}
		content.WriteString(strings.Join(tools, ", "))
		content.WriteString("\n\n")
	}

	if len(testing) > 0 {
		content.WriteString("**Testing:** ")
		tools := make([]string, len(testing))
		for i, config := range testing {
			tools[i] = config.Tool
		}
		content.WriteString(strings.Join(tools, ", "))
		content.WriteString("\n\n")
	}

	if len(containers) > 0 {
		content.WriteString("**Containerization:** ")
		tools := make([]string, len(containers))
		for i, config := range containers {
			tools[i] = config.Tool
		}
		content.WriteString(strings.Join(tools, ", "))
		content.WriteString("\n\n")
	}
}

// writeCodegen writes the code generation section
func (w *RulesWriter) writeCodegen(content *strings.Builder, codegen []shared.CodegenSpec) {
	if len(codegen) == 0 {
		return
	}

	content.WriteString("## Code Generation\n\n")

	titleCaser := cases.Title(language.English)
	for _, spec := range codegen {
		content.WriteString("**")
		content.WriteString(titleCaser.String(spec.Tool))
		content.WriteString(":** ")
		if len(spec.Paths) == 1 {
			content.WriteString("`")
			content.WriteString(spec.Paths[0])
			content.WriteString("`")
		} else {
			fmt.Fprintf(content, "%d files", len(spec.Paths))
		}
		content.WriteString("\n\n")
	}
}

// writeCICD writes the CI/CD section
func (w *RulesWriter) writeCICD(content *strings.Builder, ci []shared.CIConfig) {
	if len(ci) == 0 {
		return
	}

	content.WriteString("## CI/CD\n\n")

	for _, config := range ci {
		content.WriteString("**")
		content.WriteString(filepath.Base(config.Path))
		content.WriteString(":** ")
		if len(config.Jobs) <= 3 {
			content.WriteString(strings.Join(config.Jobs, ", "))
		} else {
			fmt.Fprintf(content, "%d jobs", len(config.Jobs))
		}
		content.WriteString("\n\n")
	}
}

// writeGeneratedFiles writes the generated files section
func (w *RulesWriter) writeGeneratedFiles(content *strings.Builder) {
	content.WriteString("## Generated/Ignored Files\n\n")
	content.WriteString("**Do not edit:** ")

	patterns := []string{
		"node_modules/", "vendor/", "dist/", "build/", "target/",
		"*.generated.*", "*.pb.go", "*.g.dart", "*.d.ts", "*.min.*",
	}

	content.WriteString(strings.Join(patterns, ", "))
	content.WriteString("\n\n")
}

// writeImportantFiles writes the important files section
func (w *RulesWriter) writeImportantFiles(content *strings.Builder, files []shared.ImportantFile) {
	if len(files) == 0 {
		return
	}

	content.WriteString("## Key Files\n\n")

	// Show top 10 files
	maxFiles := 10
	if len(files) > maxFiles {
		files = files[:maxFiles]
	}

	for _, file := range files {
		content.WriteString("- `")
		content.WriteString(file.Path)
		content.WriteString("` (")
		content.WriteString(strings.Join(file.Reasons, ", "))
		content.WriteString(")\n")
	}
	content.WriteString("\n")
}

// Helper functions

func (w *RulesWriter) findCommandsByPattern(scripts []shared.Script, patterns []string) []shared.Script {
	var found []shared.Script
	for _, script := range scripts {
		for _, pattern := range patterns {
			if strings.Contains(strings.ToLower(script.Name), pattern) {
				found = append(found, script)
				break
			}
		}
	}
	return found
}

func (w *RulesWriter) findOtherImportantCommands(scripts []shared.Script, exclude []string) []shared.Script {
	var found []shared.Script
	for _, script := range scripts {
		isExcluded := false
		for _, pattern := range exclude {
			if strings.Contains(strings.ToLower(script.Name), pattern) {
				isExcluded = true
				break
			}
		}
		if !isExcluded && len(found) < 5 { // Limit to 5 other commands
			found = append(found, script)
		}
	}
	return found
}

func (w *RulesWriter) findConfigsByTools(configs []shared.ConfigFile, tools []string) []shared.ConfigFile {
	var found []shared.ConfigFile
	for _, config := range configs {
		for _, tool := range tools {
			if config.Tool == tool {
				found = append(found, config)
				break
			}
		}
	}
	return found
}

// Read reads existing rules if they exist
func (w *RulesWriter) Read() (string, error) {
	rulesPath := filepath.Join(w.root, ".loom", "rules.md")

	data, err := os.ReadFile(rulesPath)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// Exists checks if rules exist
func (w *RulesWriter) Exists() bool {
	rulesPath := filepath.Join(w.root, ".loom", "rules.md")
	_, err := os.Stat(rulesPath)
	return err == nil
}
