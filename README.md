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
│       ├── main.go    # Main application entry point
│       ├── wails.json # Wails configuration file
│       └── frontend/  # React (Vite) frontend
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

# Install Wails CLI (if not already installed)
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Make sure the wails binary is in your PATH
export PATH=$PATH:$HOME/go/bin

# Install frontend dependencies
cd cmd/loomgui/frontend && npm install
cd ../..
```

### Running in Development Mode

```bash
# Start the development server
cd cmd/loomgui 
wails dev

# If wails is not in your PATH
~/go/bin/wails dev
```

This will start the development server and open the application in a window. You can also view the application in your browser by navigating to http://localhost:34115.

### Building for Production

```bash
# Build the application for your current platform
cd cmd/loomgui
wails build

# For specific platforms (examples)
wails build -platform=darwin/universal
wails build -platform=windows/amd64
wails build -platform=linux/amd64
```

The built application will be available in the `cmd/loomgui/build/bin` directory.

## Configuration

Loom stores configuration in the following locations:

- `~/.loom/general/` - Global settings
- `~/.loom/projects/<hash>/` - Project-specific data

### Environment Variables

- `OPENAI_API_KEY`: API key for OpenAI models
- `ANTHROPIC_API_KEY`: API key for Anthropic Claude models

## Troubleshooting

### Common Issues

1. **`wails` command not found**
   
   Ensure the Go bin directory is in your PATH:
   ```bash
   export PATH=$PATH:$HOME/go/bin
   ```
   
   For permanent addition, add this line to your shell profile (.bashrc, .zshrc, etc.).

2. **TypeScript errors during build**
   
   Make sure all dependencies are correctly installed:
   ```bash
   cd cmd/loomgui/frontend && npm ci
   ```

3. **Error loading modules during frontend build**
   
   If you encounter module loading issues, try reinstalling specific packages:
   ```bash
   cd cmd/loomgui/frontend
   npm install typescript@5.0.2 --save-dev
   npm install vite@4.3.9 --save-dev
   ```

4. **API key errors**
   
   Ensure that the appropriate API keys are set:
   ```bash
   export OPENAI_API_KEY="your_openai_key"
   export ANTHROPIC_API_KEY="your_anthropic_key"
   ```

## License

[MIT License](LICENSE)