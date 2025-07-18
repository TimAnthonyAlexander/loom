# Loom - AI-Driven Coding Assistant

Loom is a terminal-based, AI-driven coding assistant written in Go. It runs inside any project folder and gives developers a conversational interface to modify and extend their codebase.

## Usage

### Installation
```bash
go build -o loom .
```

### LLM Setup

#### OpenAI Setup
1. Get an API key from https://platform.openai.com/
2. Set your API key:
   ```bash
   export OPENAI_API_KEY="your-api-key-here"
   # OR configure via loom
   ./loom config set api_key "your-api-key-here"
   ```
3. Configure your model:
   ```bash
   ./loom config set model "openai:gpt-4o"
   # Available models: gpt-4o, gpt-4, gpt-3.5-turbo, etc.
   ```

#### Ollama Setup
1. Install Ollama from https://ollama.ai/
2. Start Ollama service:
   ```bash
   ollama serve
   ```
3. Pull a model:
   ```bash
   ollama pull codellama
   # or: ollama pull llama2, phi, etc.
   ```
4. Configure Loom:
   ```bash
   ./loom config set model "ollama:codellama"
   ```

### Basic Commands
```bash
# Initialize loom in current project
./loom init

# View/edit configuration
./loom config get model
./loom config set model "openai:gpt-4o"
./loom config set api_key "your-api-key"
./loom config set base_url "https://api.openai.com/v1"  # optional custom endpoint

# Force rebuild index
./loom index

# Start interactive TUI with AI chat
./loom
```

### Enhanced TUI Interface
- **Chat View**: Type messages, chat with AI about your project with advanced context management
- **File Tree View**: Press `Tab` to switch, use `↑↓` to scroll through indexed files  
- **Action Plan View**: Press `Ctrl+P` to view current multi-file action plans and execution status
- **Batch Approval View**: Interactive interface for reviewing and approving multiple file changes
- **Git Status View**: Press `Ctrl+G` to view detailed Git repository status
- **Undo History View**: Press `Ctrl+U` to view and manage undo history

#### Enhanced Navigation: 
- `Tab` - Switch between chat, file tree, and task views
- `Ctrl+P` - Action Plan view
- `Ctrl+G` - Git Status view  
- `Ctrl+U` - Undo History view
- `Ctrl+Z` - Quick undo last action
- `↑↓` - Scroll in views and navigate edits in batch approval
- `Enter` - Send message in chat, approve selected edit in batch mode
- `A` - Approve all changes in batch approval mode
- `R` - Reject all changes in batch approval mode
- `y`/`n` - Approve/cancel individual tasks when prompted
- `Ctrl+C` or `/quit` - Exit safely

#### Enhanced Commands:
- `/files` - Show file count and language breakdown
- `/stats` - Show detailed project statistics  
- `/tasks` - Show task execution history
- `/test` - Show test discovery, run tests, and see results
- `/summary` - Generate AI-powered session summary (also Ctrl+S)
- `/rationale` - Show change summaries and explanations
- `/git` - Display Git repository status
- `/commit "message"` - Commit staged changes with specified message
- `/undo` - Undo the last applied action
- `/quit` - Exit the application

### Chat Features
- **AI Conversation**: Ask questions about your code, architecture, or programming concepts
- **Project Context**: AI has automatic access to your project's file structure and language breakdown
- **Streaming Responses**: Real-time response streaming for immediate feedback
- **Chat History**: Persistent history across sessions (stored in `.loom/history/`)
- **Smart Memory**: Automatic message trimming while preserving important context
- **Task Execution**: AI can read files, make edits, list directories, and run commands
- **Test Integration**: Automatic test discovery and execution with AI feedback on failures
- **Change Tracking**: Every code change comes with natural language summaries and rationale
- **Session Recovery**: Automatic detection and recovery from incomplete sessions or crashes
- **Enhanced Error Handling**: Graceful recovery from errors with detailed explanations
- **Safe Operations**: All destructive operations require explicit user confirmation

### Task Capabilities
Loom's AI can now perform actual coding tasks through structured JSON commands:

#### File Operations
- **Read Files**: `{"type": "ReadFile", "path": "main.go", "max_lines": 100}`
- **Edit Files**: `{"type": "EditFile", "path": "main.go", "content": "new content"}`
- **List Directories**: `{"type": "ListDir", "path": "src/", "recursive": true}`

#### Shell Commands (Optional)
- **Run Commands**: `{"type": "RunShell", "command": "go build", "timeout": 10}`

#### Security Features
- All file paths restricted to workspace directory
- Binary files automatically detected and protected
- Secrets redacted from file content using regex patterns
- EditFile and RunShell require user confirmation (y/n prompt)
- File size limits prevent reading oversized files

### Enhanced Configuration
```json
{
  "model": "openai:gpt-4o",
  "enable_shell": false,
  "max_file_size": 512000,
  "max_context_tokens": 6000,
  "enable_test_first": false,
  "auto_save_interval": "30s",
  "api_key": "your-api-key-here",
  "base_url": "https://api.openai.com/v1"
}
```

