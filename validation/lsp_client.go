package validation

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// LSPClient manages multiple language server instances
type LSPClient struct {
	servers       map[string]*LSPServer // language -> server instance
	config        *LSPConfig
	workspacePath string
	serverConfigs map[string]ServerConfig
	mutex         sync.RWMutex
}

// LSPServer represents a single language server instance
type LSPServer struct {
	cmd           *exec.Cmd
	stdin         io.WriteCloser
	stdout        io.ReadCloser
	stderr        io.ReadCloser
	language      string
	workspacePath string
	initialized   bool
	capabilities  *ServerCapabilities
	requestID     int64
	mutex         sync.RWMutex

	// Communication channels
	requests      chan *LSPRequest
	responses     chan *LSPResponse
	notifications chan *LSPNotification
	diagnostics   chan *LSPDiagnosticNotification
	shutdownCh    chan bool
	errorCh       chan error
}

// ServerConfig defines how to start and configure a language server
type ServerConfig struct {
	Command     string                 `json:"command"`
	Args        []string               `json:"args"`
	InitOptions map[string]interface{} `json:"init_options"`
	Environment []string               `json:"environment"`
	Enabled     bool                   `json:"enabled"`
	Description string                 `json:"description"`
}

// LSPConfig holds configuration for the LSP client
type LSPConfig struct {
	GlobalEnabled   bool                    `json:"global_enabled"`
	ServerConfigs   map[string]ServerConfig `json:"server_configs"`
	ValidationRules ValidationRules         `json:"validation_rules"`
	Performance     PerformanceConfig       `json:"performance"`
}

// ValidationRules define how to interpret LSP diagnostics
type ValidationRules struct {
	RollbackOnErrors []string `json:"rollback_on_errors"`
	IgnoreWarnings   []string `json:"ignore_warnings"`
	RequiredSeverity int      `json:"required_severity"` // 1=Error, 2=Warning, 3=Info, 4=Hint
}

// PerformanceConfig controls LSP performance settings
type PerformanceConfig struct {
	TimeoutSeconds  int  `json:"timeout_seconds"`
	EnableCaching   bool `json:"enable_caching"`
	CacheTTLMinutes int  `json:"cache_ttl_minutes"`
	MaxCacheSize    int  `json:"max_cache_size"`
}

// LSP Protocol Types

// LSPRequest represents an LSP request
type LSPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

// LSPResponse represents an LSP response
type LSPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *LSPError   `json:"error,omitempty"`
}

// LSPNotification represents an LSP notification (no response expected)
type LSPNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

// LSPError represents an LSP error
type LSPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// LSP-specific types

// InitializeParams for LSP initialize request
type InitializeParams struct {
	ProcessID             *int               `json:"processId"`
	RootPath              *string            `json:"rootPath,omitempty"`
	RootURI               *string            `json:"rootUri"`
	InitializationOptions interface{}        `json:"initializationOptions,omitempty"`
	Capabilities          ClientCapabilities `json:"capabilities"`
	Trace                 *string            `json:"trace,omitempty"`
	WorkspaceFolders      []WorkspaceFolder  `json:"workspaceFolders,omitempty"`
}

// ClientCapabilities defines what the client supports
type ClientCapabilities struct {
	Workspace    *WorkspaceClientCapabilities    `json:"workspace,omitempty"`
	TextDocument *TextDocumentClientCapabilities `json:"textDocument,omitempty"`
	Window       *WindowClientCapabilities       `json:"window,omitempty"`
	General      *GeneralClientCapabilities      `json:"general,omitempty"`
}

// WorkspaceClientCapabilities defines workspace-related capabilities
type WorkspaceClientCapabilities struct {
	ApplyEdit              *bool                               `json:"applyEdit,omitempty"`
	WorkspaceEdit          *WorkspaceEditClientCapabilities    `json:"workspaceEdit,omitempty"`
	DidChangeConfiguration *DidChangeConfigurationCapabilities `json:"didChangeConfiguration,omitempty"`
	DidChangeWatchedFiles  *DidChangeWatchedFilesCapabilities  `json:"didChangeWatchedFiles,omitempty"`
	Symbol                 *WorkspaceSymbolClientCapabilities  `json:"symbol,omitempty"`
	ExecuteCommand         *ExecuteCommandClientCapabilities   `json:"executeCommand,omitempty"`
}

