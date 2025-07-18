# Loom - AI-Driven Coding Assistant

Loom is a terminal-based, AI-driven coding assistant written in Go. It runs inside any project folder and gives developers a conversational interface to modify and extend their codebase.

## Milestone 1 - Complete ✅

### Features Implemented

#### 1. Command-Line Interface (CLI)
- ✅ Binary name: `loom`
- ✅ Running `loom` starts an interactive TUI session
- ✅ Running `loom --help` shows usage information
- ✅ Commands available:
  - `loom init` - initializes .loom folder and config in current directory
  - `loom config get <key>` - retrieves config values
  - `loom config set <key> <value>` - updates config values

#### 2. Workspace Detection
- ✅ Automatically detects workspace root via Git repository
- ✅ Falls back to current directory if not in a Git repo
- ✅ Workspace path saved in memory

#### 3. Config System
- ✅ Loads config from global: `~/.loom/config.json`
- ✅ Loads config from local: `<workspace>/.loom/config.json`
- ✅ Merges global + local with local taking precedence
- ✅ Default config options:
  ```json
  {
    "model": "openai:gpt-4o",
    "enable_shell": false
  }
  ```
- ✅ `loom config get/set` commands to manipulate keys

#### 4. Basic TUI Interface
- ✅ Uses Bubble Tea framework for terminal UI
- ✅ Shows welcome message
- ✅ Input box for typing
- ✅ Message pane (echo functionality for testing)
- ✅ Ctrl+C to exit gracefully

#### 5. Project Directory Setup
- ✅ Creates `.loom/` folder on first run in workspace
- ✅ Places `index.cache` (empty for now)
- ✅ Places `config.json` (when using init command)

## Usage

### Installation
```bash
go build -o loom .
```

### Basic Commands
```bash
# Initialize loom in current project
./loom init

# Get configuration value
./loom config get model

# Set configuration value
./loom config set enable_shell true

# Start interactive TUI
./loom
```

### Project Structure
```
loom/
├── main.go                 # Entry point
├── cmd/
│   ├── root.go            # Root command and TUI launcher
│   ├── init.go            # Init command
│   └── config.go          # Config management commands
├── config/
│   └── config.go          # Config system (load/save/merge)
├── workspace/
│   └── workspace.go       # Workspace detection and .loom setup
├── tui/
│   └── tui.go             # Bubble Tea TUI interface
└── .loom/
    ├── config.json        # Local configuration
    └── index.cache        # Future: file indexing cache
```

## Next Steps (Future Milestones)
- Model integration (OpenAI/Ollama)
- File indexing and search
- Chat history persistence
- LLM-generated task execution
- Code modification capabilities 