package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ClaudeAdapter implements LLMAdapter for Claude API
type ClaudeAdapter struct {
	client  *http.Client
	config  AdapterConfig
	baseURL string
}

// ClaudeMessage represents a message in Claude API format
type ClaudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ClaudeRequest represents a request to Claude API
type ClaudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []ClaudeMessage `json:"messages"`
	Stream    bool            `json:"stream,omitempty"`
}

// ClaudeResponse represents a response from Claude API
type ClaudeResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model        string `json:"model"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// ClaudeStreamEvent represents a streaming event from Claude API
type ClaudeStreamEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index,omitempty"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta,omitempty"`
	Message ClaudeResponse `json:"message,omitempty"`
}

// NewClaudeAdapter creates a new Claude adapter
func NewClaudeAdapter(config AdapterConfig) *ClaudeAdapter {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}

	if config.Timeout == 0 {
		config.Timeout = DefaultTimeout
	}

	return &ClaudeAdapter{
		client: &http.Client{
			Timeout: config.Timeout,
		},
		config:  config,
		baseURL: baseURL,
	}
}

// Send implements LLMAdapter.Send
func (c *ClaudeAdapter) Send(ctx context.Context, messages []Message) (*Message, error) {
	// Convert our messages to Claude format
	claudeMessages := make([]ClaudeMessage, len(messages))
	for i, msg := range messages {
		claudeMessages[i] = ClaudeMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	request := ClaudeRequest{
		Model:     c.config.Model,
		MaxTokens: 4096, // Default max tokens
		Messages:  claudeMessages,
		Stream:    false,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/messages", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Claude API error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Claude API returned status %d: %s", resp.StatusCode, string(body))
	}

	var response ClaudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract text content from response
	var content string
	if len(response.Content) > 0 && response.Content[0].Type == "text" {
		content = response.Content[0].Text
	}

	return &Message{
		Role:      response.Role,
		Content:   content,
		Timestamp: time.Now(),
	}, nil
}

// Stream implements LLMAdapter.Stream
func (c *ClaudeAdapter) Stream(ctx context.Context, messages []Message, chunks chan<- StreamChunk) error {
	defer close(chunks)

	// Convert our messages to Claude format
	claudeMessages := make([]ClaudeMessage, len(messages))
	for i, msg := range messages {
		claudeMessages[i] = ClaudeMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	request := ClaudeRequest{
		Model:     c.config.Model,
		MaxTokens: 4096, // Default max tokens
		Messages:  claudeMessages,
		Stream:    true,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		chunks <- StreamChunk{Error: fmt.Errorf("failed to marshal request: %w", err)}
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/messages", bytes.NewBuffer(requestBody))
	if err != nil {
		chunks <- StreamChunk{Error: fmt.Errorf("failed to create request: %w", err)}
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.client.Do(req)
	if err != nil {
		chunks <- StreamChunk{Error: fmt.Errorf("Claude API error: %w", err)}
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("Claude API returned status %d: %s", resp.StatusCode, string(body))
		chunks <- StreamChunk{Error: err}
		return err
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Claude uses Server-Sent Events format
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				chunks <- StreamChunk{Done: true}
				return nil
			}

			var event ClaudeStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				// Skip malformed events and continue
				continue
			}

			switch event.Type {
			case "content_block_delta":
				if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
					chunks <- StreamChunk{Content: event.Delta.Text}
				}
			case "message_stop":
				chunks <- StreamChunk{Done: true}
				return nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		chunks <- StreamChunk{Error: fmt.Errorf("error reading stream: %w", err)}
		return err
	}

	return nil
}

// GetModelName implements LLMAdapter.GetModelName
func (c *ClaudeAdapter) GetModelName() string {
	return c.config.Model
}

// IsAvailable implements LLMAdapter.IsAvailable
func (c *ClaudeAdapter) IsAvailable() bool {
	if c.config.APIKey == "" || c.config.Model == "" {
		return false
	}

	// Test connection to Claude API with a minimal request
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create a minimal test message
	testMessages := []ClaudeMessage{
		{Role: "user", Content: "Hi"},
	}

	request := ClaudeRequest{
		Model:     c.config.Model,
		MaxTokens: 1,
		Messages:  testMessages,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return false
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/messages", bytes.NewBuffer(requestBody))
	if err != nil {
		return false
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Accept both 200 and rate limit errors as "available" since they indicate valid auth
	return resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusTooManyRequests
}
