package loom_edit

import (
	"crypto/sha1"
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
	File    string // The target file path
	Action  string // REPLACE, INSERT_AFTER, INSERT_BEFORE, or DELETE
	Start   int    // 1-based inclusive start line number
	End     int    // 1-based inclusive end line number
	NewText string // The replacement/insertion text
}

// ParseEditCommand parses a LOOM_EDIT command block into an EditCommand struct
func ParseEditCommand(input string) (*EditCommand, error) {
	// Normalize line endings and ensure input is not empty
	input = strings.ReplaceAll(input, "\r\n", "\n")
	input = strings.ReplaceAll(input, "\r", "\n")

	if strings.TrimSpace(input) == "" {
		return nil, fmt.Errorf("empty LOOM_EDIT command")
	}

	// Regex to match different variations of the LOOM_EDIT header line
	// This handles both formats: >>LOOM_EDIT and 🔧 LOOM_EDIT
	// And allows for more variations in the line range format
	headerRegex := regexp.MustCompile(`(?:>>|🔧 )LOOM_EDIT file=([^\s]+) (REPLACE|INSERT_AFTER|INSERT_BEFORE|DELETE)(?:\s+(\d+)(?:-(\d+))?)?`)

	lines := strings.Split(input, "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("invalid LOOM_EDIT format: too few lines")
	}

	// Parse header line
	headerLine := lines[0]
	headerMatches := headerRegex.FindStringSubmatch(headerLine)
	if headerMatches == nil {
		return nil, fmt.Errorf("invalid LOOM_EDIT header format: %s", headerLine)
	}

	cmd := &EditCommand{
		File:   headerMatches[1],
		Action: headerMatches[2],
	}

	// Validate required action
	switch cmd.Action {
	case "REPLACE", "INSERT_AFTER", "INSERT_BEFORE", "DELETE":
		// Valid actions
	default:
		return nil, fmt.Errorf("invalid action: %s (must be REPLACE, INSERT_AFTER, INSERT_BEFORE, or DELETE)", cmd.Action)
	}

	// Check if we have line numbers at all
	if len(headerMatches) <= 3 || headerMatches[3] == "" {
		return nil, fmt.Errorf("missing line number in %s action", cmd.Action)
	}

	// Parse start line number
	start, err := strconv.Atoi(headerMatches[3])
	if err != nil {
		return nil, fmt.Errorf("invalid start line number: %v", err)
	}
	if start < 1 {
		return nil, fmt.Errorf("line numbers must be >= 1, got start=%d", start)
	}
	cmd.Start = start

	// Parse end line number (optional for some operations)
	if len(headerMatches) > 4 && headerMatches[4] != "" {
		end, err := strconv.Atoi(headerMatches[4])
		if err != nil {
			return nil, fmt.Errorf("invalid end line number: %v", err)
		}
		if end < start {
			return nil, fmt.Errorf("end line (%d) must be >= start line (%d)", end, start)
		}
		cmd.End = end
	} else {
		// For all operations, if no end line is specified, default to start line
		cmd.End = start

		// Warn if REPLACE or DELETE should have specified an end line
		if cmd.Action == "REPLACE" || cmd.Action == "DELETE" {
			fmt.Printf("Warning: %s action missing end line number, defaulting to %d\n", cmd.Action, start)
		}
	}

	// Extract new text (everything between header line and <<LOOM_EDIT)
	var newTextLines []string
	var foundClosingTag bool

	for i := 1; i < len(lines); i++ {
		line := lines[i]
		if line == "<<LOOM_EDIT" {
			foundClosingTag = true
			break
		}
		newTextLines = append(newTextLines, line)
	}

	if !foundClosingTag {
		return nil, fmt.Errorf("invalid LOOM_EDIT format: missing closing <<LOOM_EDIT tag")
	}

	// For DELETE action, newText should be empty
	if cmd.Action == "DELETE" && strings.TrimSpace(strings.Join(newTextLines, "")) != "" {
		fmt.Printf("Warning: DELETE action should have empty content block, ignoring provided content\n")
		newTextLines = nil
	}

	cmd.NewText = strings.Join(newTextLines, "\n")

	return cmd, nil
}

func HashContent(content string) string {
	return fmt.Sprintf("%x", sha1.Sum([]byte(content)))
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

	err = ioutil.WriteFile(filePath, []byte(newContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	return nil
}