// TextDocumentClientCapabilities defines text document capabilities
type TextDocumentClientCapabilities struct {
	Synchronization    *TextDocumentSyncClientCapabilities         `json:"synchronization,omitempty"`
	Completion         *CompletionClientCapabilities               `json:"completion,omitempty"`
	Hover              *HoverClientCapabilities                    `json:"hover,omitempty"`
	SignatureHelp      *SignatureHelpClientCapabilities            `json:"signatureHelp,omitempty"`
	Declaration        *DeclarationClientCapabilities              `json:"declaration,omitempty"`
	Definition         *DefinitionClientCapabilities               `json:"definition,omitempty"`
	TypeDefinition     *TypeDefinitionClientCapabilities           `json:"typeDefinition,omitempty"`
	Implementation     *ImplementationClientCapabilities           `json:"implementation,omitempty"`
	References         *ReferenceClientCapabilities                `json:"references,omitempty"`
	DocumentHighlight  *DocumentHighlightClientCapabilities        `json:"documentHighlight,omitempty"`
	DocumentSymbol     *DocumentSymbolClientCapabilities           `json:"documentSymbol,omitempty"`
	CodeAction         *CodeActionClientCapabilities               `json:"codeAction,omitempty"`
	CodeLens           *CodeLensClientCapabilities                 `json:"codeLens,omitempty"`
	DocumentLink       *DocumentLinkClientCapabilities             `json:"documentLink,omitempty"`
	ColorProvider      *DocumentColorClientCapabilities            `json:"colorProvider,omitempty"`
	Formatting         *DocumentFormattingClientCapabilities       `json:"formatting,omitempty"`
	RangeFormatting    *DocumentRangeFormattingClientCapabilities  `json:"rangeFormatting,omitempty"`
	OnTypeFormatting   *DocumentOnTypeFormattingClientCapabilities `json:"onTypeFormatting,omitempty"`
	Rename             *RenameClientCapabilities                   `json:"rename,omitempty"`
	PublishDiagnostics *PublishDiagnosticsClientCapabilities       `json:"publishDiagnostics,omitempty"`
	FoldingRange       *FoldingRangeClientCapabilities             `json:"foldingRange,omitempty"`
}

// WindowClientCapabilities defines window-related capabilities
type WindowClientCapabilities struct {
	WorkDoneProgress *bool `json:"workDoneProgress,omitempty"`
}

// GeneralClientCapabilities defines general capabilities
type GeneralClientCapabilities struct {
	RegularExpressions *RegularExpressionsClientCapabilities `json:"regularExpressions,omitempty"`
	Markdown           *MarkdownClientCapabilities           `json:"markdown,omitempty"`
}

