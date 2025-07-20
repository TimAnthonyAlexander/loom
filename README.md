# Spoon
**Advanced AI-Driven Coding Assistant**

A sophisticated terminal-based AI coding assistant written in Go that provides a conversational interface for understanding, modifying, and extending codebases. Features autonomous task execution, intelligent context management, comprehensive security, and seamless project integration.

*Written by Tim Anthony Alexander. I am not a professional Go developer, so bear with me.*

[![CI/CD Pipeline](https://github.com/TimAnthonyAlexander/loom/actions/workflows/ci.yml/badge.svg)](https://github.com/TimAnthonyAlexander/loom/actions/workflows/ci.yml)

---

## Core Capabilities

### Advanced AI Integration
- **Multi-LLM Support** — OpenAI (GPT-4o, GPT-4.1), Claude (Sonnet 3.5, Opus 4), and Ollama (local models)
- **Project-Aware Intelligence** — Automatically analyzes project conventions and coding standards  
- **Autonomous Exploration** — Comprehensive project analysis without requiring explicit prompts
- **Streaming Responses** — Real-time response streaming for immediate feedback

### Sophisticated Task Execution
- **Natural Language Tasks** — AI uses intuitive commands like "🔧 READ main.go" and "🔧 EDIT config.json → add settings"
- **JSON Legacy Support** — Backward compatibility with JSON task format for existing workflows
- **Sequential Task Manager** — Objective-driven exploration with suppressed intermediate output
- **Task Confirmation** — Preview and approval for destructive operations
- **Task Debug Mode** — Troubleshooting for AI task generation issues
- **User Confirmation** — Safe execution with preview and approval for destructive operations

### Intelligent Context Management
- **Token-Aware Optimization** — Smart context window management with file references and snippets
- **Language-Aware Extraction** — Understands code structures (functions, classes, methods)
- **Auto-Summarization** — AI-powered session, progress, and action plan summaries
- **File Reference System** — Efficient file summaries without full content inclusion

### Comprehensive Security
- **Secret Detection** — 25+ patterns for API keys, passwords, tokens, certificates, and PII
- **Workspace Isolation** — All operations restricted to project workspace
- **Binary File Protection** — Automatic detection and exclusion of binary files
- **Gitignore Integration** — Respects .gitignore patterns for file operations

### Enhanced Terminal Interface
- **Multiple Views** — Chat, File Tree, Tasks with interactive navigation
- **Interactive Navigation** — Tab switching, scrolling, keyboard shortcuts
- **Task Confirmation** — Review and approve individual file changes with previews
- **Task Confirmation** — Clear previews with diff display for all modifications
- **Progress Tracking** — Real-time task execution status and history

### Testing Integration
- **Test Discovery** — Automatically finds tests in multiple languages (Go, JavaScript, Python)
- **Test Execution** — Runs tests and provides AI analysis of failures
- **Test-First Development** — Optional requirement for tests before implementation
- **Framework Support** — Go testing, Jest, pytest, and more

### Advanced Workspace Management
- **Fast Indexing** — Multi-threaded file indexing with real-time watching
- **Language Detection** — Automatic programming language identification
- **Cache System** — Compressed index cache for instant startup
- **File Watching** — Real-time updates with batched processing
- **Performance Optimized** — Handles large projects efficiently

### Workspace Management
- **File Operations** — Read, edit, create, and delete files with validation
- **Pre-condition Validation** — Check file state before destructive operations
- **Change Tracking** — Monitor file modifications and provide rationales

### Safety Features
- **Backup Creation** — Automatic backups before file modifications
- **User Confirmation** — Required approval for all destructive operations
- **File Validation** — Pre-checks before applying changes

### Session Management
- **Persistent Sessions** — Chat history preserved across sessions
- **Crash Recovery** — Automatic detection and recovery from incomplete sessions
- **Task Audit Trail** — Complete record of all task executions
- **Auto-save** — Configurable periodic session saving
- **Session Loading** — Continue from latest or load specific sessions by ID

---

## Installation

```bash
# Clone and build
git clone https://github.com/timanthonyalexander/loom
cd loom
go build -o loom .
```

## LLM Setup

### OpenAI Configuration
```bash
# Set API key
export OPENAI_API_KEY="your-api-key-here"
# OR configure via loom
./loom config set api_key "your-api-key-here"

# Configure model
./loom config set model "openai:gpt-4o"
```

**Available OpenAI Models:**
- `openai:o3` (recommended)
- `openai:gpt-4.1` (cheaper)

### Claude Configuration
```bash
# Set API key
export ANTHROPIC_API_KEY="your-api-key-here"
# OR configure via loom
./loom config set api_key "your-api-key-here"

# Configure model
./loom config set model "claude:claude-3-5-sonnet-20241022"
```

**Available Claude Models:**
- `claude:claude-3-5-sonnet-20241022` (balanced)
- `claude:claude-3-5-haiku-20241022` (fast)
- `claude:claude-opus-4-20250514` (most capable)

### Ollama Setup (Local Models)
```bash
# Install and start Ollama
ollama serve

# Pull a model
ollama pull codellama
# OR: ollama pull llama2, phi, deepseek-coder, etc.

# Configure Spoon
./loom config set model "ollama:codellama"
```

## Quick Start

```bash
# Initialize loom in your project
./loom init

# Start interactive AI assistant
./loom

# Continue from latest session
./loom --continue

# Load specific session
./loom --session "session-id"

# Force rebuild workspace index
./loom index
```

## Interactive Interface

### Navigation
- **`Tab`** — Switch between chat, file tree, and tasks views
- **`↑↓`** — Scroll in chat view
- **`Enter`** — Send message, approve changes, execute tasks
- **`y/n`** — Approve/cancel individual tasks
- **`Ctrl+S`** — Quick summary generation
- **`Ctrl+C`** — Exit safely

### Special Commands
- **`/help`** — Show comprehensive command help
- **`/files`** — Show file count and language breakdown
- **`/stats`** — Detailed project statistics and index information
- **`/tasks`** — Task execution history and current status
- **`/test`** — Test discovery results and execution
- **`/summary`** — AI-generated session summary (also `Ctrl+S`)
- **`/rationale`** — Change summaries and explanations
- **`/debug`** — Toggle task debugging mode
- **`/quit`** — Exit application

## Task System

Spoon's AI can autonomously perform coding tasks through intuitive natural language commands:

### Task Types

#### READ — Intelligent File Reading
```
🔧 READ main.go (max: 200 lines)
🔧 READ config.go (lines 10-50)
🔧 READ large_file.go (first 300 lines)
```
- Smart continuation for large files
- Contextual snippet extraction
- Language-aware structure detection
- Flexible line range and limit options

#### EDIT — Safe File Modification
```
🔧 EDIT main.go → add error handling

```go
package main

import (
    "fmt"
    "log"
)

func main() {
    if err := run(); err != nil {
        log.Fatal(err)
    }
}
```
```
- User confirmation required
- Diff preview before application
- Backup creation for recovery
- Natural language descriptions with actual code content

#### LIST — Directory Exploration
```
🔧 LIST .
🔧 LIST src/ recursive
🔧 LIST components/
```
- Respects .gitignore patterns
- Language and file type detection
- Intelligent directory traversal
- Optional recursive exploration

#### RUN — Command Execution
```
🔧 RUN go test
🔧 RUN npm run build (timeout: 60)
🔧 RUN go mod tidy
```
- User confirmation required
- Timeout protection
- Output capture and formatting
- Configurable timeout settings

### Natural Language vs JSON Format

Spoon now uses an intuitive natural language task format that's much more reliable and user-friendly than the previous JSON approach:

#### ✅ **New Natural Language Format (Recommended)**
```
🔧 READ main.go (max: 100 lines)
🔧 EDIT config.json → add database settings

```json
{
  "database": {
    "host": "localhost",
    "port": 5432
  }
}
```
```

#### 📜 **Legacy JSON Format (Still Supported)**
```json
{"type": "ReadFile", "path": "main.go", "max_lines": 100}
{"type": "EditFile", "path": "config.json", "content": "..."}
```

#### **Benefits of Natural Language Format:**
- **More Reliable** — LLMs generate natural language more consistently than JSON
- **Human Readable** — Task commands are easy to understand and debug
- **Less Error-Prone** — No syntax requirements, quotes, or bracket matching
- **Better UX** — Clear separation between task intent and actual content
- **Future-Proof** — Works with any LLM model without JSON formatting constraints

### Task Execution Modes

#### Autonomous Mode (Default)
AI executes tasks automatically with user confirmation for destructive operations.

#### Sequential Exploration
For comprehensive project analysis:
1. **Objective Setting** — AI establishes exploration goals
2. **Suppressed Exploration** — Quiet systematic analysis
3. **Comprehensive Synthesis** — Detailed architectural summary

#### Task Confirmation
For file modifications:
1. **Planning** — AI suggests necessary changes
2. **Preview** — Show diff of proposed changes
3. **User Approval** — Review and approve each change individually
4. **Execution** — Apply approved changes safely

## Configuration

### Enhanced Configuration Options
```json
{
  "model": "openai:gpt-4o",
  "api_key": "your-api-key-here",
  "base_url": "https://api.openai.com/v1",
  "enable_shell": false,
  "max_file_size": 512000,
  "max_context_tokens": 6000,
  "enable_test_first": false,
  "auto_save_interval": "30s"
}
```

#### Configuration Commands
```bash
# View current configuration
./loom config get model
./loom config get api_key

# Set configuration values
./loom config set model "openai:gpt-4o"
# OR: ./loom config set model "claude:claude-3-5-sonnet-20241022"
# OR: ./loom config set model "ollama:codellama"
./loom config set enable_shell true
./loom config set max_context_tokens 8000

# Reset to defaults
rm .loom/config.json
./loom init
```

#### Key Settings
- **`max_context_tokens`** — Context window size (default: 6000)
- **`enable_test_first`** — Require tests before implementation (default: false)
- **`auto_save_interval`** — Session persistence frequency (default: "30s")
- **`enable_shell`** — Allow shell command execution (default: false)
- **`max_file_size`** — Maximum file size for indexing (default: 512KB)

## Security Features

### Secret Detection
Automatically detects and redacts 25+ types of secrets:
- **API Keys** — AWS, Google, Azure, GitHub, GitLab
- **Authentication** — Passwords, tokens, certificates
- **Database** — Connection strings, credentials
- **Payment** — Stripe, PayPal keys
- **Communication** — Slack, Discord tokens
- **Personal Info** — Email addresses, phone numbers

### Security Constraints
- All operations restricted to workspace directory
- Binary files automatically excluded
- Secrets redacted from file content
- User confirmation for destructive operations
- File size limits prevent resource exhaustion

## Testing Features

### Test Discovery
Automatically discovers tests in multiple languages:
- **Go** — `*_test.go` files with standard testing
- **JavaScript/TypeScript** — `*.test.js`, `*.spec.js` with Jest/Mocha
- **Python** — `test_*.py`, `*_test.py` with pytest

### Test Integration
- **Automatic Execution** — Run tests after code changes
- **Failure Analysis** — AI analyzes test failures and suggests fixes
- **Test-First Development** — Optional enforcement of tests before implementation
- **Framework Support** — Works with popular testing frameworks

### Test Commands
```bash
# Discover and run tests
/test

# Test-specific responses to AI prompts
"yes" - Run discovered tests
"no" - Skip testing for now
```

## Project Statistics & Analysis

### Workspace Analysis
Spoon provides detailed insights into your project:
- **File Count** — Total files indexed with language breakdown
- **Size Analysis** — Total project size and file size distribution
- **Language Detection** — Automatic identification of 30+ programming languages
- **File Status** — Modified files, language breakdown, change tracking
- **Change Tracking** — Real-time file modification detection

### Performance Benchmarks
- **Small Projects** (< 100 files): < 200ms indexing
- **Medium Projects** (100-1000 files): < 1 second indexing
- **Large Projects** (1000+ files): < 2 seconds indexing
- **Cache Reload**: < 50ms for any project size
- **LLM Streaming**: < 100ms response latency

## Session & Persistence

### Session Features
- **Persistent History** — Chat preserved across sessions
- **Crash Recovery** — Automatic detection and recovery
- **Session Loading** — Continue from latest or specific sessions
- **Task Auditing** — Complete record of all operations
- **Auto-save** — Configurable session persistence

### Session Commands
```bash
# Continue latest session
./loom --continue

# Load specific session
./loom --session "2024-01-15-1430"

# List available sessions
./loom sessions list

# Clean old sessions
./loom sessions clean
```

## Troubleshooting

### LLM Issues
- **"LLM not available"** — Verify API key and model configuration
- **OpenAI errors** — Check API key with `echo $OPENAI_API_KEY`
- **Ollama connection** — Ensure Ollama is running with `ollama serve`
- **Model not found** — For Ollama, run `ollama pull <model-name>` first

### Task Execution Issues
- **AI explains but doesn't act** — Enable debug mode with `/debug` command
- **Tasks not parsed** — Check if AI is outputting proper natural language task commands
- **Task confirmation** — Destructive operations require user approval by design

### Debug Mode
```bash
# Enable task debugging
export LOOM_DEBUG_TASKS=1
./loom

# Or toggle in session
/debug
```

Debug mode shows detailed task parsing information when AI suggests actions but doesn't provide executable task commands. It helps identify when the AI indicates it wants to perform actions but doesn't use the proper natural language task format.

### Performance Issues
- **Slow indexing** — Check `.gitignore` patterns and exclude large directories
- **High memory usage** — Reduce `max_file_size` in configuration
- **Cache corruption** — Delete `.loom/index.cache` and rebuild

### Session Recovery
```bash
# Check for recoverable sessions
./loom sessions recover

# Force clean start
rm -rf .loom/sessions/
./loom init
```

## Example Usage Scenarios

### Project Exploration
```
> Tell me about this project

Spoon: I'll comprehensively analyze this project. Let me start by reading the README and exploring the structure.

OBJECTIVE: Understand this Go project's architecture and key components

README.md
root/
main.go
...

OBJECTIVE_COMPLETE: This is a sophisticated AI coding assistant built in Go...
[Detailed architectural analysis follows]
```

### Code Implementation
```
> Add error handling to the main function

Spoon: I'll enhance the main function with comprehensive error handling.

📖 Reading file: main.go

🔧 READ main.go

✏️ Editing main.go → add error handling and logging

🔧 EDIT main.go → add error handling and logging

TASK CONFIRMATION REQUIRED

[Diff preview shown]

> y

Applied Edit main.go
Enhanced main function with logging and structured error handling.
```

### Testing Integration
```
> Run the tests

Test Discovery Complete
Found 15 tests in the workspace. Would you like to run them?

> yes

All tests passed! Your changes look good.
Test Results: 15 passed, 0 failed, 0 skipped
```

### File Management
```
> Show me the current workspace state

Workspace Status:
Files: 42 indexed
Languages: Go 78.2%, Markdown 12.1%, YAML 9.7%
Recent changes: 3 files modified
Tasks: Ready

> Tell me about the recent changes

Recent Changes:
• Enhanced main.go with error handling
• Updated configuration structure
• Added comprehensive documentation
```

## Advanced Features

### Autonomous Exploration
Spoon can autonomously explore and understand codebases:
- **Project Convention Analysis** — Automatically detects coding standards
- **Architectural Insights** — Understands component relationships
- **Technology Stack Analysis** — Identifies frameworks and patterns
- **Best Practice Detection** — Recognizes project-specific conventions

### Context Optimization
- **Smart Token Management** — Efficient use of LLM context windows
- **File Reference System** — Summaries instead of full file inclusion
- **Language-Aware Snippets** — Extracts meaningful code structures
- **Auto-Summarization** — Compresses chat history intelligently

### File Operations
- **Individual Changes** — Edit, create, and delete files one at a time
- **Change Preview** — Review all file modifications before application
- **Task Confirmation** — Interactive approval for each operation
- **Safe Execution** — Backup and validation for all changes

## Future Roadmap

- **Advanced Code Analysis** — Syntax tree parsing and AST manipulation
- **Plugin System** — Custom task types and integrations
- **IDE Integration** — Language server protocol support
- **Multi-file Search/Replace** — Pattern-based modifications
- **Project Templates** — Scaffolding with customizable generators
- **CI/CD Integration** — Pipeline integration and automation
- **Real-time Collaboration** — Shared sessions and pair programming
- **Advanced Debugging** — Integrated debugger and profiler support
- **Code Quality Metrics** — Automated code review and suggestions

---

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

I am not expecting anyone to contribute as I started this as a little project just for myself.

## Acknowledgments

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) for the terminal interface, [fsnotify](https://github.com/fsnotify/fsnotify) for efficient file watching, [Cobra](https://github.com/spf13/cobra) for CLI command structure, and OpenAI, Claude, and Ollama for LLM integration.

*Tim Anthony Alexander, 2025.*
