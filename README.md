# Loom v2

Loom is a desktop-first AI coding companion that embeds a large-language-model "agent" directly inside a developer's workspace. Instead of replying with free-form suggestions, the model interacts through a fixed set of tools—read-file, grep-search, edit-file, run-tests, etc.—and every destructive action pauses for human approval.

## Architecture

Loom consists of a Go backend that orchestrates the interaction between the user, LLM, and the workspace, with a React frontend served via Wails for the UI.

### Core Components

- **Engine**: Orchestrates the communication loop between user, LLM, and tools
- **Tool Registry**: Manages available tools and their execution
- **LLM Adapters**: Provider-specific implementations for OpenAI, Anthropic Claude, and Ollama
- **Editor**: Safe file editing with validation and approval
- **Indexer**: Code searching using ripgrep
- **Memory**: Persistent storage for conversations and settings
- **Bridge**: Connects Go backend with React frontend

## Project Structure

```
loom/
├── go.mod
├── cmd/
│   ├── loom/          # CLI entry-point (headless REPL or agent tests)
│   └── loomgui/       # Wails desktop app
├── internal/          # Non-public implementation
│   ├── engine/        # Orchestrator + task loop
│   ├── adapter/       # LLM back-ends
│   │   ├── openai/
│   │   ├── anthropic/
│   │   └── ollama/
│   ├── tool/          # Tool registry & implementations
│   ├── editor/        # Safe file-editing & validation pipeline
│   ├── indexer/       # ripgrep wrappers, language stats
│   ├── memory/        # ~/.loom/* persistence
│   ├── bridge/        # Go⇆Wails JS bindings & event helpers
│   └── security/      # workspace jail, secret scrubbing
└── ui/                # React (Vite) SPA
```

## Available Tools

Loom currently includes these core tools:

1. **read_file**: Read contents of files in the workspace
2. **search_code**: Search the codebase for patterns using ripgrep
3. **edit_file**: Make changes to files with approval

## Development

### Prerequisites

- Go 1.18+
- Node.js 18+
- [Wails](https://wails.io/docs/gettingstarted/installation)
- ripgrep (for code search functionality)

### Setup

```bash
# Install Go dependencies
go mod tidy

# Install frontend dependencies
cd ui && npm install
```

### Running in Development Mode

```bash
# Start the development server
cd cmd/loomgui && wails dev
```

### Building

```bash
# Build the application
cd cmd/loomgui && wails build
```

## Configuration

Loom stores configuration in the following locations:

- `~/.loom/general/` - Global settings
- `~/.loom/projects/<hash>/` - Project-specific data

### Environment Variables

- `OPENAI_API_KEY`: API key for OpenAI models
- `ANTHROPIC_API_KEY`: API key for Anthropic Claude models

## License

[MIT License](LICENSE)