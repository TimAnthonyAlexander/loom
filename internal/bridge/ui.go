package bridge

import (
	"context"
	"sync"

	"github.com/loom/loom/internal/engine"
	"github.com/loom/loom/internal/tool"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the main application struct that bridges Go and JavaScript.
type App struct {
	ctx      context.Context
	cancelFn context.CancelFunc
	engine   *engine.Engine
	registry *tool.Registry
	mu       sync.Mutex
}

// NewApp creates a new application bridge.
func NewApp(eng *engine.Engine, registry *tool.Registry) *App {
	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		ctx:      ctx,
		cancelFn: cancel,
		engine:   eng,
		registry: registry,
	}

	// Set the app as the UI bridge for the engine
	eng.SetBridge(app)

	return app
}

// SetContext sets the Wails runtime context.
func (a *App) SetContext(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.ctx = ctx
}

// SendUserMessage sends a message from the user to the engine.
func (a *App) SendUserMessage(message string) {
	a.engine.Enqueue(message)
}

// Approve responds to an approval request.
func (a *App) Approve(id string, ok bool) {
	a.engine.ResolveApproval(id, ok)
}

// GetTools returns all available tools for the UI.
func (a *App) GetTools() []map[string]interface{} {
	tools := a.registry.Tools()
	result := make([]map[string]interface{}, 0, len(tools))

	for _, tool := range tools {
		result = append(result, map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"safe":        tool.Safe,
		})
	}

	return result
}

// SendChat implements UIBridge.SendChat.
func (a *App) SendChat(role, text string) {
	a.emitEvent("chat:new", map[string]interface{}{
		"role":    role,
		"content": text,
	})
}

// EmitAssistant streams assistant messages to the frontend.
func (a *App) EmitAssistant(text string) {
	a.emitEvent("assistant-msg", text)
}

// PromptApproval implements UIBridge.PromptApproval.
func (a *App) PromptApproval(actionID, summary, diff string) (approved bool) {
	a.emitEvent("task:prompt", map[string]interface{}{
		"id":      actionID,
		"summary": summary,
		"diff":    diff,
	})

	// Note: The actual approval comes through the Approve method
	// This just emits the event to the UI
	return false
}

// emitEvent emits an event to the frontend.
func (a *App) emitEvent(name string, data interface{}) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, name, data)
	}
}
