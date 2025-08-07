package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

// Definition describes a tool that can be invoked by the LLM.
type Definition struct {
	Name        string
	Description string
	JSONSchema  map[string]interface{}
	Safe        bool // true = no user confirmation required
	Handler     func(ctx context.Context, raw json.RawMessage) (interface{}, error)
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
func (r *Registry) Schemas() []map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	schemas := make([]map[string]interface{}, 0, len(r.tools))
	for _, def := range r.tools {
		schema := map[string]interface{}{
			"name":        def.Name,
			"description": def.Description,
			"parameters":  def.JSONSchema,
		}
		schemas = append(schemas, schema)
	}

	return schemas
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