// ServerCapabilities defines what the server supports (received in initialize response)
type ServerCapabilities struct {
	TextDocumentSync                 interface{}                      `json:"textDocumentSync,omitempty"`
	CompletionProvider               *CompletionOptions               `json:"completionProvider,omitempty"`
	HoverProvider                    interface{}                      `json:"hoverProvider,omitempty"`
	SignatureHelpProvider            *SignatureHelpOptions            `json:"signatureHelpProvider,omitempty"`
	DeclarationProvider              interface{}                      `json:"declarationProvider,omitempty"`
	DefinitionProvider               interface{}                      `json:"definitionProvider,omitempty"`
	TypeDefinitionProvider           interface{}                      `json:"typeDefinitionProvider,omitempty"`
	ImplementationProvider           interface{}                      `json:"implementationProvider,omitempty"`
	ReferencesProvider               interface{}                      `json:"referencesProvider,omitempty"`
	DocumentHighlightProvider        interface{}                      `json:"documentHighlightProvider,omitempty"`
	DocumentSymbolProvider           interface{}                      `json:"documentSymbolProvider,omitempty"`
	CodeActionProvider               interface{}                      `json:"codeActionProvider,omitempty"`
	CodeLensProvider                 *CodeLensOptions                 `json:"codeLensProvider,omitempty"`
	DocumentLinkProvider             *DocumentLinkOptions             `json:"documentLinkProvider,omitempty"`
	ColorProvider                    interface{}                      `json:"colorProvider,omitempty"`
	DocumentFormattingProvider       interface{}                      `json:"documentFormattingProvider,omitempty"`
	DocumentRangeFormattingProvider  interface{}                      `json:"documentRangeFormattingProvider,omitempty"`
	DocumentOnTypeFormattingProvider *DocumentOnTypeFormattingOptions `json:"documentOnTypeFormattingProvider,omitempty"`
	RenameProvider                   interface{}                      `json:"renameProvider,omitempty"`
	FoldingRangeProvider             interface{}                      `json:"foldingRangeProvider,omitempty"`
	ExecuteCommandProvider           *ExecuteCommandOptions           `json:"executeCommandProvider,omitempty"`
	SelectionRangeProvider           interface{}                      `json:"selectionRangeProvider,omitempty"`
	WorkspaceSymbolProvider          interface{}                      `json:"workspaceSymbolProvider,omitempty"`
	Workspace                        *WorkspaceServerCapabilities     `json:"workspace,omitempty"`
	Experimental                     interface{}                      `json:"experimental,omitempty"`
}

// WorkspaceFolder represents a workspace folder
type WorkspaceFolder struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

// LSPDiagnosticNotification represents textDocument/publishDiagnostics notification
type LSPDiagnosticNotification struct {
	URI         string          `json:"uri"`
	Version     *int            `json:"version"`
	Diagnostics []LSPDiagnostic `json:"diagnostics"`
}

// Position represents a position in a text document
type Position struct {
	Line      int `json:"line"`      // 0-based
	Character int `json:"character"` // 0-based
}

// Range represents a range in a text document
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// LSPDiagnostic represents a diagnostic (error, warning, etc.)
type LSPDiagnostic struct {
	Range              Range                   `json:"range"`
	Severity           *int                    `json:"severity,omitempty"` // 1=Error, 2=Warning, 3=Info, 4=Hint
	Code               interface{}             `json:"code,omitempty"`
	CodeDescription    *CodeDescription        `json:"codeDescription,omitempty"`
	Source             *string                 `json:"source,omitempty"`
	Message            string                  `json:"message"`
	Tags               []int                   `json:"tags,omitempty"`
	RelatedInformation []DiagnosticRelatedInfo `json:"relatedInformation,omitempty"`
	Data               interface{}             `json:"data,omitempty"`
}

// CodeDescription provides additional information about a diagnostic code
type CodeDescription struct {
	Href string `json:"href"`
}

// DiagnosticRelatedInfo represents related information for a diagnostic
type DiagnosticRelatedInfo struct {
	Location Location `json:"location"`
	Message  string   `json:"message"`
}

// Location represents a location in a text document
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// Default server configurations
var DefaultServerConfigs = map[string]ServerConfig{
	"go": {
		Command:     "gopls",
		Args:        []string{"serve"},
		Enabled:     true,
		Description: "Go language server (gopls)",
	},
	"typescript": {
		Command:     "typescript-language-server",
		Args:        []string{"--stdio"},
		Enabled:     false, // Requires manual installation
		Description: "TypeScript language server",
	},
	"javascript": {
		Command:     "typescript-language-server",
		Args:        []string{"--stdio"},
		Enabled:     false, // Requires manual installation
		Description: "JavaScript language server (via TypeScript)",
	},
	"python": {
		Command:     "pylsp",
		Args:        []string{},
		Enabled:     false, // Requires manual installation
		Description: "Python LSP server",
	},
	"rust": {
		Command:     "rust-analyzer",
		Args:        []string{},
		Enabled:     false, // Requires manual installation
		Description: "Rust language server",
	},
}

