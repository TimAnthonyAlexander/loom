package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/loom/loom/internal/profiler"
	"github.com/loom/loom/internal/profiler/shared"
)

// GetProjectProfileArgs represents arguments for the get_project_profile tool
type GetProjectProfileArgs struct {
	Section string `json:"section,omitempty"` // "summary", "important_files", "scripts", "configs", "rules", "components"
	TopN    int    `json:"top_n,omitempty"`   // Number of items to return (for important_files)
}

// GetHotlistArgs represents arguments for the get_hotlist tool
type GetHotlistArgs struct {
	TopN int `json:"top_n,omitempty"` // Number of files to return
}

// ExplainFileImportanceArgs represents arguments for the explain_file_importance tool
type ExplainFileImportanceArgs struct {
	Path string `json:"path"` // File path to explain
}

// ProjectProfileTool provides read-only access to project profile data
type ProjectProfileTool struct {
	contextBuilder *profiler.ProjectContextBuilder
	workspaceRoot  string
}

// NewProjectProfileTool creates a new project profile tool
func NewProjectProfileTool(workspaceRoot string) *ProjectProfileTool {
	return &ProjectProfileTool{
		contextBuilder: profiler.NewFileSystemProjectContextBuilder(),
		workspaceRoot:  workspaceRoot,
	}
}

// GetProjectProfile returns project profile information based on the requested section
func (t *ProjectProfileTool) GetProjectProfile(ctx context.Context, raw json.RawMessage) (interface{}, error) {
	var args GetProjectProfileArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("failed to parse args: %w", err)
	}

	loader := &profiler.FileSystemProfileLoader{}
	profile, err := loader.LoadProfile(t.workspaceRoot)
	if err != nil {
		return map[string]interface{}{
			"mode":  "none",
			"error": "No project profile available. Run profiling first.",
		}, nil
	}

	switch strings.ToLower(args.Section) {
	case "summary":
		return t.buildSummarySection(profile), nil
	case "important_files":
		return t.buildImportantFilesSection(profile, args.TopN), nil
	case "scripts":
		return t.buildScriptsSection(profile), nil
	case "configs":
		return t.buildConfigsSection(profile), nil
	case "rules":
		return t.buildRulesSection(), nil
	case "components":
		return t.buildComponentsSection(profile), nil
	default:
		return t.buildFullProfile(profile), nil
	}
}

// GetHotlist returns the hotlist of important files
func (t *ProjectProfileTool) GetHotlist(ctx context.Context, raw json.RawMessage) (interface{}, error) {
	var args GetHotlistArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("failed to parse args: %w", err)
	}

	loader := &profiler.FileSystemProfileLoader{}
	hotlist, err := loader.LoadHotlist(t.workspaceRoot)
	if err != nil {
		return map[string]interface{}{
			"mode":  "none",
			"error": "No hotlist available. Run profiling first.",
		}, nil
	}

	topN := args.TopN
	if topN <= 0 || topN > len(hotlist) {
		topN = len(hotlist)
	}

	result := make([]map[string]interface{}, topN)
	for i := 0; i < topN; i++ {
		file := hotlist[i]
		result[i] = map[string]interface{}{
			"path":         file.Path,
			"score":        file.Score,
			"reasons":      file.Reasons,
			"components":   file.Components,
			"confidence":   file.Confidence,
			"is_generated": file.IsGenerated,
		}
	}

	return map[string]interface{}{
		"files": result,
		"total": len(hotlist),
	}, nil
}

