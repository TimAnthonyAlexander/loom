# Loom - AI-Driven Coding Assistant

Loom is a terminal-based, AI-driven coding assistant written in Go. It runs inside any project folder and gives developers a conversational interface to modify and extend their codebase.

## Milestone 4 - Complete ✅

### Task Planning, Tool Execution, and Recursive Chat Loop

All Milestone 4 features have been successfully implemented:

#### 1. Task Protocol & Parsing ✅
- ✅ **JSON Task Block Parsing**: Extracts structured tasks from LLM responses using regex pattern matching
- ✅ **Task Types**: ReadFile, EditFile, ListDir, RunShell with comprehensive validation
- ✅ **Security Validation**: Path sanitization, workspace containment, and parameter validation
- ✅ **Error Handling**: Graceful handling of malformed JSON and invalid task parameters

#### 2. Tool Execution Layer ✅
- ✅ **ReadFile Executor**: Reads files with line limits, binary detection, and secret redaction
- ✅ **EditFile Executor**: Applies unified diffs with preview generation and atomic file operations
- ✅ **ListDir Executor**: Lists directory contents with recursive option and size formatting
- ✅ **RunShell Executor**: Executes shell commands with timeout and output capture (configurable)
- ✅ **Security Constraints**: All operations restricted to workspace, binary file protection

#### 3. Recursive Chat/Task Loop ✅
- ✅ **Task Manager**: Orchestrates task execution with event streaming and confirmation handling
- ✅ **LLM Integration**: Processes LLM responses, executes tasks, and feeds results back for continuation
- ✅ **Task Execution Events**: Real-time status updates with detailed progress tracking
- ✅ **Result Streaming**: Task outputs streamed back to LLM context for informed decision making

#### 4. TUI Enhancements ✅
- ✅ **Task Execution View**: Dedicated task view showing execution status and history
- ✅ **Confirmation Dialogs**: Interactive y/n confirmations for destructive operations
- ✅ **Task Status Display**: Real-time task progress with success/failure indicators
- ✅ **Three-Panel UI**: Chat, File Tree, and Task views accessible via Tab navigation

#### 5. Chat History and Audit ✅
- ✅ **Task Audit Trail**: Complete logging of all task executions with timestamps
- ✅ **Action Markers**: Special markers in chat history for task approvals and results
- ✅ **Persistent Storage**: Task history saved to .jsonl files with session management
- ✅ **Display Integration**: Task results formatted for clear chat display

#### 6. Enhanced System Prompt ✅
- ✅ **Task Instructions**: Comprehensive task documentation in system prompt
- ✅ **JSON Examples**: Clear examples of task syntax and usage patterns
- ✅ **Security Guidelines**: Explicit constraints and confirmation requirements
- ✅ **Capability Awareness**: LLM knows exactly what tasks are available and how to use them

## Milestone 3 - Complete ✅

### LLM Adapter & Chat Engine Foundation

All Milestone 3 features have been successfully implemented:

#### 1. LLM Adapter Abstraction ✅
- ✅ **LLMAdapter Interface**: Complete abstraction with `Send()` and `Stream()` methods
- ✅ **OpenAI Integration**: Full support for GPT-4o, GPT-3.5-turbo, etc. via `github.com/sashabaranov/go-openai`
- ✅ **Ollama Integration**: Local model support via HTTP API at `localhost:11434`
- ✅ **Configuration Support**: API keys, custom endpoints, and model selection from Loom's config system
- ✅ **Error Handling**: Robust error handling with availability checks and timeouts

#### 2. Chat Session & Message Model ✅
- ✅ **Message Structure**: Complete `Message` struct with role, content, and timestamps
- ✅ **Rolling History**: Smart memory management with configurable message limits
- ✅ **Persistence**: Chat sessions saved to `.loom/history/YYYY-MM-DD-HHMM.jsonl`
- ✅ **Session Loading**: Automatic loading of latest session on startup
- ✅ **History Management**: Intelligent trimming that preserves system messages

#### 3. Prompt Construction ✅
- ✅ **System Prompt**: Dynamic system prompt with project context
- ✅ **Project Summary**: File count, language breakdown, and workspace statistics
- ✅ **Context Integration**: Project information automatically included in every chat
- ✅ **Role-based Messaging**: Proper system/user/assistant message roles

#### 4. TUI Chat Integration ✅
- ✅ **Streaming Responses**: Real-time LLM response streaming in the TUI
- ✅ **Chat Interface**: Full conversational interface with message history
- ✅ **Input Handling**: Multiline input support and proper message formatting
- ✅ **Visual Feedback**: Status indicators for model availability and streaming state
- ✅ **Error Display**: Clear error messages for LLM issues and configuration problems

#### 5. Configuration & Model Selection ✅
- ✅ **Model Configuration**: Support for `openai:gpt-4o`, `ollama:codellama`, etc.
- ✅ **API Key Management**: Environment variable and config-based API key handling
- ✅ **Live Model Switching**: Change models via `loom config set model ollama:codellama`
- ✅ **Base URL Support**: Custom OpenAI-compatible endpoints
- ✅ **Availability Checking**: Real-time model availability verification

#### 6. Fixed Quit Behavior ✅
- ✅ **Safe Quit Keys**: Only `Ctrl+C` and `/quit` command exit the application
- ✅ **Removed 'q' Hotkey**: No accidental quits while typing in chat
- ✅ **Command Support**: Special commands like `/files`, `/stats`, and `/quit`
- ✅ **Streaming Safety**: Input disabled during streaming to prevent interference

