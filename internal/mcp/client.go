package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/loom/loom/internal/config"
)

// ToolSpec represents a single MCP tool as reported by the server
type ToolSpec struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// Client manages a single MCP server process speaking JSON-RPC over stdio
type Client struct {
	alias  string
	cfg    config.MCPServerConfig
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	mu     sync.Mutex
	reqID  atomic.Int64
	inited bool
	// initCh is non-nil when an initialization handshake is in-flight.
	// Other callers of EnsureInitialized will wait on this channel.
	initCh chan error
	// cfgHash captures the canonical config hash at creation time
	cfgHash string
	// Waiting request channels keyed by id, guarded by waitMu
	waitMu   sync.Mutex
	waiters  map[int64]chan any
	readOnce sync.Once
}

func NewClient(alias string, cfg config.MCPServerConfig) (*Client, error) {
	if strings.TrimSpace(cfg.Command) == "" {
		return nil, fmt.Errorf("mcp server %s: command is empty", alias)
	}
	cmd := exec.Command(cfg.Command, cfg.Args...)
	if len(cfg.Env) > 0 {
		cmd.Env = append(os.Environ(), cfg.Env...)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	// Surface MCP server stderr to our dev console for debugging
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	c := &Client{
		alias:   alias,
		cfg:     cfg,
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewReader(stdoutPipe),
		waiters: make(map[int64]chan any),
	}
	// Start a single read loop to dispatch all responses and notifications
	c.readOnce.Do(func() { go c.readLoop() })
	// Record canonical hash (computed in manager)
	// Note: set by manager after construction when hashing helpers are available
	// Proactively initialize with a generous timeout in the background
	go func() {
		// Long init window to tolerate cold starts
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		if err := c.EnsureInitialized(ctx); err != nil {
			log.Printf("[mcp] %s: initialization failed: %v", alias, err)
		} else {
			log.Printf("[mcp] %s: initialized", alias)
		}
	}()
	return c, nil
}

func (c *Client) Close() {
	_ = c.stdin.Close()
	_ = c.cmd.Process.Kill()
	_, _ = c.cmd.Process.Wait()
}

// Initialize sends a minimal initialize request per MCP JSON-RPC
func (c *Client) Initialize(ctx context.Context) error {
	// Single-shot latest version; only downgrade if explicitly unsupported
	latest := "2025-06-18"
	log.Printf("[mcp] %s: sending initialize with protocol %s", c.alias, latest)
	if err := c.initializeWithVersion(ctx, latest); err != nil {
		errLower := strings.ToLower(err.Error())
		if (strings.Contains(errLower, "unsupported") && strings.Contains(errLower, "protocol")) || strings.Contains(errLower, "version not supported") {
			fallback := "2025-03-26"
			log.Printf("[mcp] %s: server does not support %s; retrying initialize with %s", c.alias, latest, fallback)
			if err := c.initializeWithVersion(ctx, fallback); err != nil {
				errLower2 := strings.ToLower(err.Error())
				if (strings.Contains(errLower2, "unsupported") && strings.Contains(errLower2, "protocol")) || strings.Contains(errLower2, "version not supported") {
					oldest := "2024-11-05"
					log.Printf("[mcp] %s: server does not support %s; retrying initialize with %s", c.alias, fallback, oldest)
					if err := c.initializeWithVersion(ctx, oldest); err != nil {
						return err
					}
				} else {
					return err
				}
			}
		} else {
			return err
		}
	}
	c.mu.Lock()
	c.inited = true
	c.mu.Unlock()
	// Send initialized notification as per spec
	_ = c.notify("notifications/initialized", nil)
	log.Printf("[mcp] %s: initialized (protocol negotiated)", c.alias)
	return nil
}

func (c *Client) initializeWithVersion(ctx context.Context, version string) error {
	params := map[string]any{
		"protocolVersion": version,
		"clientInfo":      map[string]any{"name": "loom", "version": "0.1"},
		// Advertise minimal client capabilities per spec
		"capabilities": map[string]any{
			"roots":    map[string]any{},
			"sampling": map[string]any{},
			// Explicitly advertise tool support so servers expose tool APIs
			"tools": map[string]any{},
			// Hint that we support stdio framed transport
			"transport": map[string]any{"stdio": map[string]any{}},
		},
	}
	res, err := c.request(ctx, "initialize", params)
	if err == nil {
		log.Printf("[mcp] %s: initialize result received", c.alias)
		_ = res
	}
	return err
}

// ListTools queries the server for available tools
func (c *Client) ListTools(ctx context.Context) ([]ToolSpec, error) {
	if err := c.EnsureInitialized(ctx); err != nil {
		return nil, err
	}
	res, err := c.request(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	// Expected shape: { tools: [ { name, description, inputSchema|input_schema } ] }
	var out []ToolSpec
	if m, ok := res.(map[string]any); ok {
		if arr, ok := m["tools"].([]any); ok {
			for _, it := range arr {
				if tm, ok := it.(map[string]any); ok {
					name, _ := tm["name"].(string)
					if name == "" {
						continue
					}
					desc, _ := tm["description"].(string)
					var schema map[string]any
					if s, ok := tm["inputSchema"].(map[string]any); ok {
						schema = s
					}
					if s, ok := tm["input_schema"].(map[string]any); ok {
						schema = s
					}
					out = append(out, ToolSpec{Name: name, Description: desc, InputSchema: schema})
				}
			}
		}
	}
	return out, nil
}

// notify sends a JSON-RPC notification (no id)
func (c *Client) notify(method string, params any) error {
	n := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		n["params"] = params
	}
	payload, err := json.Marshal(n)
	if err != nil {
		return err
	}
	// MCP stdio: newline-delimited JSON (no headers)
	if _, err := c.stdin.Write(append(payload, '\n')); err != nil {
		return err
	}
	return nil
}

// EnsureInitialized retries initialize until success or context timeout
func (c *Client) EnsureInitialized(ctx context.Context) error {
	for {
		c.mu.Lock()
		if c.inited {
			c.mu.Unlock()
			return nil
		}
		if c.initCh == nil {
			// Start a background initializer with a generous timeout independent of caller
			ch := make(chan error, 1)
			c.initCh = ch
			c.mu.Unlock()
			go func() {
				initCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
				defer cancel()
				err := c.Initialize(initCtx)
				// Mark state and notify waiters
				c.mu.Lock()
				if err == nil {
					c.inited = true
				}
				c.initCh = nil
				c.mu.Unlock()
				ch <- err
				close(ch)
			}()
		} else {
			ch := c.initCh
			c.mu.Unlock()
			// Wait for either completion or caller timeout; do not cancel the initializer
			select {
			case <-ctx.Done():
				return ctx.Err()
			case err := <-ch:
				if err == nil {
					return nil
				}
				// On failure, loop to attempt starting a new initializer if caller still alive
			}
		}
		// Brief delay to avoid tight loop on repeated failures
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}
}

// CallTool invokes a tool on the server with raw JSON arguments
func (c *Client) CallTool(ctx context.Context, toolName string, args json.RawMessage) (string, error) {
	// If args is empty, send an empty object
	var params map[string]any
	if len(args) == 0 {
		params = map[string]any{"name": toolName, "arguments": map[string]any{}}
	} else {
		var anyArgs any
		if err := json.Unmarshal(args, &anyArgs); err != nil {
			// pass through as raw string if not an object
			params = map[string]any{"name": toolName, "arguments": string(args)}
		} else {
			params = map[string]any{"name": toolName, "arguments": anyArgs}
		}
	}
	res, err := c.request(ctx, "tools/call", params)
	if err != nil {
		return "", err
	}
	// Return result as pretty JSON for the model
	b, _ := json.MarshalIndent(res, "", "  ")
	return string(b), nil
}

// request sends a JSON-RPC request and waits for a response
func (c *Client) request(ctx context.Context, method string, params any) (any, error) {
	c.mu.Lock()
	id := c.reqID.Add(1)
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	// Write newline-delimited JSON (no headers)
	if _, err := c.stdin.Write(append(payload, '\n')); err != nil {
		c.mu.Unlock()
		return nil, err
	}
	if method == "initialize" || method == "tools/list" {
		log.Printf("[mcp] %s: sent %s (%d bytes + newline)", c.alias, method, len(payload))
	}
	// Register waiter and release the client lock before blocking
	ch := make(chan any, 1)
	c.waitMu.Lock()
	c.waiters[id] = ch
	c.waitMu.Unlock()
	c.mu.Unlock()
	select {
	case <-ctx.Done():
		// Cleanup waiter if still present
		c.waitMu.Lock()
		delete(c.waiters, id)
		c.waitMu.Unlock()
		return nil, ctx.Err()
	case v := <-ch:
		switch resp := v.(type) {
		case error:
			return nil, resp
		default:
			return resp, nil
		}
	}
}

// readLoop runs once per client and dispatches responses/notifications
func (c *Client) readLoop() {
	reader := c.stdout
	for {
		// Primary: newline-delimited JSON
		lineBytes, err := reader.ReadBytes('\n')
		if err != nil {
			c.waitMu.Lock()
			for id, ch := range c.waiters {
				ch <- fmt.Errorf("read error: %v", err)
				close(ch)
				delete(c.waiters, id)
			}
			c.waitMu.Unlock()
			return
		}
		line := strings.TrimRight(string(lineBytes), "\r\n")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "{") {
			var resp map[string]any
			d := json.NewDecoder(strings.NewReader(trimmed))
			d.UseNumber()
			if err := d.Decode(&resp); err == nil {
				c.dispatch(resp)
				continue
			}
		}
		// Fallback: framed Content-Length
		if strings.HasPrefix(strings.ToLower(trimmed), "content-length:") {
			headers := map[string]string{}
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 {
				headers[strings.ToLower(strings.TrimSpace(parts[0]))] = strings.TrimSpace(parts[1])
			}
			// Read header lines until blank
			for {
				hb, err := reader.ReadBytes('\n')
				if err != nil {
					c.waitMu.Lock()
					for id, ch := range c.waiters {
						ch <- fmt.Errorf("read header error: %v", err)
						close(ch)
						delete(c.waiters, id)
					}
					c.waitMu.Unlock()
					return
				}
				hl := strings.TrimRight(string(hb), "\r\n")
				if hl == "" {
					break
				}
				kv := strings.SplitN(hl, ":", 2)
				if len(kv) == 2 {
					headers[strings.ToLower(strings.TrimSpace(kv[0]))] = strings.TrimSpace(kv[1])
				}
			}
			clStr := headers["content-length"]
			n, err := strconv.Atoi(strings.TrimSpace(clStr))
			if err != nil || n < 0 {
				continue
			}
			body := make([]byte, n)
			if _, err := io.ReadFull(reader, body); err != nil {
				c.waitMu.Lock()
				for id, ch := range c.waiters {
					ch <- fmt.Errorf("read body error: %v", err)
					close(ch)
					delete(c.waiters, id)
				}
				c.waitMu.Unlock()
				return
			}
			var resp map[string]any
			d := json.NewDecoder(bytes.NewReader(body))
			d.UseNumber()
			if err := d.Decode(&resp); err == nil {
				c.dispatch(resp)
				continue
			}
		}
	}
}

func (c *Client) dispatch(resp map[string]any) {
	if v, ok := resp["id"]; ok && v != nil {
		// Response to a request
		var idNum int64
		switch t := v.(type) {
		case json.Number:
			if n, err := t.Int64(); err == nil {
				idNum = n
			}
		case float64:
			idNum = int64(t)
		case int64:
			idNum = t
		}
		if idNum != 0 {
			c.waitMu.Lock()
			ch := c.waiters[idNum]
			delete(c.waiters, idNum)
			c.waitMu.Unlock()
			if ch != nil {
				if errObj, ok := resp["error"].(map[string]any); ok && errObj != nil {
					msg, _ := errObj["message"].(string)
					if msg == "" {
						msg = "error"
					}
					ch <- fmt.Errorf("mcp error: %s", msg)
				} else {
					ch <- resp["result"]
				}
				close(ch)
			}
		}
		return
	}
	// Notification; ignore
}