// DefaultLSPConfig returns default LSP configuration
func DefaultLSPConfig() *LSPConfig {
	return &LSPConfig{
		GlobalEnabled: false, // Start disabled by default
		ServerConfigs: DefaultServerConfigs,
		ValidationRules: ValidationRules{
			RollbackOnErrors: []string{
				"syntax error",
				"unexpected token",
				"parse error",
				"invalid syntax",
			},
			IgnoreWarnings:   []string{},
			RequiredSeverity: 1, // Only rollback on errors
		},
		Performance: PerformanceConfig{
			TimeoutSeconds:  10,
			EnableCaching:   true,
			CacheTTLMinutes: 30,
			MaxCacheSize:    1000,
		},
	}
}

// Various capability structures (simplified for now)
type WorkspaceEditClientCapabilities struct{}
type DidChangeConfigurationCapabilities struct{}
type DidChangeWatchedFilesCapabilities struct{}
type WorkspaceSymbolClientCapabilities struct{}
type ExecuteCommandClientCapabilities struct{}
type TextDocumentSyncClientCapabilities struct{}
type CompletionClientCapabilities struct{}
type HoverClientCapabilities struct{}
type SignatureHelpClientCapabilities struct{}
type DeclarationClientCapabilities struct{}
type DefinitionClientCapabilities struct{}
type TypeDefinitionClientCapabilities struct{}
type ImplementationClientCapabilities struct{}
type ReferenceClientCapabilities struct{}
type DocumentHighlightClientCapabilities struct{}
type DocumentSymbolClientCapabilities struct{}
type CodeActionClientCapabilities struct{}
type CodeLensClientCapabilities struct{}
type DocumentLinkClientCapabilities struct{}
type DocumentColorClientCapabilities struct{}
type DocumentFormattingClientCapabilities struct{}
type DocumentRangeFormattingClientCapabilities struct{}
type DocumentOnTypeFormattingClientCapabilities struct{}
type RenameClientCapabilities struct{}
type PublishDiagnosticsClientCapabilities struct{}
type FoldingRangeClientCapabilities struct{}
type RegularExpressionsClientCapabilities struct{}
type MarkdownClientCapabilities struct{}
type CompletionOptions struct{}
type SignatureHelpOptions struct{}
type CodeLensOptions struct{}
type DocumentLinkOptions struct{}
type DocumentOnTypeFormattingOptions struct{}
type ExecuteCommandOptions struct{}
type WorkspaceServerCapabilities struct{}

// NewLSPClient creates a new LSP client
func NewLSPClient(workspacePath string, config *LSPConfig) *LSPClient {
	if config == nil {
		config = DefaultLSPConfig()
	}

	return &LSPClient{
		servers:       make(map[string]*LSPServer),
		config:        config,
		workspacePath: workspacePath,
		serverConfigs: config.ServerConfigs,
	}
}

// GetOrStartServer gets an existing server or starts a new one for the given language
func (c *LSPClient) GetOrStartServer(language string) (*LSPServer, error) {
	c.mutex.RLock()
	if server, exists := c.servers[language]; exists && server.IsHealthy() {
		c.mutex.RUnlock()
		return server, nil
	}
	c.mutex.RUnlock()

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Double-check after acquiring write lock
	if server, exists := c.servers[language]; exists && server.IsHealthy() {
		return server, nil
	}

	// Start new server
	server, err := c.startServer(language)
	if err != nil {
		return nil, fmt.Errorf("failed to start %s language server: %w", language, err)
	}

	c.servers[language] = server
	return server, nil
}

