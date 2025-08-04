package loom_edit

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// EditCommand represents a parsed LOOM_EDIT command
type EditCommand struct {
	File      string // The target file path
	Action    string // REPLACE, INSERT_AFTER, INSERT_BEFORE, DELETE, SEARCH_REPLACE, or CREATE
	Start     int    // 1-based inclusive start line number
	End       int    // 1-based inclusive end line number
	NewText   string // The replacement/insertion text
	OldString string // For SEARCH_REPLACE: the string to search for
	NewString string // For SEARCH_REPLACE: the string to replace with
}

// ParseEditCommand parses a LOOM_EDIT command block into an EditCommand struct
func ParseEditCommand(input string) (*EditCommand, error) {
	// Normalize line endings and ensure input is not empty
	input = strings.ReplaceAll(input, "\r\n", "\n")
	input = strings.ReplaceAll(input, "\r", "\n")

	if strings.TrimSpace(input) == "" {
		return nil, fmt.Errorf("empty LOOM_EDIT command")
	}

	lines := strings.Split(input, "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("invalid LOOM_EDIT format: too few lines")
	}

	// Parse header line
	headerLine := lines[0]

	// Base pattern for all commands - try strict format first
	basePattern := regexp.MustCompile(`^(?:>>|🔧 )LOOM_EDIT file=([^\s]+) (REPLACE|INSERT_AFTER|INSERT_BEFORE|DELETE|SEARCH_REPLACE|CREATE)`)
	baseMatches := basePattern.FindStringSubmatch(headerLine)

	// If strict format fails, try without prefix (and warn)
	if baseMatches == nil {
		fallbackPattern := regexp.MustCompile(`^LOOM_EDIT file=([^\s]+) (REPLACE|INSERT_AFTER|INSERT_BEFORE|DELETE|SEARCH_REPLACE|CREATE)`)
		baseMatches = fallbackPattern.FindStringSubmatch(headerLine)
		
		if baseMatches != nil {
			fmt.Printf("Warning: LOOM_EDIT command missing >> prefix. Please use: >>LOOM_EDIT ...\n")
		} else {
			return nil, fmt.Errorf("invalid LOOM_EDIT header format: %s", headerLine)
		}
	}

	filePath := baseMatches[1]
	action := baseMatches[2]

	cmd := &EditCommand{
		File:   filePath,
		Action: action,
	}

	// Check for closing tag
	var foundClosingTag bool
	var closeIndex int

	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "<<LOOM_EDIT" {
			foundClosingTag = true
			closeIndex = i
			break
		}
	}

	if !foundClosingTag {
		return nil, fmt.Errorf("invalid LOOM_EDIT format: missing closing <<LOOM_EDIT tag")
	}

	// Handle SEARCH_REPLACE specially
	if action == "SEARCH_REPLACE" {
		// Try to handle simple case first (single line quotes)
		searchReplacePattern := regexp.MustCompile(`SEARCH_REPLACE\s+"([^"]+)"\s+"([^"]+)"`)
		srMatches := searchReplacePattern.FindStringSubmatch(headerLine)

		if srMatches != nil && len(srMatches) >= 3 {
			cmd.OldString = srMatches[1]
			cmd.NewString = srMatches[2]
		} else {
			// More complex case: try to find quotes in the content
			fullText := strings.Join(lines[:closeIndex+1], "\n")

			// Try to extract the quoted strings
			var oldString, newString string
			var oldStart, oldEnd, newStart, newEnd = -1, -1, -1, -1

			// Find position of "SEARCH_REPLACE"
			srPos := strings.Index(fullText, "SEARCH_REPLACE")
			if srPos == -1 {
				return nil, fmt.Errorf("SEARCH_REPLACE keyword not found in command")
			}

			// Find first quote after SEARCH_REPLACE
			firstQuote := strings.Index(fullText[srPos:], "\"")
			if firstQuote == -1 {
				return nil, fmt.Errorf("missing first quote for SEARCH_REPLACE old string")
			}

			oldStart = srPos + firstQuote + 1

			// Find matching closing quote
			for i := oldStart; i < len(fullText); i++ {
				if fullText[i] == '"' && (i == 0 || fullText[i-1] != '\\') {
					oldEnd = i
					break
				}
			}

			if oldEnd == -1 {
				return nil, fmt.Errorf("missing closing quote for SEARCH_REPLACE old string")
			}

			// Find second opening quote
			newStart = -1
			for i := oldEnd + 1; i < len(fullText); i++ {
				if fullText[i] == '"' && (i == 0 || fullText[i-1] != '\\') {
					newStart = i + 1
					break
				}
			}

			if newStart == -1 {
				return nil, fmt.Errorf("missing opening quote for SEARCH_REPLACE new string")
			}

			// Find second closing quote
			for i := newStart; i < len(fullText); i++ {
				if fullText[i] == '"' && (i == 0 || fullText[i-1] != '\\') {
					newEnd = i
					break
				}
			}

			if newEnd == -1 {
				return nil, fmt.Errorf("missing closing quote for SEARCH_REPLACE new string")
			}

			oldString = fullText[oldStart:oldEnd]
			newString = fullText[newStart:newEnd]

			// Unescape any escaped quotes
			oldString = strings.ReplaceAll(oldString, "\\\"", "\"")
			newString = strings.ReplaceAll(newString, "\\\"", "\"")

			cmd.OldString = oldString
			cmd.NewString = newString
		}

		// Default values for line numbers (not used for SEARCH_REPLACE)
		cmd.Start = 1
		cmd.End = 1
	} else if action == "CREATE" {
		// For CREATE action, we don't need line numbers
		// Extract new text (everything between header line and <<LOOM_EDIT)
		var newTextLines []string

		for i := 1; i < closeIndex; i++ {
			newTextLines = append(newTextLines, lines[i])
		}

		cmd.NewText = strings.Join(newTextLines, "\n")
		// Set start and end to 1 for consistency
		cmd.Start = 1
		cmd.End = 1
	} else {
		// For other actions, look for line numbers
		linePattern := regexp.MustCompile(`\s+(\d+)(?:-(\d+))?`)
		lineMatches := linePattern.FindStringSubmatch(headerLine)

		if lineMatches == nil {
			return nil, fmt.Errorf("missing line number in %s action", action)
		}

		// Parse start line
		start, err := strconv.Atoi(lineMatches[1])
		if err != nil {
			return nil, fmt.Errorf("invalid start line number: %v", err)
		}
		if start < 1 {
			return nil, fmt.Errorf("line numbers must be >= 1, got start=%d", start)
		}
		cmd.Start = start

		// Parse end line if present, otherwise default to start line
		if len(lineMatches) > 2 && lineMatches[2] != "" {
			end, err := strconv.Atoi(lineMatches[2])
			if err != nil {
				return nil, fmt.Errorf("invalid end line number: %v", err)
			}
			if end < start {
				return nil, fmt.Errorf("end line (%d) must be >= start line (%d)", end, start)
			}
			cmd.End = end
		} else {
			cmd.End = start
			// Warn for actions that typically have a range
			if action == "REPLACE" || action == "DELETE" {
				fmt.Printf("Warning: %s action missing end line number, defaulting to %d\n", action, start)
			}
		}

		// Extract new text (everything between header line and <<LOOM_EDIT)
		var newTextLines []string

		for i := 1; i < closeIndex; i++ {
			newTextLines = append(newTextLines, lines[i])
		}

		// For DELETE action, newText should be empty
		if action == "DELETE" && strings.TrimSpace(strings.Join(newTextLines, "")) != "" {
			fmt.Printf("Warning: DELETE action should have empty content block, ignoring provided content\n")
			newTextLines = nil
		}

		cmd.NewText = strings.Join(newTextLines, "\n")
	}

	return cmd, nil
}

