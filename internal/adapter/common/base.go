package common

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/loom/loom/internal/engine"
)

// BaseAdapter provides common functionality shared across all HTTP-based LLM adapters
type BaseAdapter struct {
	APIKey     string
	Model      string
	Endpoint   string
	HTTPClient *http.Client
	Debug      bool
	Timeout    time.Duration
}

// NewBaseAdapter creates a new base adapter with common configuration
func NewBaseAdapter(apiKey, model, endpoint string) *BaseAdapter {
	timeout := 120 * time.Second
	if t := os.Getenv("LOOM_HTTP_TIMEOUT"); t != "" {
		if parsed, err := time.ParseDuration(t); err == nil {
			timeout = parsed
		}
	}

	// Enable debug logs via env flags
	debug := false
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("LOOM_DEBUG"))); v == "1" || v == "true" || v == "yes" || v == "debug" {
		debug = true
	}

	return &BaseAdapter{
		APIKey:   apiKey,
		Model:    model,
		Endpoint: endpoint,
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
		Debug:   debug,
		Timeout: timeout,
	}
}

// WithEndpoint sets a custom endpoint
func (b *BaseAdapter) WithEndpoint(endpoint string) *BaseAdapter {
	if strings.TrimSpace(endpoint) != "" {
		b.Endpoint = endpoint
	}
	return b
}

// WithDebug enables or disables debug logging
func (b *BaseAdapter) WithDebug(debug bool) *BaseAdapter {
	b.Debug = debug
	return b
}

// Debugf logs debug messages if debug mode is enabled
func (b *BaseAdapter) Debugf(format string, args ...interface{}) {
	if b.Debug {
		fmt.Printf("[adapter] "+format+"\n", args...)
	}
}

// MakeRequest creates and executes an HTTP request with common error handling
func (b *BaseAdapter) MakeRequest(ctx context.Context, requestBody interface{}, headers map[string]string) (*http.Response, error) {
	// Marshal request body
	reqBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("marshal error: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.Endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("request creation error: %v", err)
	}

	// Set default headers
	req.Header.Set("Content-Type", "application/json")

	// Set custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	b.Debugf("POST %s", b.Endpoint)

	// Make the request
	resp, err := b.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP error: %v", err)
	}

	return resp, nil
}

// ChatWithRetry implements the common retry logic for empty responses
// This replaces identical implementations across all adapter clients
func (b *BaseAdapter) ChatWithRetry(
	ctx context.Context,
	messages []engine.Message,
	tools []engine.ToolSchema,
	stream bool,
	resultCh chan<- engine.TokenOrToolCall,
	attemptChat func(context.Context, []engine.Message, []engine.ToolSchema, bool, chan<- engine.TokenOrToolCall) (bool, bool),
) {
	// Try first attempt and track if we receive any content
	contentReceived, toolCallReceived := attemptChat(ctx, messages, tools, stream, resultCh)

	// If we got empty response, retry with opposite streaming mode
	if !contentReceived && !toolCallReceived {
		select {
		case <-ctx.Done():
			return
		case resultCh <- engine.TokenOrToolCall{Token: "Retrying due to empty response..."}:
		}
		attemptChat(ctx, messages, tools, !stream, resultCh)
	}
}

// HandleErrorResponse reads and formats error responses from HTTP responses
func (b *BaseAdapter) HandleErrorResponse(resp *http.Response) error {
	data, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf("API error (%d): failed to read response body", resp.StatusCode)
	}
	return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(data))
}

// CreateScanner creates a buffered scanner for streaming responses
func (b *BaseAdapter) CreateScanner(body io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(body)
	// Increase the scanner buffer to safely handle larger SSE chunks
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	return scanner
}

// ProcessSSELine processes a single Server-Sent Events line
func (b *BaseAdapter) ProcessSSELine(line string) (event, data string, isDone bool) {
	if line == "" {
		return "", "", false
	}

	if strings.HasPrefix(line, "event: ") {
		return strings.TrimSpace(line[7:]), "", false
	}

	if strings.HasPrefix(line, "data: ") {
		payload := strings.TrimSpace(line[6:])
		if payload == "[DONE]" {
			return "", "", true
		}
		return "", payload, false
	}

	// Skip other SSE formats (comments, etc.)
	return "", "", false
}

// SendErrorToken sends an error message as a token to the result channel
func (b *BaseAdapter) SendErrorToken(ctx context.Context, resultCh chan<- engine.TokenOrToolCall, format string, args ...interface{}) {
	select {
	case <-ctx.Done():
		return
	case resultCh <- engine.TokenOrToolCall{Token: fmt.Sprintf(format, args...)}:
	}
}