// startServer starts a new language server process
func (c *LSPClient) startServer(language string) (*LSPServer, error) {
	config, exists := c.serverConfigs[language]
	if !exists {
		return nil, fmt.Errorf("no configuration found for language: %s", language)
	}

	if !config.Enabled {
		return nil, fmt.Errorf("language server for %s is disabled", language)
	}

	// Check if command exists
	if _, err := exec.LookPath(config.Command); err != nil {
		return nil, fmt.Errorf("language server command '%s' not found: %w", config.Command, err)
	}

	// Create command
	cmd := exec.Command(config.Command, config.Args...)
	cmd.Dir = c.workspacePath

	// Set up environment
	if len(config.Environment) > 0 {
		cmd.Env = append(cmd.Env, config.Environment...)
	}

	// Get pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return nil, fmt.Errorf("failed to start language server process: %w", err)
	}

	server := &LSPServer{
		cmd:           cmd,
		stdin:         stdin,
		stdout:        stdout,
		stderr:        stderr,
		language:      language,
		workspacePath: c.workspacePath,
		requestID:     0,

		requests:      make(chan *LSPRequest, 100),
		responses:     make(chan *LSPResponse, 100),
		notifications: make(chan *LSPNotification, 100),
		diagnostics:   make(chan *LSPDiagnosticNotification, 100),
		shutdownCh:    make(chan bool, 1),
		errorCh:       make(chan error, 10),
	}

	// Start communication goroutines
	go server.readMessages()
	go server.writeMessages()
	go server.handleMessages()

	// Initialize the server
	if err := server.initialize(); err != nil {
		server.Shutdown()
		return nil, fmt.Errorf("failed to initialize language server: %w", err)
	}

	return server, nil
}

// ValidateFile validates a file using LSP
func (c *LSPClient) ValidateFile(filePath string) (*ValidationResult, error) {
	if !c.config.GlobalEnabled {
		return &ValidationResult{
			IsValid:       true,
			ValidatorUsed: "disabled",
		}, nil
	}

	// Detect language
	language := DetectLanguage(filePath)
	if language == "text" {
		return &ValidationResult{
			IsValid:       true,
			ValidatorUsed: "unsupported",
		}, nil
	}

	// Get or start server
	server, err := c.GetOrStartServer(language)
	if err != nil {
		return &ValidationResult{
			IsValid:       true, // Assume valid if server unavailable
			ValidatorUsed: "unavailable",
		}, nil
	}

	// Validate with timeout
	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(c.config.Performance.TimeoutSeconds)*time.Second)
	defer cancel()

	return server.ValidateFile(ctx, filePath)
}

// Shutdown shuts down all language servers
func (c *LSPClient) Shutdown() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for language, server := range c.servers {
		fmt.Printf("Shutting down %s language server...\n", language)
		server.Shutdown()
	}

	c.servers = make(map[string]*LSPServer)
}

// LSPServer methods

// IsHealthy checks if the server is healthy and responsive
func (s *LSPServer) IsHealthy() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.initialized && s.cmd != nil && s.cmd.Process != nil
}

// GetNextRequestID returns the next request ID
func (s *LSPServer) GetNextRequestID() int64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.requestID++
	return s.requestID
}

// initialize sends the LSP initialize request
func (s *LSPServer) initialize() error {
	workspaceURI := "file://" + s.workspacePath

	params := InitializeParams{
		ProcessID: func() *int { pid := s.cmd.Process.Pid; return &pid }(),
		RootURI:   &workspaceURI,
		Capabilities: ClientCapabilities{
			TextDocument: &TextDocumentClientCapabilities{
				PublishDiagnostics: &PublishDiagnosticsClientCapabilities{},
				Synchronization:    &TextDocumentSyncClientCapabilities{},
			},
		},
		WorkspaceFolders: []WorkspaceFolder{
			{
				URI:  workspaceURI,
				Name: filepath.Base(s.workspacePath),
			},
		},
	}

	request := &LSPRequest{
		JSONRPC: "2.0",
		ID:      s.GetNextRequestID(),
		Method:  "initialize",
		Params:  params,
	}

	// Send initialize request
	s.requests <- request

	// Wait for initialize response with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	select {
	case response := <-s.responses:
		if response.Error != nil {
			return fmt.Errorf("initialize failed: %s", response.Error.Message)
		}

		// Parse server capabilities
		if response.Result != nil {
			if resultMap, ok := response.Result.(map[string]interface{}); ok {
				if caps, exists := resultMap["capabilities"]; exists {
					s.capabilities = parseServerCapabilities(caps)
				}
			}
		}

		s.mutex.Lock()
		s.initialized = true
		s.mutex.Unlock()

		// Send initialized notification
		notification := &LSPNotification{
			JSONRPC: "2.0",
			Method:  "initialized",
			Params:  map[string]interface{}{},
		}
		s.notifications <- notification

		return nil

	case err := <-s.errorCh:
		return fmt.Errorf("initialize error: %w", err)

	case <-ctx.Done():
		return fmt.Errorf("initialize timeout")
	}
}

