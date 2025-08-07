# LLM Adapters for Loom

This package contains adapters for different LLM providers used in the Loom application. Each adapter implements the common `engine.LLM` interface to provide consistent interaction with different LLM backends.

## Supported Providers

### OpenAI

POST `/v1/chat/completions`  
Fields: `model`, `messages[]`, `tools[]`, `stream`

Tool call response: Assistant message with `tool_calls`

```go
client := openai.New(apiKey, "gpt-4o")
```

### Anthropic

POST `/v1/messages`  
Fields: `model`, `messages[]`, `tools[]`, `stream`

Tool call response: `stop_reason=tool_use`, `content[].type=="tool_use"`

```go
client := anthropic.New(apiKey, "claude-3-opus-20240229")
```

### Ollama

POST `http://localhost:11434/v1/chat/completions`  
OpenAI-compatible schema.

```go
client := ollama.New("http://localhost:11434/v1", "llama3")
```

## Configuration

Environment variables:

- `OPENAI_API_KEY`: API key for OpenAI
- `ANTHROPIC_API_KEY`: API key for Anthropic Claude
- `LOOM_PROVIDER`: Override provider selection (values: "openai", "anthropic", "ollama")
- `LOOM_MODEL`: Override model selection (e.g., "gpt-4o", "claude-3-opus", "llama3")
- `LOOM_ENDPOINT`: Override API endpoint
- `OLLAMA_ENDPOINT`: Custom Ollama endpoint (default: "http://localhost:11434")