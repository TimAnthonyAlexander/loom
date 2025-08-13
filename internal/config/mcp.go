package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// MCPServerConfig defines how to start and interact with a single MCP server
type MCPServerConfig struct {
	Command    string   `json:"command"`
	Args       []string `json:"args,omitempty"`
	Env        []string `json:"env,omitempty"`         // optional KEY=VALUE entries
	Safe       bool     `json:"safe,omitempty"`        // defaults to false → requires approval
	TimeoutSec int      `json:"timeout_sec,omitempty"` // per-call timeout; defaults applied by caller
}

// ProjectMCP is the on-disk schema for <workspace>/.loom/mcp.json
type ProjectMCP struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

// LoadProjectMCP loads MCP server configuration from <workspace>/.loom/mcp.json.
// If the file is missing, it returns an empty map without error.
func LoadProjectMCP(workspace string) (map[string]MCPServerConfig, error) {
	ws := filepath.Clean(stringsTrimSpaceSafe(workspace))
	if ws == "" {
		return map[string]MCPServerConfig{}, errors.New("workspace path is empty")
	}
	path := filepath.Join(ws, ".loom", "mcp.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]MCPServerConfig{}, nil
		}
		return map[string]MCPServerConfig{}, err
	}
	if len(data) == 0 {
		return map[string]MCPServerConfig{}, nil
	}
	var cfg ProjectMCP
	if err := json.Unmarshal(data, &cfg); err != nil {
		return map[string]MCPServerConfig{}, err
	}
	if cfg.MCPServers == nil {
		return map[string]MCPServerConfig{}, nil
	}
	return cfg.MCPServers, nil
}

// stringsTrimSpaceSafe is a tiny helper to avoid importing strings directly here
func stringsTrimSpaceSafe(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