// parseServerCapabilities parses server capabilities from initialize response
func parseServerCapabilities(caps interface{}) *ServerCapabilities {
	// For now, return minimal capabilities
	// TODO: Implement full parsing if needed
	return &ServerCapabilities{}
}

// ValidateFile validates a file and returns diagnostics
func (s *LSPServer) ValidateFile(ctx context.Context, filePath string) (*ValidationResult, error) {
	if !s.IsHealthy() {
		return nil, fmt.Errorf("server is not healthy")
	}

	// Read file content
	content, err := readFileContent(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	fileURI := "file://" + filePath

	// Send textDocument/didOpen notification
	didOpenParams := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        fileURI,
			"languageId": s.language,
			"version":    1,
			"text":       content,
		},
	}

	notification := &LSPNotification{
		JSONRPC: "2.0",
		Method:  "textDocument/didOpen",
		Params:  didOpenParams,
	}

	s.notifications <- notification

	// Wait for diagnostics with timeout
	select {
	case diagnosticNotification := <-s.diagnostics:
		if diagnosticNotification.URI == fileURI {
			return s.processDiagnostics(diagnosticNotification.Diagnostics), nil
		}

	case <-ctx.Done():
		return nil, fmt.Errorf("validation timeout")

	case <-time.After(5 * time.Second):
		// No diagnostics received - assume file is valid
		return &ValidationResult{
			IsValid:       true,
			Errors:        []LSPDiagnostic{},
			Warnings:      []LSPDiagnostic{},
			Language:      s.language,
			ValidatorUsed: "lsp",
		}, nil
	}

	return nil, fmt.Errorf("unexpected response")
}

// processDiagnostics converts LSP diagnostics to ValidationResult
func (s *LSPServer) processDiagnostics(diagnostics []LSPDiagnostic) *ValidationResult {
	var errors, warnings, hints []LSPDiagnostic

	for _, diag := range diagnostics {
		switch {
		case diag.Severity == nil || *diag.Severity == 1:
			errors = append(errors, diag)
		case *diag.Severity == 2:
			warnings = append(warnings, diag)
		case *diag.Severity == 3 || *diag.Severity == 4:
			hints = append(hints, diag)
		}
	}

	return &ValidationResult{
		IsValid:       len(errors) == 0,
		Errors:        errors,
		Warnings:      warnings,
		Hints:         hints,
		Language:      s.language,
		ValidatorUsed: "lsp",
	}
}

// Shutdown gracefully shuts down the language server
func (s *LSPServer) Shutdown() {
	if !s.IsHealthy() {
		return
	}

	// Send shutdown request
	request := &LSPRequest{
		JSONRPC: "2.0",
		ID:      s.GetNextRequestID(),
		Method:  "shutdown",
		Params:  nil,
	}

	s.requests <- request

	// Send exit notification
	notification := &LSPNotification{
		JSONRPC: "2.0",
		Method:  "exit",
		Params:  nil,
	}

	s.notifications <- notification

	// Signal shutdown
	s.shutdownCh <- true

	// Wait a bit for graceful shutdown
	time.Sleep(100 * time.Millisecond)

	// Force kill if still running
	if s.cmd.Process != nil {
		s.cmd.Process.Kill()
	}

	// Close pipes
	if s.stdin != nil {
		s.stdin.Close()
	}
	if s.stdout != nil {
		s.stdout.Close()
	}
	if s.stderr != nil {
		s.stderr.Close()
	}
}

