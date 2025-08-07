package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

// Schema represents the schema for a tool as exposed to the LLM.
type Schema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"` // JSON-Schema fragment
	Safe        bool           `json:"safe"`       // if false → needs approval
}

// Definition describes a tool that can be invoked by the LLM.
type Definition struct {
	Name        string
	Description string
	JSONSchema  map[string]interface{}
	Safe        bool // true = no user confirmation required
	Handler     func(ctx context.Context, raw json.RawMessage) (interface{}, error)
	Schema      Schema // Pre-computed schema for LLM
}

// Registry manages the available tools.
type Registry struct {
	tools map[string]Definition
	mu    sync.RWMutex
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Definition),
	}
}

// Register adds a tool to the registry.
func (r *Registry) Register(def Definition) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[def.Name]; exists {
		return fmt.Errorf("tool %q already registered", def.Name)
	}

	if def.Handler == nil {
		return errors.New("tool handler cannot be nil")
	}

	// Initialize the Schema field if not already set
	if def.Schema.Name == "" {
		def.Schema = Schema{
			Name:        def.Name,
			Description: def.Description,
			Parameters:  def.JSONSchema,
			Safe:        def.Safe,
		}
	}

	r.tools[def.Name] = def
	return nil
}

// Get retrieves a tool definition by name.
func (r *Registry) Get(name string) (Definition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	def, ok := r.tools[name]
	return def, ok
}

// Tools returns all registered tool definitions.
func (r *Registry) Tools() []Definition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]Definition, 0, len(r.tools))
	for _, def := range r.tools {
		defs = append(defs, def)
	}

	return defs
}

// Schemas returns the schema of all registered tools.
func (r *Registry) Schemas() []Schema {
	r.mu.RLock()
	defer r.mu.RUnlock()

	schemas := make([]Schema, 0, len(r.tools))
	for _, t := range r.tools {
		schemas = append(schemas, t.Schema)
	}
	return schemas
}

// ExecutionResult contains the result of a tool execution
type ExecutionResult struct {
	Content string `json:"content"` // The content to return to the LLM
	Diff    string `json:"diff"`    // Diff representation for approvals
	Safe    bool   `json:"safe"`    // Whether this execution is safe
}

// ToolCall represents a request to invoke a tool
type ToolCall struct {
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

// Invoke executes a tool by name with the given arguments.
func (r *Registry) Invoke(ctx context.Context, name string, args json.RawMessage) (interface{}, error) {
	r.mu.RLock()
	def, ok := r.tools[name]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown tool %q", name)
	}

	return def.Handler(ctx, args)
}

// InvokeToolCall executes a tool call and returns a structured result.
func (r *Registry) InvokeToolCall(ctx context.Context, call *ToolCall) (*ExecutionResult, error) {
	result, err := r.Invoke(ctx, call.Name, call.Args)
	if err != nil {
		return &ExecutionResult{
			Content: fmt.Sprintf("Error: %v", err),
			Diff:    "",
			Safe:    true, // Errors are safe to show
		}, nil
	}

	// Convert result to string if not already an ExecutionResult
	if execResult, ok := result.(*ExecutionResult); ok {
		return execResult, nil
	}

	// Convert to string representation
	var content string
	switch v := result.(type) {
	case string:
		content = v
	case []byte:
		content = string(v)
	default:
		jsonBytes, _ := json.MarshalIndent(result, "", "  ")
		content = string(jsonBytes)
	}

	// Get tool definition to check safety
	r.mu.RLock()
	def, ok := r.tools[call.Name]
	r.mu.RUnlock()

	safe := ok && def.Safe

	return &ExecutionResult{
		Content: content,
		Diff:    "", // No diff for regular tools
		Safe:    safe,
	}, nil
}
