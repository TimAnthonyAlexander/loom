package common

import (
	"strings"
)

// ValidateToolArgs applies minimal, tool-specific validation for required fields
// to avoid prematurely emitting incomplete tool calls during streaming.
// This replaces identical implementations across all adapter clients.
func ValidateToolArgs(toolName string, args map[string]interface{}) bool {
	switch toolName {
	case "read_file":
		// require path
		if v, ok := args["path"]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return true
			}
		}
		return false
	case "search_code":
		if v, ok := args["query"]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return true
			}
		}
		return false
	case "edit_file", "apply_edit":
		if v, ok := args["path"]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return true
			}
		}
		return false
	default:
		// For other tools, accept any JSON (including empty) by default
		return true
	}
}

// IsEmptyResponse checks if a response is effectively empty (only whitespace).
// This replaces identical implementations across all adapter clients.
func IsEmptyResponse(content string) bool {
	return strings.TrimSpace(content) == ""
}
