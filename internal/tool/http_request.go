package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strings"
	"time"
)

// HTTPRequestArgs describes an HTTP request to perform.
type HTTPRequestArgs struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
	Timeout int               `json:"timeout,omitempty"` // seconds
}

// HTTPResponse represents the HTTP response returned to the model.
type HTTPResponse struct {
	Status     int               `json:"status"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	DurationMs int               `json:"duration_ms"`
}

// RegisterHTTPRequest registers the http_request tool which performs HTTP calls.
func RegisterHTTPRequest(registry *Registry) error {
	return registry.Register(Definition{
		Name:        "http_request",
		Description: "Make HTTP calls against local dev servers or APIs.",
		Safe:        true,
		JSONSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"method": map[string]interface{}{
					"type":        "string",
					"description": "HTTP method (GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS)",
				},
				"url": map[string]interface{}{
					"type":        "string",
					"description": "Absolute URL, e.g. http://localhost:3000/api",
				},
				"headers": map[string]interface{}{
					"type":                 "object",
					"description":          "Request headers as key/value strings",
					"additionalProperties": map[string]interface{}{"type": "string"},
				},
				"body": map[string]interface{}{
					"type":        "string",
					"description": "Raw request body as string (use with appropriate Content-Type)",
				},
				"timeout": map[string]interface{}{
					"type":        "integer",
					"description": "Timeout in seconds (default 60, max 600)",
				},
			},
			"required": []string{"method", "url"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			var args HTTPRequestArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, fmt.Errorf("failed to parse arguments: %w", err)
			}
			return performHTTPRequest(ctx, args)
		},
	})
}

func performHTTPRequest(parentCtx context.Context, args HTTPRequestArgs) (*HTTPResponse, error) {
	method := strings.ToUpper(strings.TrimSpace(args.Method))
	if method == "" {
		return nil, errors.New("method is required")
	}
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions:
		// ok
	default:
		return nil, fmt.Errorf("unsupported method: %s", method)
	}

	if strings.TrimSpace(args.URL) == "" {
		return nil, errors.New("url is required")
	}
	// Validate URL and restrict to http/https
	parsed, err := neturl.Parse(args.URL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid url: %s", args.URL)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported url scheme: %s", parsed.Scheme)
	}

	timeoutSeconds := normalizeHTTPTimeout(args.Timeout)
	ctx, cancel := context.WithTimeout(parentCtx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	var bodyReader io.Reader
	if args.Body != "" {
		bodyReader = strings.NewReader(args.Body)
	}

	req, err := http.NewRequestWithContext(ctx, method, parsed.String(), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	// Set headers
	for k, v := range args.Headers {
		if strings.TrimSpace(k) == "" {
			continue
		}
		req.Header.Set(k, v)
	}

	client := &http.Client{}

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)
	if err != nil {
		// Return a structured error as a safe result
		return &HTTPResponse{
			Status:     0,
			Headers:    map[string]string{},
			Body:       fmt.Sprintf("request error: %v", err),
			DurationMs: int(duration / time.Millisecond),
		}, nil
	}
	defer resp.Body.Close()

	// Read response body (unbounded; assumes dev/local APIs)
	data, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		// Include partial data if any
		data = []byte(fmt.Sprintf("failed to read body: %v", readErr))
	}

	// Flatten headers to map[string]string (join multi-values)
	flatHeaders := make(map[string]string, len(resp.Header))
	for k, vals := range resp.Header {
		flatHeaders[k] = strings.Join(vals, ", ")
	}

	return &HTTPResponse{
		Status:     resp.StatusCode,
		Headers:    flatHeaders,
		Body:       string(data),
		DurationMs: int(duration / time.Millisecond),
	}, nil
}

func normalizeHTTPTimeout(seconds int) int {
	if seconds <= 0 {
		return 60
	}
	if seconds > 600 {
		return 600
	}
	return seconds
}
