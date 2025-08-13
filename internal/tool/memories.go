package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// MemoryItem represents a single user memory item.
type MemoryItem struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// memoriesFilePath returns the absolute path to the user's memories JSON file: ~/.loom/memories.json
func memoriesFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve user home: %w", err)
	}
	dir := filepath.Join(home, ".loom")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create .loom directory: %w", err)
	}
	return filepath.Join(dir, "memories.json"), nil
}

var memoriesMu sync.Mutex

func loadMemories() ([]MemoryItem, error) {
	path, err := memoriesFilePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []MemoryItem{}, nil
		}
		return nil, fmt.Errorf("failed to read memories: %w", err)
	}
	// Try array form first
	var list []MemoryItem
	if err := json.Unmarshal(data, &list); err == nil {
		return list, nil
	}
	// Try object form {"memories": [...]}
	var wrapper struct {
		Memories []MemoryItem `json:"memories"`
	}
	if err := json.Unmarshal(data, &wrapper); err == nil && wrapper.Memories != nil {
		return wrapper.Memories, nil
	}
	return nil, errors.New("invalid memories format")
}

func saveMemories(items []MemoryItem) error {
	path, err := memoriesFilePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize memories: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write memories: %w", err)
	}
	return nil
}

// RegisterMemories registers the `memories` tool providing add/list/update/delete actions.
func RegisterMemories(registry *Registry) error {
	return registry.Register(Definition{
		Name:        "memories",
		Description: "Manage user memories (add, list, update, delete). User-specific and persistent across projects.",
		Safe:        true,
		JSONSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"description": "Action to perform",
					"enum":        []string{"add", "list", "update", "delete"},
				},
				"id": map[string]interface{}{
					"type":        "string",
					"description": "Identifier for the memory (required for update/delete; optional for add)",
				},
				"text": map[string]interface{}{
					"type":        "string",
					"description": "Memory text (required for add/update)",
				},
			},
			"required": []string{"action"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			var args struct {
				Action string `json:"action"`
				ID     string `json:"id"`
				Text   string `json:"text"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, fmt.Errorf("failed to parse arguments: %w", err)
			}
			action := strings.ToLower(strings.TrimSpace(args.Action))
			switch action {
			case "list":
				memoriesMu.Lock()
				items, err := loadMemories()
				memoriesMu.Unlock()
				if err != nil {
					return nil, err
				}
				return map[string]interface{}{"memories": items, "count": len(items)}, nil
			case "add":
				if strings.TrimSpace(args.Text) == "" {
					return nil, errors.New("text is required for add")
				}
				id := strings.TrimSpace(args.ID)
				if id == "" {
					id = time.Now().Format("20060102-150405")
				}
				newItem := MemoryItem{ID: id, Text: args.Text}
				memoriesMu.Lock()
				items, err := loadMemories()
				if err != nil {
					memoriesMu.Unlock()
					return nil, err
				}
				// Replace if existing id
				replaced := false
				for i := range items {
					if items[i].ID == id {
						items[i] = newItem
						replaced = true
						break
					}
				}
				if !replaced {
					items = append(items, newItem)
				}
				err = saveMemories(items)
				memoriesMu.Unlock()
				if err != nil {
					return nil, err
				}
				return map[string]interface{}{"status": "ok", "memory": newItem}, nil
			case "update":
				if strings.TrimSpace(args.ID) == "" {
					return nil, errors.New("id is required for update")
				}
				if strings.TrimSpace(args.Text) == "" {
					return nil, errors.New("text is required for update")
				}
				memoriesMu.Lock()
				items, err := loadMemories()
				if err != nil {
					memoriesMu.Unlock()
					return nil, err
				}
				found := false
				for i := range items {
					if items[i].ID == args.ID {
						items[i].Text = args.Text
						found = true
						break
					}
				}
				if !found {
					memoriesMu.Unlock()
					return nil, fmt.Errorf("memory with id %q not found", args.ID)
				}
				err = saveMemories(items)
				memoriesMu.Unlock()
				if err != nil {
					return nil, err
				}
				return map[string]interface{}{"status": "ok", "id": args.ID}, nil
			case "delete":
				if strings.TrimSpace(args.ID) == "" {
					return nil, errors.New("id is required for delete")
				}
				memoriesMu.Lock()
				items, err := loadMemories()
				if err != nil {
					memoriesMu.Unlock()
					return nil, err
				}
				out := make([]MemoryItem, 0, len(items))
				removed := false
				for _, it := range items {
					if it.ID == args.ID {
						removed = true
						continue
					}
					out = append(out, it)
				}
				if !removed {
					memoriesMu.Unlock()
					return nil, fmt.Errorf("memory with id %q not found", args.ID)
				}
				err = saveMemories(out)
				memoriesMu.Unlock()
				if err != nil {
					return nil, err
				}
				return map[string]interface{}{"status": "ok", "id": args.ID}, nil
			default:
				return nil, fmt.Errorf("unsupported action: %s", args.Action)
			}
		},
	})
}