// ApplyEdit applies an EditCommand to a file
func ApplyEdit(filePath string, cmd *EditCommand) error {
	// Validate the command
	if cmd == nil {
		return fmt.Errorf("cannot apply nil command")
	}

	if cmd.File == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	if cmd.Action == "" {
		return fmt.Errorf("action cannot be empty")
	}

	if cmd.Start < 1 {
		return fmt.Errorf("invalid start line: %d (must be >= 1)", cmd.Start)
	}

	if cmd.End < cmd.Start {
		return fmt.Errorf("invalid line range: end (%d) cannot be less than start (%d)", cmd.End, cmd.Start)
	}

	// Special handling for CREATE action
	if cmd.Action == "CREATE" {
		// Check if file already exists
		if _, err := os.Stat(filePath); err == nil {
			return fmt.Errorf("cannot CREATE file that already exists: %s", filePath)
		}

		// Create directory if it doesn't exist
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory for new file: %v", err)
		}

		// Write the new content directly
		if err := ioutil.WriteFile(filePath, []byte(cmd.NewText), 0644); err != nil {
			return fmt.Errorf("failed to create new file: %v", err)
		}

		fmt.Printf("Created new file: %s\n", filePath)
		return nil
	}

	// Special handling for SEARCH_REPLACE
	if cmd.Action == "SEARCH_REPLACE" {
		if cmd.OldString == "" {
			return fmt.Errorf("SEARCH_REPLACE requires non-empty old string")
		}

		// Read the current file content
		content, err := ioutil.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %v", err)
		}

		contentStr := string(content)

		// Normalize line endings for better matching
		normalizedContent := strings.ReplaceAll(contentStr, "\r\n", "\n")
		normalizedOldString := strings.ReplaceAll(cmd.OldString, "\r\n", "\n")
		normalizedOldString = strings.ReplaceAll(normalizedOldString, "\r", "\n")

		// Check if the old string exists in the file
		if !strings.Contains(normalizedContent, normalizedOldString) {
			return fmt.Errorf("SEARCH_REPLACE failed: old string not found in file")
		}

		// Perform the replacement
		newContent := strings.Replace(normalizedContent, normalizedOldString, cmd.NewString, -1)

		// Check if any replacements were made
		if newContent == normalizedContent {
			fmt.Printf("Warning: SEARCH_REPLACE didn't change anything (old and new strings may be identical)\n")
		}

		// Write the modified content back to file
		err = ioutil.WriteFile(filePath, []byte(newContent), 0644)
		if err != nil {
			return fmt.Errorf("failed to write file: %v", err)
		}

		// Count occurrences for reporting
		occurrences := strings.Count(normalizedContent, normalizedOldString)
		fmt.Printf("Replaced %d occurrence(s) of the specified string\n", occurrences)

		return nil
	}

	// Read the current file content
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		// Special handling for file not found when using REPLACE
		if os.IsNotExist(err) && cmd.Action == "REPLACE" {
			// We'll create a new file with the content
			fmt.Printf("File %s not found, will create new file\n", filePath)

			// For new files, ensure directory exists
			dir := filepath.Dir(filePath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory for new file: %v", err)
			}

			// Write the new content directly
			if err := ioutil.WriteFile(filePath, []byte(cmd.NewText), 0644); err != nil {
				return fmt.Errorf("failed to create new file: %v", err)
			}

			return nil
		}
		return fmt.Errorf("failed to read file: %v", err)
	}

	// Normalize line endings to LF and detect trailing newline
	contentStr := strings.ReplaceAll(string(content), "\r\n", "\n")
	contentStr = strings.ReplaceAll(contentStr, "\r", "\n")
	hasFinalNewline := strings.HasSuffix(contentStr, "\n")

	// Split content into lines (using normalized content)
	lines := strings.Split(contentStr, "\n")
	if hasFinalNewline && len(lines) > 0 && lines[len(lines)-1] == "" {
		// Remove trailing empty line (artifact of splitting on newline)
		lines = lines[:len(lines)-1]
	}

	// Normalize newlines in the new text
	normalizedNewText := strings.ReplaceAll(cmd.NewText, "\r\n", "\n")
	normalizedNewText = strings.ReplaceAll(normalizedNewText, "\r", "\n")

	// Validate line range
	if cmd.Start < 1 || cmd.Start > len(lines) {
		return fmt.Errorf("start line %d is out of range (1-%d)", cmd.Start, len(lines))
	}
	if cmd.End < cmd.Start || cmd.End > len(lines) {
		return fmt.Errorf("end line %d is out of range (%d-%d)", cmd.End, cmd.Start, len(lines))
	}

	// Apply the edit based on action
	var newLines []string

	switch cmd.Action {
	case "REPLACE":
		// Replace lines Start through End with NewText
		newLines = append(newLines, lines[:cmd.Start-1]...)
		if normalizedNewText != "" {
			newLines = append(newLines, strings.Split(normalizedNewText, "\n")...)
		}
		newLines = append(newLines, lines[cmd.End:]...)

	case "INSERT_AFTER":
		// Insert NewText after line Start
		newLines = append(newLines, lines[:cmd.Start]...)
		if normalizedNewText != "" {
			newLines = append(newLines, strings.Split(normalizedNewText, "\n")...)
		}
		newLines = append(newLines, lines[cmd.Start:]...)

	case "INSERT_BEFORE":
		// Insert NewText before line Start
		newLines = append(newLines, lines[:cmd.Start-1]...)
		if normalizedNewText != "" {
			newLines = append(newLines, strings.Split(normalizedNewText, "\n")...)
		}
		newLines = append(newLines, lines[cmd.Start-1:]...)

	case "DELETE":
		// Delete lines Start through End
		newLines = append(newLines, lines[:cmd.Start-1]...)
		newLines = append(newLines, lines[cmd.End:]...)

	default:
		return fmt.Errorf("unsupported action: %s", cmd.Action)
	}

	// Write the modified content back to file
	newContent := strings.Join(newLines, "\n")

	// Preserve original trailing newline behavior
	if hasFinalNewline && !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	} else if !hasFinalNewline && strings.HasSuffix(newContent, "\n") {
		newContent = strings.TrimSuffix(newContent, "\n")
	}

	// Always write with LF line endings for consistency across platforms
	err = ioutil.WriteFile(filePath, []byte(newContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	return nil
}