// readFileContent reads file content, handling different encodings
func readFileContent(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// JSON-RPC Communication Methods

// readMessages reads messages from the language server stdout
func (s *LSPServer) readMessages() {
	defer func() {
		if r := recover(); r != nil {
			s.errorCh <- fmt.Errorf("readMessages panic: %v", r)
		}
	}()

	scanner := bufio.NewScanner(s.stdout)
	var contentLength int
	var inHeaders bool = true

	for scanner.Scan() {
		select {
		case <-s.shutdownCh:
			return
		default:
		}

		line := scanner.Text()

		if inHeaders {
			if line == "" {
				// End of headers, read content
				inHeaders = false
				if contentLength > 0 {
					content := make([]byte, contentLength)
					if _, err := io.ReadFull(s.stdout, content); err != nil {
						s.errorCh <- fmt.Errorf("failed to read message content: %w", err)
						continue
					}

					if err := s.processMessage(content); err != nil {
						s.errorCh <- fmt.Errorf("failed to process message: %w", err)
					}

					contentLength = 0
					inHeaders = true
				}
			} else if strings.HasPrefix(line, "Content-Length: ") {
				lengthStr := strings.TrimPrefix(line, "Content-Length: ")
				var err error
				contentLength, err = strconv.Atoi(lengthStr)
				if err != nil {
					s.errorCh <- fmt.Errorf("invalid Content-Length: %s", lengthStr)
					continue
				}
			}
			// Ignore other headers
		}
	}

	if err := scanner.Err(); err != nil {
		s.errorCh <- fmt.Errorf("scanner error: %w", err)
	}
}

// processMessage processes a received JSON-RPC message
func (s *LSPServer) processMessage(content []byte) error {
	// Try to parse as response first (has ID)
	var response LSPResponse
	if err := json.Unmarshal(content, &response); err == nil && response.ID != 0 {
		s.responses <- &response
		return nil
	}

	// Try to parse as notification
	var notification LSPNotification
	if err := json.Unmarshal(content, &notification); err == nil {
		// Handle specific notifications
		switch notification.Method {
		case "textDocument/publishDiagnostics":
			if diagNotif, err := s.parseDiagnosticNotification(content); err == nil {
				s.diagnostics <- diagNotif
			}
		}
		return nil
	}

	return fmt.Errorf("failed to parse message: %s", string(content))
}

// parseDiagnosticNotification parses a publishDiagnostics notification
func (s *LSPServer) parseDiagnosticNotification(content []byte) (*LSPDiagnosticNotification, error) {
	var notification struct {
		JSONRPC string                    `json:"jsonrpc"`
		Method  string                    `json:"method"`
		Params  LSPDiagnosticNotification `json:"params"`
	}

	if err := json.Unmarshal(content, &notification); err != nil {
		return nil, err
	}

	return &notification.Params, nil
}

// writeMessages writes messages to the language server stdin
func (s *LSPServer) writeMessages() {
	defer func() {
		if r := recover(); r != nil {
			s.errorCh <- fmt.Errorf("writeMessages panic: %v", r)
		}
	}()

	for {
		select {
		case request := <-s.requests:
			if err := s.sendMessage(request); err != nil {
				s.errorCh <- fmt.Errorf("failed to send request: %w", err)
			}

		case notification := <-s.notifications:
			if err := s.sendMessage(notification); err != nil {
				s.errorCh <- fmt.Errorf("failed to send notification: %w", err)
			}

		case <-s.shutdownCh:
			return
		}
	}
}

// sendMessage sends a JSON-RPC message using the LSP protocol format
func (s *LSPServer) sendMessage(message interface{}) error {
	content, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// LSP uses Content-Length header followed by the JSON content
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(content))

	if _, err := s.stdin.Write([]byte(header)); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	if _, err := s.stdin.Write(content); err != nil {
		return fmt.Errorf("failed to write content: %w", err)
	}

	return nil
}

// handleMessages handles errors and manages server state
func (s *LSPServer) handleMessages() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("handleMessages panic in %s server: %v\n", s.language, r)
		}
	}()

	for {
		select {
		case err := <-s.errorCh:
			fmt.Printf("LSP server error (%s): %v\n", s.language, err)
			// Could implement retry logic here

		case <-s.shutdownCh:
			return
		}
	}
}
