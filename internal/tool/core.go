package tool

import (
	"log"

	"github.com/loom/loom/internal/indexer"
)

// RegisterCoreTools registers all core tools with the registry for the given workspace.
// This centralizes tool registration to avoid duplication across main.go, SetWorkspace, and ReloadMCP.
func RegisterCoreTools(registry *Registry, workspacePath string) {
	// Create indexer
	idx := indexer.NewRipgrepIndexer(workspacePath)

	// Register file tools
	if err := RegisterReadFile(registry, workspacePath); err != nil {
		log.Printf("Failed to register read_file tool: %v", err)
	}

	if err := RegisterSearchCode(registry, idx); err != nil {
		log.Printf("Failed to register search_code tool: %v", err)
	}

	if err := RegisterEditFile(registry, workspacePath); err != nil {
		log.Printf("Failed to register edit_file tool: %v", err)
	}

	if err := RegisterApplyEdit(registry, workspacePath); err != nil {
		log.Printf("Failed to register apply_edit tool: %v", err)
	}

	if err := RegisterListDir(registry, workspacePath); err != nil {
		log.Printf("Failed to register list_dir tool: %v", err)
	}

	// Shell tools
	if err := RegisterRunShell(registry, workspacePath); err != nil {
		log.Printf("Failed to register run_shell tool: %v", err)
	}
	if err := RegisterApplyShell(registry, workspacePath); err != nil {
		log.Printf("Failed to register apply_shell tool: %v", err)
	}

	// Git tools
	if err := RegisterGitTools(registry, workspacePath); err != nil {
		log.Printf("Failed to register git tools: %v", err)
	}

	// HTTP request tool (workspace-independent)
	if err := RegisterHTTPRequest(registry); err != nil {
		log.Printf("Failed to register http_request tool: %v", err)
	}

	// User-scoped tools (workspace-independent)
	if err := RegisterMemories(registry); err != nil {
		log.Printf("Failed to register memories tool: %v", err)
	}

	if err := RegisterTodoList(registry); err != nil {
		log.Printf("Failed to register todo_list tool: %v", err)
	}

	if err := RegisterUserChoice(registry); err != nil {
		log.Printf("Failed to register user_choice tool: %v", err)
	}

	// Project profile tools
	if err := RegisterProjectProfileTools(registry, workspacePath); err != nil {
		log.Printf("Failed to register project profile tools: %v", err)
	}
}