// ExplainFileImportance explains why a specific file is important
func (t *ProjectProfileTool) ExplainFileImportance(ctx context.Context, raw json.RawMessage) (interface{}, error) {
	var args ExplainFileImportanceArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("failed to parse args: %w", err)
	}

	if args.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	loader := &profiler.FileSystemProfileLoader{}
	profile, err := loader.LoadProfile(t.workspaceRoot)
	if err != nil {
		return map[string]interface{}{
			"error": "No project profile available. Run profiling first.",
		}, nil
	}

	// Find the file in important files
	for _, file := range profile.ImportantFiles {
		if file.Path == args.Path {
			return map[string]interface{}{
				"path":         file.Path,
				"score":        file.Score,
				"reasons":      file.Reasons,
				"components":   file.Components,
				"penalties":    file.Penalties,
				"confidence":   file.Confidence,
				"is_generated": file.IsGenerated,
				"explanation":  t.buildExplanation(file),
			}, nil
		}
	}

	return map[string]interface{}{
		"path":        args.Path,
		"score":       0.0,
		"explanation": "File not found in project profile or has zero importance score.",
	}, nil
}

// buildSummarySection creates a summary of the project
func (t *ProjectProfileTool) buildSummarySection(profile *shared.Profile) map[string]interface{} {
	return map[string]interface{}{
		"languages":      profile.Languages,
		"entrypoints":    profile.Entrypoints,
		"file_count":     profile.Metrics.Files,
		"confidence":     t.calculateAverageConfidence(profile),
		"created_at":     profile.CreatedAtUnix,
		"version":        profile.Version,
		"workspace_root": profile.WorkspaceRoot,
	}
}

// buildImportantFilesSection creates the important files section
func (t *ProjectProfileTool) buildImportantFilesSection(profile *shared.Profile, topN int) map[string]interface{} {
	if topN <= 0 || topN > len(profile.ImportantFiles) {
		topN = len(profile.ImportantFiles)
	}

	files := make([]map[string]interface{}, topN)
	for i := 0; i < topN; i++ {
		file := profile.ImportantFiles[i]
		files[i] = map[string]interface{}{
			"path":       file.Path,
			"score":      file.Score,
			"reasons":    file.Reasons,
			"confidence": file.Confidence,
		}
	}

	return map[string]interface{}{
		"files": files,
		"total": len(profile.ImportantFiles),
	}
}

// buildScriptsSection creates the scripts section
func (t *ProjectProfileTool) buildScriptsSection(profile *shared.Profile) map[string]interface{} {
	scripts := make([]map[string]interface{}, len(profile.Scripts))
	for i, script := range profile.Scripts {
		scripts[i] = map[string]interface{}{
			"name":   script.Name,
			"source": script.Source,
			"cmd":    script.Cmd,
			"paths":  script.Paths,
		}
	}

	return map[string]interface{}{
		"scripts": scripts,
	}
}

// buildConfigsSection creates the configs section
func (t *ProjectProfileTool) buildConfigsSection(profile *shared.Profile) map[string]interface{} {
	configs := make([]map[string]interface{}, len(profile.Configs))
	for i, config := range profile.Configs {
		configs[i] = map[string]interface{}{
			"tool": config.Tool,
			"path": config.Path,
		}
	}

	codegen := make([]map[string]interface{}, len(profile.Codegen))
	for i, cg := range profile.Codegen {
		codegen[i] = map[string]interface{}{
			"tool":  cg.Tool,
			"paths": cg.Paths,
		}
	}

	return map[string]interface{}{
		"configs": configs,
		"codegen": codegen,
	}
}

// buildRulesSection loads and returns the rules
func (t *ProjectProfileTool) buildRulesSection() map[string]interface{} {
	loader := &profiler.FileSystemProfileLoader{}
	rules, err := loader.LoadRules(t.workspaceRoot)
	if err != nil {
		return map[string]interface{}{
			"error": "No rules file available",
		}
	}

	return map[string]interface{}{
		"rules": rules,
	}
}

// buildComponentsSection creates the components section
func (t *ProjectProfileTool) buildComponentsSection(profile *shared.Profile) map[string]interface{} {
	return map[string]interface{}{
		"routes_services": profile.RoutesServices,
		"ci":              profile.CI,
		"heuristics":      profile.Heuristics,
		"git_stats":       profile.GitStats,
	}
}

