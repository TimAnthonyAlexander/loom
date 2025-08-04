package events

import (
	"context"
	"fmt"
	"loom/shared/models"
	"loom/task"
	"sync"
	"time"
)

// EventType defines the types of events that can be emitted
type EventType string

const (
	// Chat events
	ChatMessageReceived EventType = "chat:message_received"
	ChatStreamStarted   EventType = "chat:stream_started"
	ChatStreamChunk     EventType = "chat:stream_chunk"
	ChatStreamEnded     EventType = "chat:stream_ended"
	ChatError           EventType = "chat:error"

	// Task events (detailed - for internal LLM processing)
	TaskCreated            EventType = "task:created"
	TaskStatusChanged      EventType = "task:status_changed"
	TaskConfirmationNeeded EventType = "task:confirmation_needed"
	TaskCompleted          EventType = "task:completed"
	TaskFailed             EventType = "task:failed"

	// User task events (simplified - for UI display)
	UserTaskStatusChanged EventType = "user_task:status_changed"
	UserTaskStarted       EventType = "user_task:started"
	UserTaskCompleted     EventType = "user_task:completed"
	UserTaskFailed        EventType = "user_task:failed"
	UserTaskProgress      EventType = "user_task:progress"

	// File events
	FileChanged     EventType = "file:changed"
	FileTreeUpdated EventType = "file:tree_updated"
	FileOpened      EventType = "file:opened"

	// System events
	SystemConfigChanged EventType = "system:config_changed"
	SystemError         EventType = "system:error"
)

// Event represents an event in the system
type Event struct {
	Type      EventType   `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp"`
}

// EventHandler is a function that handles events
type EventHandler func(event Event)

// EventBus provides event-driven communication between components
type EventBus struct {
	handlers map[EventType][]EventHandler
	mutex    sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewEventBus creates a new event bus
func NewEventBus() *EventBus {
	ctx, cancel := context.WithCancel(context.Background())
	return &EventBus{
		handlers: make(map[EventType][]EventHandler),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Subscribe adds an event handler for a specific event type
func (eb *EventBus) Subscribe(eventType EventType, handler EventHandler) {
	eb.mutex.Lock()
	defer eb.mutex.Unlock()

	eb.handlers[eventType] = append(eb.handlers[eventType], handler)
}

// Unsubscribe removes all handlers for a specific event type
func (eb *EventBus) Unsubscribe(eventType EventType) {
	eb.mutex.Lock()
	defer eb.mutex.Unlock()

	delete(eb.handlers, eventType)
}

// Emit publishes an event to all registered handlers
func (eb *EventBus) Emit(eventType EventType, data interface{}) {
	eb.mutex.RLock()
	handlers := eb.handlers[eventType]
	eb.mutex.RUnlock()

	event := Event{
		Type:      eventType,
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
	}

	// Execute handlers in goroutines to avoid blocking
	for _, handler := range handlers {
		go func(h EventHandler) {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("Event handler panic for %s: %v\n", eventType, r)
				}
			}()
			h(event)
		}(handler)
	}
}

// Close shuts down the event bus
func (eb *EventBus) Close() {
	eb.cancel()
}

// Helper methods for common event emissions

// EmitChatMessage emits a chat message event
func (eb *EventBus) EmitChatMessage(message models.Message) {
	eb.Emit(ChatMessageReceived, message)
}

// EmitTaskStatusChange emits a task status change event
func (eb *EventBus) EmitTaskStatusChange(taskInfo models.TaskInfo) {
	eb.Emit(TaskStatusChanged, taskInfo)
}

// EmitTaskConfirmationNeeded emits a task confirmation needed event
func (eb *EventBus) EmitTaskConfirmationNeeded(confirmation models.TaskConfirmation) {
	eb.Emit(TaskConfirmationNeeded, confirmation)
}

// EmitFileTreeUpdate emits a file tree update event
func (eb *EventBus) EmitFileTreeUpdate(files []models.FileInfo) {
	eb.Emit(FileTreeUpdated, files)
}

// EmitSystemError emits a system error event
func (eb *EventBus) EmitSystemError(err error) {
	eb.Emit(SystemError, map[string]string{
		"error": err.Error(),
	})
}

// EmitUserTaskEvent emits a simplified user task event
func (eb *EventBus) EmitUserTaskEvent(event task.UserTaskEvent) {
	switch event.Type {
	case "started":
		eb.Emit(UserTaskStarted, event)
	case "completed":
		eb.Emit(UserTaskCompleted, event)
	case "failed":
		eb.Emit(UserTaskFailed, event)
	case "progress":
		eb.Emit(UserTaskProgress, event)
	default:
		eb.Emit(UserTaskStatusChanged, event)
	}
}