#### New Configuration Options:
- `max_context_tokens` - Maximum tokens for LLM context window (default: 6000)
- `enable_test_first` - Require tests before implementation changes (default: false)  
- `auto_save_interval` - Session auto-save interval (default: "30s")
- Enhanced secret detection with 25+ patterns automatically enabled

### Model Examples
```bash
# OpenAI models
./loom config set model "openai:gpt-4o"
./loom config set model "openai:gpt-4"
./loom config set model "openai:gpt-3.5-turbo"

# Ollama models
./loom config set model "ollama:codellama"
./loom config set model "ollama:llama2"
./loom config set model "ollama:phi"
```

### Example Chat Session
```
> Add error handling to the main function

Loom: I'll help you add error handling to the main function. Let me first read the current main.go file to understand its structure.

🔧 Task: Read main.go (max 200 lines)
✅ Status: Success
📄 Output: File: main.go (13 lines read)

package main

import (
	"loom/cmd"
	"os"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

I can see the main function already has basic error handling, but we can improve it with logging. Let me enhance it:

🔧 Task: Edit main.go (apply diff)  
⚠️  TASK CONFIRMATION REQUIRED

Task: Edit main.go (apply diff)

Preview:
[Diff shows addition of fmt import and error logging before os.Exit(1)]

Do you want to proceed with this task?
Press 'y' to approve, 'n' to cancel

> y

✅ Applied Edit main.go (apply diff)

The main function now includes proper error logging before exiting. The changes add:
- Import of the fmt package for error output
- fmt.Fprintf to log errors to stderr before exiting
- This provides better debugging information when the application fails
```

### Enhanced Project Structure
```
loom/
├── main.go                 # Entry point
├── cmd/
│   ├── root.go            # Root command with enhanced integrations
│   ├── init.go            # Init command
│   ├── config.go          # Config management commands
│   └── index.go           # Index rebuild command
├── config/
│   └── config.go          # Config system with enhanced settings
├── workspace/
│   └── workspace.go       # Workspace detection and .loom setup
├── indexer/
│   ├── indexer.go         # Core indexing engine with fsnotify
│   └── gitignore.go       # .gitignore pattern matching
├── llm/
│   ├── adapter.go         # LLM adapter interface
│   ├── openai.go          # OpenAI implementation
│   ├── ollama.go          # Ollama implementation
│   └── factory.go         # Adapter factory
├── context/
│   └── context.go         # Context management and token optimization
├── chat/
│   └── session.go         # Chat session management with enhanced audit
├── task/
│   ├── task.go            # Task protocol and parsing
│   ├── executor.go        # Task execution engine
│   ├── staged_executor.go # Multi-file staging and coordination
│   ├── action_plan.go     # Action plan management
│   ├── manager.go         # Task orchestration and recursive chat
│   └── task_test.go       # Comprehensive task tests
├── git/
│   └── git.go             # Git integration and status management
├── security/
│   └── secrets.go         # Enhanced secret detection and redaction
├── undo/
│   └── undo.go            # Undo system with backup management
├── session/
│   └── persistence.go     # Session persistence and crash recovery
├── tui/
│   ├── tui.go             # Base TUI implementation
│   └── enhanced_tui.go    # Enhanced TUI with batch approval
└── .loom/
    ├── config.json        # Local configuration
    ├── index.cache        # Compressed file index cache
    ├── sessions/          # Session persistence files
    │   ├── session_*.json # Complete session state
    │   └── *.safe.json    # Sanitized session backups
    ├── undo/              # Undo stack and backups
    │   ├── stack.json     # Undo action history
    │   └── *.backup       # File backups
    ├── staging/           # Temporary staging area
    └── history/           # Chat history files with enhanced audit
        └── 2024-01-15-1430.jsonl
```

## Troubleshooting

### LLM Issues
- **"LLM not available"**: Check your API key and model configuration
- **OpenAI errors**: Verify API key with `echo $OPENAI_API_KEY` or check config
- **Ollama connection**: Ensure Ollama is running with `ollama serve`
- **Model not found**: For Ollama, run `ollama pull <model-name>` first

### Configuration
```bash
# Check current configuration
./loom config get model
./loom config get api_key

# Reset to defaults
rm .loom/config.json
./loom init
```

## Performance Benchmarks
- **Small projects** (< 100 files): < 200ms indexing
- **Medium projects** (100-1000 files): < 1 second indexing  
- **Large projects** (1000+ files): < 2 seconds indexing
- **Cache reload**: < 50ms for any project size
- **LLM Streaming**: Real-time response chunks with <100ms latency

## Next Steps (Future Milestones)
- Advanced code analysis with syntax tree parsing and AST manipulation
- Plugin system for custom task types and integrations
- Code refactoring tools with automated testing support
- IDE integration and language server protocol support
- Multi-file search and replace with pattern matching
- Project templates and scaffolding with customizable generators
- Integration with CI/CD pipelines and development workflows
- Real-time collaboration features and shared sessions
- Advanced debugging and profiling integration
- Code quality metrics and automated code review

The user maintains complete control with:
- Clear summaries and diffs for all changes
- Batch approval workflows for multi-file edits
- Comprehensive undo system with persistent history
- Git integration with automatic staging and commit support
- Session persistence with crash recovery
- Enhanced security with comprehensive secret detection 
