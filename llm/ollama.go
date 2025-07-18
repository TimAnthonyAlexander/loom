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

// OllamaAdapter implements LLMAdapter for Ollama API
type OllamaAdapter struct {
	client  *http.Client
	config  AdapterConfig
	baseURL string
}

// OllamaMessage represents a message in Ollama API format
type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OllamaChatRequest represents a chat request to Ollama
type OllamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

// OllamaChatResponse represents a chat response from Ollama
type OllamaChatResponse struct {
	Model     string        `json:"model"`
	CreatedAt time.Time     `json:"created_at"`
	Message   OllamaMessage `json:"message"`
	Done      bool          `json:"done"`
}

// NewOllamaAdapter creates a new Ollama adapter
func NewOllamaAdapter(config AdapterConfig) *OllamaAdapter {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	if config.Timeout == 0 {
		config.Timeout = DefaultTimeout
	}

	return &OllamaAdapter{
		client: &http.Client{
			Timeout: config.Timeout,
		},
		config:  config,
		baseURL: baseURL,
	}
}

// Send implements LLMAdapter.Send
func (o *OllamaAdapter) Send(ctx context.Context, messages []Message) (*Message, error) {
	// Convert our messages to Ollama format
	ollamaMessages := make([]OllamaMessage, len(messages))
	for i, msg := range messages {
		ollamaMessages[i] = OllamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	request := OllamaChatRequest{
		Model:    o.config.Model,
		Messages: ollamaMessages,
		Stream:   false,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/api/chat", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Ollama API error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama API returned status %d: %s", resp.StatusCode, string(body))
	}

	var response OllamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &Message{
		Role:      response.Message.Role,
		Content:   response.Message.Content,
		Timestamp: time.Now(),
	}, nil
}

// Stream implements LLMAdapter.Stream
func (o *OllamaAdapter) Stream(ctx context.Context, messages []Message, chunks chan<- StreamChunk) error {
	defer close(chunks)

	// Convert our messages to Ollama format
	ollamaMessages := make([]OllamaMessage, len(messages))
	for i, msg := range messages {
		ollamaMessages[i] = OllamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	request := OllamaChatRequest{
		Model:    o.config.Model,
		Messages: ollamaMessages,
		Stream:   true,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		chunks <- StreamChunk{Error: fmt.Errorf("failed to marshal request: %w", err)}
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/api/chat", bytes.NewBuffer(requestBody))
	if err != nil {
		chunks <- StreamChunk{Error: fmt.Errorf("failed to create request: %w", err)}
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		chunks <- StreamChunk{Error: fmt.Errorf("Ollama API error: %w", err)}
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("Ollama API returned status %d: %s", resp.StatusCode, string(body))
		chunks <- StreamChunk{Error: err}
		return err
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var response OllamaChatResponse
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			chunks <- StreamChunk{Error: fmt.Errorf("failed to decode streaming response: %w", err)}
			return err
		}

		if response.Message.Content != "" {
			chunks <- StreamChunk{Content: response.Message.Content}
		}

		if response.Done {
			chunks <- StreamChunk{Done: true}
			return nil
		}
	}

	if err := scanner.Err(); err != nil {
		chunks <- StreamChunk{Error: fmt.Errorf("error reading stream: %w", err)}
		return err
	}

	return nil
}

// GetModelName implements LLMAdapter.GetModelName
func (o *OllamaAdapter) GetModelName() string {
	return o.config.Model
}

// IsAvailable implements LLMAdapter.IsAvailable
func (o *OllamaAdapter) IsAvailable() bool {
	if o.config.Model == "" {
		return false
	}

	// Test connection to Ollama
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", o.baseURL+"/api/tags", nil)
	if err != nil {
		return false
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// parseOllamaModel extracts the model name from a model string like "ollama:codellama"
func parseOllamaModel(modelStr string) string {
	parts := strings.SplitN(modelStr, ":", 2)
	if len(parts) == 2 && parts[0] == "ollama" {
		return parts[1]
	}
	// Fallback - assume it's just the model name
	return modelStr
}
