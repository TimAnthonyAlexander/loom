package tui

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"loom/indexer"
	"loom/llm"
	"loom/paths"
)

// loadOrGenerateProjectSummary loads a cached project summary if it exists. If not, it
// asks the configured LLM to write a concise description of the code-base and caches
// the result under <workspace>/.loom/project_summary.txt.  The summary is generated
// only once per workspace and reused on subsequent Loom launches.
func loadOrGenerateProjectSummary(workspacePath string, idx *indexer.Index, adapter llm.LLMAdapter) (string, error) {
	// Resolve the per-project Loom directory in the user's home (~/.loom/projects/<hash>)
	pp, err := paths.NewProjectPaths(workspacePath)
	if err != nil {
		return "", err
	}

	summaryDir := pp.ProjectDir()
	summaryPath := filepath.Join(summaryDir, "project_summary.txt")

	// If we already have a summary on disk – just return it.
	if data, err := os.ReadFile(summaryPath); err == nil {
		return string(data), nil
	}

	// If no LLM is available we can still return a very crude summary so the
	// welcome message is not empty.
	basicSummary := func() string {
		stats := idx.GetStats()
		return fmt.Sprintf("This project contains %d files (≈%.1f MB). Primary languages: %s.",
			stats.TotalFiles,
			float64(stats.TotalSize)/1024/1024,
			topLanguages(stats.LanguagePercent, 3))
	}

	if adapter == nil || !adapter.IsAvailable() {
		return basicSummary(), nil
	}

	fmt.Println("Generating project summary...")

	// Collect context for the LLM – README, package meta files, simple file tree…
	readmeSnippet := readFirstFile(workspacePath, []string{"README.md", "README.MD", "Readme.md", "readme.md", "README.txt"}, 200)
	packageInfo := readFirstFile(workspacePath, []string{"go.mod", "package.json", "composer.json", "pyproject.toml", "Cargo.toml"}, 100)

	// Top-level structure (dir1/, dir2/, main.go …)
	entries, _ := os.ReadDir(workspacePath)
	var structureParts []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") { // skip hidden & VCS dirs
			continue
		}
		if e.IsDir() {
			structureParts = append(structureParts, name+"/")
		} else {
			structureParts = append(structureParts, name)
		}
	}
	topStructure := strings.Join(structureParts, ", ")

	// Language stats
	stats := idx.GetStats()
	langSummary := topLanguages(stats.LanguagePercent, 5)

	// Prompt engineering – ask for 2-3 sentences
	systemPrompt := llm.Message{
		Role:      "system",
		Content:   "You are an expert technical writer who creates concise project overviews.",
		Timestamp: time.Now(),
	}

	userPrompt := llm.Message{
		Role: "user",
		Content: fmt.Sprintf(`Write a short (2-3 sentences) high-level summary of a software project for new contributors.
Focus on the purpose of the project and the main technologies used.

Language stats: %s
Top-level structure: %s

README excerpt:
"""
%s
"""

Package metadata:
"""
%s
"""`, langSummary, topStructure, readmeSnippet, packageInfo),
		Timestamp: time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	reply, err := adapter.Send(ctx, []llm.Message{systemPrompt, userPrompt})
	if err != nil {
		// Fall back to basic summary if the LLM request fails
		return basicSummary(), nil
	}

	summary := strings.TrimSpace(reply.Content)

	// Cache the summary for future runs (ignore errors – not critical)
	_ = os.MkdirAll(summaryDir, fs.ModePerm)
	_ = os.WriteFile(summaryPath, []byte(summary), 0644)

	return summary, nil
}

// readFirstFile tries the provided candidate filenames (relative to workspacePath)
// and, if one exists, returns the first maxLines lines joined by newlines.
func readFirstFile(workspacePath string, candidates []string, maxLines int) string {
	for _, name := range candidates {
		path := filepath.Join(workspacePath, name)
		data, err := os.ReadFile(path)
		if err == nil {
			lines := strings.Split(string(data), "\n")
			if len(lines) > maxLines {
				lines = lines[:maxLines]
			}
			return strings.Join(lines, "\n")
		}
	}
	return ""
}

// topLanguages returns a comma-separated string of the n most prevalent languages
// with their percentage (e.g. "Go 75.0%, JavaScript 20.0%, HTML 5.0%")
func topLanguages(langPercent map[string]float64, n int) string {
	type lp struct {
		name    string
		percent float64
	}
	var list []lp
	for k, v := range langPercent {
		list = append(list, lp{k, v})
	}
	// sort descending by percent
	sort.Slice(list, func(i, j int) bool { return list[i].percent > list[j].percent })
	if len(list) > n {
		list = list[:n]
	}
	parts := make([]string, len(list))
	for i, l := range list {
		parts[i] = fmt.Sprintf("%s %.1f%%", l.name, l.percent)
	}
	return strings.Join(parts, ", ")
}
