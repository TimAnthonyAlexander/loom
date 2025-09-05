package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/loom/loom/internal/tool"
)

// normalizeAttachedFiles normalizes a list of file paths for attachment.
// Paths are normalized to forward slashes and trimmed. Empty entries are ignored.
func normalizeAttachedFiles(paths []string) []string {
	normalized := make([]string, 0, len(paths))
	for _, p := range paths {
		s := strings.TrimSpace(p)
		if s == "" {
			continue
		}
		s = strings.ReplaceAll(s, "\\", "/")
		// Remove any leading ./
		for strings.HasPrefix(s, "./") {
			s = strings.TrimPrefix(s, "./")
		}
		normalized = append(normalized, s)
	}
	return normalized
}

// convertSchemas converts tool.Schema to ToolSchema
func convertSchemas(schemas []tool.Schema) []ToolSchema {
	result := make([]ToolSchema, len(schemas))
	for i, schema := range schemas {
		result[i] = ToolSchema{
			Name:        schema.Name,
			Description: schema.Description,
			Schema:      schema.Parameters,
		}
	}
	return result
}

// loadUserMemoriesForPrompt reads ~/.loom/memories.json and returns entries for prompt injection.
func loadUserMemoriesForPrompt() []MemoryEntry {
	type mem struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	path := filepath.Join(home, ".loom", "memories.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var list []mem
	if json.Unmarshal(data, &list) == nil {
		out := make([]MemoryEntry, 0, len(list))
		for _, it := range list {
			out = append(out, MemoryEntry{ID: strings.TrimSpace(it.ID), Text: strings.TrimSpace(it.Text)})
		}
		return out
	}
	var wrapper struct {
		Memories []mem `json:"memories"`
	}
	if json.Unmarshal(data, &wrapper) == nil && wrapper.Memories != nil {
		out := make([]MemoryEntry, 0, len(wrapper.Memories))
		for _, it := range wrapper.Memories {
			out = append(out, MemoryEntry{ID: strings.TrimSpace(it.ID), Text: strings.TrimSpace(it.Text)})
		}
		return out
	}
	return nil
}