// buildFullProfile returns the complete profile
func (t *ProjectProfileTool) buildFullProfile(profile *shared.Profile) map[string]interface{} {
	return map[string]interface{}{
		"summary":         t.buildSummarySection(profile),
		"important_files": t.buildImportantFilesSection(profile, 0),
		"scripts":         t.buildScriptsSection(profile),
		"configs":         t.buildConfigsSection(profile),
		"components":      t.buildComponentsSection(profile),
	}
}

// buildExplanation creates a human-readable explanation of file importance
func (t *ProjectProfileTool) buildExplanation(file shared.ImportantFile) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("Score: %.3f (confidence: %.2f)", file.Score, file.Confidence))

	if len(file.Reasons) > 0 {
		parts = append(parts, "Reasons: "+strings.Join(file.Reasons, ", "))
	}

	if len(file.Components) > 0 {
		var componentParts []string
		for name, value := range file.Components {
			componentParts = append(componentParts, fmt.Sprintf("%s: %.3f", name, value))
		}
		sort.Strings(componentParts)
		parts = append(parts, "Components: "+strings.Join(componentParts, ", "))
	}

	if len(file.Penalties) > 0 {
		var penaltyParts []string
		for name, value := range file.Penalties {
			penaltyParts = append(penaltyParts, fmt.Sprintf("%s: %.3f", name, value))
		}
		sort.Strings(penaltyParts)
		parts = append(parts, "Penalties: "+strings.Join(penaltyParts, ", "))
	}

	if file.IsGenerated {
		parts = append(parts, "WARNING: This file appears to be generated")
	}

	return strings.Join(parts, ". ")
}

// calculateAverageConfidence calculates the average confidence across all important files
func (t *ProjectProfileTool) calculateAverageConfidence(profile *shared.Profile) float64 {
	if len(profile.ImportantFiles) == 0 {
		return 0.0
	}

	total := 0.0
	count := 0
	for _, file := range profile.ImportantFiles {
		if file.Confidence > 0 {
			total += file.Confidence
			count++
		}
	}

	if count == 0 {
		return 0.5
	}

	return total / float64(count)
}

// RegisterProjectProfileTools registers all project profile tools with the registry
func RegisterProjectProfileTools(registry *Registry, workspaceRoot string) error {
	tool := NewProjectProfileTool(workspaceRoot)

	// get_project_profile tool
	err := registry.Register(Definition{
		Name:        "get_project_profile",
		Description: "Get project profile information. Use 'section' to get specific parts: 'summary', 'important_files', 'scripts', 'configs', 'rules', 'components'. Use 'top_n' with 'important_files' to limit results.",
		JSONSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"section": map[string]interface{}{
					"type":        "string",
					"description": "Specific section to retrieve: 'summary', 'important_files', 'scripts', 'configs', 'rules', 'components'",
					"enum":        []string{"summary", "important_files", "scripts", "configs", "rules", "components"},
				},
				"top_n": map[string]interface{}{
					"type":        "integer",
					"description": "Number of items to return (for important_files section)",
					"minimum":     1,
					"maximum":     100,
				},
			},
		},
		Safe:    true,
		Handler: tool.GetProjectProfile,
	})
	if err != nil {
		return err
	}

	// get_hotlist tool
	err = registry.Register(Definition{
		Name:        "get_hotlist",
		Description: "Get the hotlist of most important files in the project with their importance scores and breakdown.",
		JSONSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"top_n": map[string]interface{}{
					"type":        "integer",
					"description": "Number of files to return (default: all)",
					"minimum":     1,
					"maximum":     100,
				},
			},
		},
		Safe:    true,
		Handler: tool.GetHotlist,
	})
	if err != nil {
		return err
	}

	// explain_file_importance tool
	err = registry.Register(Definition{
		Name:        "explain_file_importance",
		Description: "Explain why a specific file has its importance score, including component breakdown and reasoning.",
		JSONSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File path to explain",
				},
			},
			"required": []string{"path"},
		},
		Safe:    true,
		Handler: tool.ExplainFileImportance,
	})
	if err != nil {
		return err
	}

	return nil
}
