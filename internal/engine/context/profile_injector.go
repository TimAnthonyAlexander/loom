package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/loom/loom/internal/profiler/shared"
)

// ProfileLoader provides an interface for loading project profiles
type ProfileLoader interface {
	LoadProfile(workspaceRoot string) (*shared.Profile, error)
	LoadRules(workspaceRoot string) (string, error)
	LoadHotlist(workspaceRoot string) ([]shared.ImportantFile, error)
}

// FileSystemProfileLoader loads profiles from the filesystem
type FileSystemProfileLoader struct{}

// LoadProfile loads the project profile from .loom/project_profile.json
func (l *FileSystemProfileLoader) LoadProfile(workspaceRoot string) (*shared.Profile, error) {
	profilePath := filepath.Join(workspaceRoot, ".loom", "project_profile.json")
	data, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, err
	}

	var profile shared.Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, err
	}

	return &profile, nil
}

// LoadRules loads the project rules from .loom/rules.md
func (l *FileSystemProfileLoader) LoadRules(workspaceRoot string) (string, error) {
	rulesPath := filepath.Join(workspaceRoot, ".loom", "rules.md")
	data, err := os.ReadFile(rulesPath)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// LoadHotlist loads the hotlist from .loom/hotlist.txt and parses the important files from the profile
func (l *FileSystemProfileLoader) LoadHotlist(workspaceRoot string) ([]shared.ImportantFile, error) {
	profile, err := l.LoadProfile(workspaceRoot)
	if err != nil {
		return nil, err
	}

	return profile.ImportantFiles, nil
}

// ProjectContextBuilder builds compact project context blocks for system prompt injection
type ProjectContextBuilder struct {
	loader ProfileLoader
}

// NewProjectContextBuilder creates a new project context builder
func NewProjectContextBuilder(loader ProfileLoader) *ProjectContextBuilder {
	return &ProjectContextBuilder{loader: loader}
}

// NewFileSystemProjectContextBuilder creates a builder with filesystem loader
func NewFileSystemProjectContextBuilder() *ProjectContextBuilder {
	return NewProjectContextBuilder(&FileSystemProfileLoader{})
}

// BuildProjectContextBlock creates a compact, deterministic project context block
func (b *ProjectContextBuilder) BuildProjectContextBlock(workspaceRoot string) (string, error) {
	profile, err := b.loader.LoadProfile(workspaceRoot)
	if err != nil {
		return "", err
	}

	var ctx strings.Builder
	ctx.WriteString("[project-profile]\n")

	// Languages
	if len(profile.Languages) > 0 {
		ctx.WriteString(fmt.Sprintf("languages: %s\n", strings.Join(profile.Languages, ", ")))
	}

	// Entrypoints
	if len(profile.Entrypoints) > 0 {
		entrypoints := make([]string, 0, len(profile.Entrypoints))
		for _, ep := range profile.Entrypoints {
			desc := ep.Path
			if ep.Kind != "" {
				desc = fmt.Sprintf("%s (%s)", ep.Path, ep.Kind)
			}
			entrypoints = append(entrypoints, desc)
		}
		ctx.WriteString(fmt.Sprintf("entrypoints: %s\n", strings.Join(entrypoints, "; ")))
	}

	// Scripts/Commands
	if len(profile.Scripts) > 0 {
		commands := make([]string, 0, len(profile.Scripts))
		for _, script := range profile.Scripts {
			if script.Name != "" && script.Cmd != "" {
				commands = append(commands, fmt.Sprintf("%s=%s", script.Name, script.Cmd))
			}
		}
		if len(commands) > 0 {
			ctx.WriteString(fmt.Sprintf("commands: %s\n", strings.Join(commands, "; ")))
		}
	}

	// Generated/Protected paths
	generated := b.buildGeneratedPaths(profile)
	if generated != "" {
		ctx.WriteString(fmt.Sprintf("generated: %s\n", generated))
	}

	// Top files (first 10)
	if len(profile.ImportantFiles) > 0 {
		ctx.WriteString("top-files:\n")
		count := len(profile.ImportantFiles)
		if count > 10 {
			count = 10
		}
		for i := 0; i < count; i++ {
			file := profile.ImportantFiles[i]
			ctx.WriteString(fmt.Sprintf("- %s\n", file.Path))
		}
	}

	// Confidence
	confidence := b.calculateConfidence(profile)
	ctx.WriteString(fmt.Sprintf("confidence: %.2f\n", confidence))

	// Policy
	ctx.WriteString("policy: prefer files with higher importance scores; do not modify generated paths; if uncertain, ask for contract.\n")
	ctx.WriteString("[/project-profile]\n")

	return ctx.String(), nil
}

// buildGeneratedPaths builds a string of generated/protected paths
func (b *ProjectContextBuilder) buildGeneratedPaths(profile *shared.Profile) string {
	paths := make(map[string]bool)

	// From codegen specs
	for _, codegen := range profile.Codegen {
		for _, path := range codegen.Paths {
			paths[path] = true
		}
	}

	// Common generated patterns
	commonGenerated := []string{
		"node_modules/", "vendor/", "dist/", "build/",
		"*.generated.*", "*.pb.go", "*.d.ts", ".next/",
		"coverage/", ".vite/", "target/",
	}

	for _, pattern := range commonGenerated {
		paths[pattern] = true
	}

	// Convert to sorted slice
	result := make([]string, 0, len(paths))
	for path := range paths {
		result = append(result, path)
	}
	sort.Strings(result)

	return strings.Join(result, ", ")
}

// calculateConfidence estimates overall profile confidence
func (b *ProjectContextBuilder) calculateConfidence(profile *shared.Profile) float64 {
	if len(profile.ImportantFiles) == 0 {
		return 0.0
	}

	// Calculate average confidence from important files
	totalConfidence := 0.0
	count := 0

	for _, file := range profile.ImportantFiles {
		if file.Confidence > 0 {
			totalConfidence += file.Confidence
			count++
		}
	}

	if count == 0 {
		return 0.5 // Default moderate confidence
	}

	return totalConfidence / float64(count)
}

// BuildRulesBlock creates a compact rules block for system prompt injection
func (b *ProjectContextBuilder) BuildRulesBlock(workspaceRoot string, maxChars int) (string, error) {
	rules, err := b.loader.LoadRules(workspaceRoot)
	if err != nil {
		return "", err
	}

	if len(rules) <= maxChars {
		return rules, nil
	}

	// Truncate and add ellipsis
	truncated := rules[:maxChars-10] // Leave room for ellipsis message
	return truncated + "\n\n[...truncated, use get_project_profile tool for full rules]", nil
}
