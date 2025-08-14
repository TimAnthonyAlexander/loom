package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/loom/loom/internal/config"
)

// Manager supervises multiple MCP clients and exposes helper methods
type Manager struct {
	mu      sync.RWMutex
	clients map[string]*Client // alias -> client
	// lastCfgHash is a stable hash of the last applied config set, used to make Start idempotent
	lastCfgHash string
}

func NewManager() *Manager {
	return &Manager{clients: make(map[string]*Client)}
}

// Start creates clients for all configured servers. Idempotent: if the config
// has not changed since the last Start, this is a no-op.
func (m *Manager) Start(cfgs map[string]config.MCPServerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Compute a stable hash of cfgs to avoid duplicate restarts on identical input
	hash := hashConfigs(cfgs)
	if hash == m.lastCfgHash {
		log.Printf("[mcp] Start: noop due to identical hash=%s (aliases=%d)", hash[:8], len(m.clients))
		return nil
	}
	// For changed config, restart only the aliases that changed; keep identical ones running
	// Build current map for comparison
	newClients := make(map[string]*Client)
	for alias, cfg := range cfgs {
		// If an existing client for alias exists and its canonicalized config matches, reuse it
		if existing, ok := m.clients[alias]; ok {
			if configsCanonicallyEqual(existing.cfg, cfg) {
				pid := -1
				if existing != nil && existing.cmd != nil && existing.cmd.Process != nil {
					pid = existing.cmd.Process.Pid
				}
				log.Printf("[mcp] Start(alias=%s): cfgHash=%s action=noop pid=%d", alias, hashConfigs(map[string]config.MCPServerConfig{"a": cfg})[:8], pid)
				newClients[alias] = existing
				continue
			}
			// Config changed: stop the old one
			log.Printf("[mcp] Start(alias=%s): restart (config changed)", alias)
			existing.Close()
		} else {
			log.Printf("[mcp] Start(alias=%s): start (new)", alias)
		}
		// Start fresh client
		// Normalize command path
		cfg.Command = canonicalizeCommandPath(cfg.Command)
		c, err := NewClient(alias, cfg)
		if err != nil {
			log.Printf("[mcp] %s: failed to start client: %v", alias, err)
			continue
		}
		log.Printf("[mcp] Start(alias=%s): started pid=%d", alias, c.cmd.Process.Pid)
		newClients[alias] = c
	}
	// Stop clients that are no longer configured
	for alias, c := range m.clients {
		if _, stillPresent := cfgs[alias]; !stillPresent {
			c.Close()
		}
	}
	m.clients = newClients
	log.Printf("[mcp] Start: new hash=%s (aliases now=%d)", hash[:8], len(newClients))
	m.lastCfgHash = hash
	return nil
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.clients {
		c.Close()
	}
	m.clients = make(map[string]*Client)
	m.lastCfgHash = ""
}

// hashConfigs computes a stable string hash for the MCP config set.
// It sorts keys and serializes minimal fields to ensure stable ordering.
func hashConfigs(cfgs map[string]config.MCPServerConfig) string {
	if len(cfgs) == 0 {
		return "empty"
	}
	aliases := make([]string, 0, len(cfgs))
	for alias := range cfgs {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	h := sha256.New()
	for _, alias := range aliases {
		cfg := cfgs[alias]
		cfg.Command = canonicalizeCommandPath(cfg.Command)
		// Normalize env order for hashing to be stable; preserve arg order
		args := append([]string(nil), cfg.Args...)
		env := append([]string(nil), cfg.Env...)
		for i := range args {
			args[i] = strings.TrimSpace(args[i])
		}
		for i := range env {
			env[i] = strings.TrimSpace(env[i])
		}
		sort.Strings(env)
		io := struct {
			Alias      string
			Command    string
			Args       []string
			Env        []string
			Safe       bool
			TimeoutSec int
		}{Alias: alias, Command: cfg.Command, Args: args, Env: env, Safe: cfg.Safe, TimeoutSec: cfg.TimeoutSec}
		b, _ := json.Marshal(io)
		_, _ = h.Write(b)
	}
	sum := h.Sum(nil)
	return fmt.Sprintf("%x", sum)
}

// configsCanonicallyEqual compares two configs after canonicalization
func configsCanonicallyEqual(a, b config.MCPServerConfig) bool {
	ac := a
	bc := b
	ac.Command = canonicalizeCommandPath(ac.Command)
	bc.Command = canonicalizeCommandPath(bc.Command)
	// Compare normalized fields by hash
	return hashConfigs(map[string]config.MCPServerConfig{"a": ac}) == hashConfigs(map[string]config.MCPServerConfig{"a": bc})
}

// canonicalizeCommandPath normalizes command paths for hashing and stability
func canonicalizeCommandPath(cmd string) string {
	trimmed := strings.TrimSpace(cmd)
	if trimmed == "" {
		return ""
	}
	if strings.ContainsRune(trimmed, filepath.Separator) {
		if abs, err := filepath.Abs(trimmed); err == nil {
			return abs
		}
	}
	// If not a path, attempt to resolve via LookPath; fall back to original
	if p, err := exec.LookPath(trimmed); err == nil {
		if abs, err := filepath.Abs(p); err == nil {
			return abs
		}
		return p
	}
	return trimmed
}

// ListTools returns discovered tool specs per server
func (m *Manager) ListTools() (map[string][]ToolSpec, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string][]ToolSpec)
	for alias, c := range m.clients {
		var tools []ToolSpec
		var err error
		// Retry a few times to allow slow-starting servers to finish initialization
		for attempt := 0; attempt < 3; attempt++ {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			tools, err = c.ListTools(ctx)
			cancel()
			if err == nil && len(tools) > 0 {
				break
			}
			if attempt < 2 {
				time.Sleep(500 * time.Millisecond)
			}
		}
		if err != nil {
			log.Printf("[mcp] %s: ListTools failed: %v", alias, err)
			continue
		}
		out[alias] = tools
	}
	return out, nil
}

// Call delegates a single tool call to a specific server
func (m *Manager) Call(server, tool string, args json.RawMessage, timeout time.Duration) (string, error) {
	m.mu.RLock()
	c := m.clients[server]
	m.mu.RUnlock()
	if c == nil {
		return "", fmt.Errorf("unknown mcp server: %s", server)
	}
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	return c.CallTool(ctx, tool, args)
}