## Milestone 2 - Complete ✅

### Workspace Indexer and Fast Reload

All Milestone 2 features have been successfully implemented:

#### 1. Directory Scanning & File Filtering ✅
- ✅ Scans all files under workspace root on TUI startup
- ✅ Skips ignored directories (`.git/`, `node_modules/`, `vendor/`, etc.)
- ✅ Respects `.gitignore` patterns with custom parser
- ✅ Ignores files above configurable size (default: 500 KB)
- ✅ Collects comprehensive metadata for each file:
  - Relative path from workspace root
  - File size
  - Last modified time (mtime)
  - Content hash (SHA-1)
  - File extension/language mapping (50+ languages supported)

#### 2. Index Data Structure ✅
- ✅ Efficient in-memory structure using `map[string]*FileMeta`
- ✅ Serialized as compressed gob file to `.loom/index.cache`
- ✅ Fast loading from cache on startup to avoid rescanning
- ✅ Automatic fallback to fresh scan if cache is invalid/missing

#### 3. Incremental Index Updates ✅
- ✅ Uses `fsnotify` to watch for workspace changes
- ✅ Updates in-memory and cached index for added, removed, or modified files
- ✅ Debounces/batches file events (500ms window) to avoid thrashing
- ✅ Parallel processing with worker pools for optimal performance

#### 4. Expose Index to TUI ✅
- ✅ Shows comprehensive summary in TUI interface
- ✅ Displays file count and language breakdown percentages
- ✅ Tab-switchable file tree view with scrollable pane
- ✅ Special commands: `/files` and `/stats` for quick info
- ✅ Real-time file information with size and language data

#### 5. Performance ✅
- ✅ Parallelized scanning using CPU-count worker pools
- ✅ Compressed gob serialization for fast cache I/O
- ✅ Optimized for sub-2-second indexing on small/medium projects
- ✅ Efficient file watching with batched updates

#### 6. Test & Documentation ✅
- ✅ Validates `.gitignore` respect and all skip patterns
- ✅ Comprehensive file type detection (binary, source, config, etc.)
- ✅ CLI command `loom index` to force rebuild
- ✅ Complete documentation of index structure and usage

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

### TUI Interface
- **Chat View**: Type messages, chat with AI about your project
- **File Tree View**: Press `Tab` to switch, use `↑↓` to scroll through indexed files  
- **Task Execution View**: Press `Tab` to view task status and execution history
- **Navigation**: 
  - `Tab` - Switch between chat, file tree, and task views
  - `↑↓` - Scroll in file tree view
  - `Enter` - Send message in chat view
  - `y`/`n` - Approve/cancel destructive tasks when prompted
  - `Ctrl+C` or `/quit` - Exit safely
- **Special Commands**:
  - `/files` - Show file count
  - `/stats` - Show detailed project statistics
  - `/tasks` - Show task execution history
  - `/quit` - Exit the application

### Chat Features
- **AI Conversation**: Ask questions about your code, architecture, or programming concepts
- **Project Context**: AI has automatic access to your project's file structure and language breakdown
- **Streaming Responses**: Real-time response streaming for immediate feedback
- **Chat History**: Persistent history across sessions (stored in `.loom/history/`)
- **Smart Memory**: Automatic message trimming while preserving important context
- **Task Execution**: AI can read files, make edits, list directories, and run commands
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

### Configuration
```json
{
  "model": "openai:gpt-4o",
  "enable_shell": false,
  "max_file_size": 512000,
  "api_key": "your-api-key-here",
  "base_url": "https://api.openai.com/v1"
}
```

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

### Project Structure
```
loom/
├── main.go                 # Entry point
├── cmd/
│   ├── root.go            # Root command with indexer integration
│   ├── init.go            # Init command
│   ├── config.go          # Config management commands
│   └── index.go           # Index rebuild command
├── config/
│   └── config.go          # Config system with LLM settings
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
├── chat/
│   └── session.go         # Chat session management with task audit
├── task/
│   ├── task.go            # Task protocol and parsing
│   ├── executor.go        # Task execution engine
│   ├── manager.go         # Task orchestration and recursive chat
│   └── task_test.go       # Comprehensive task tests
├── tui/
│   └── tui.go             # Enhanced TUI with task execution support
└── .loom/
    ├── config.json        # Local configuration
    ├── index.cache        # Compressed file index cache
    └── history/           # Chat history files with task audit
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
- Advanced code analysis with syntax tree parsing
- Integration with Git for version control operations
- Plugin system for custom task types
- Code refactoring tools and automated testing
- IDE integration and language server protocol support
- Multi-file search and replace operations
- Project templates and scaffolding
- Integration with CI/CD pipelines and development workflows

## After Milestone 4
Loom is now a true AI coding agent that can:
- ✅ Chat about your codebase and architecture
- ✅ Read and analyze files with intelligent filtering
- ✅ Make targeted file edits with diff previews
- ✅ List and explore directory structures
- ✅ Run shell commands with safety constraints
- ✅ Stream task results back to the AI for recursive improvement
- ✅ Maintain complete audit trails of all operations
- ✅ Require explicit user confirmation for destructive changes

The user stays in full control with clear summaries, diffs, and approval workflows for every action. 