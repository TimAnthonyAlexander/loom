package loom_edit

import (
	"crypto/sha1"
	"fmt"
	"io/ioutil"
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
	// Regex to match the LOOM_EDIT header line - support both >>LOOM_EDIT and 🔧 LOOM_EDIT formats
	headerRegex := regexp.MustCompile(`(?:>>|🔧 )LOOM_EDIT file=([^\s]+) (REPLACE|INSERT_AFTER|INSERT_BEFORE|DELETE) (\d+)(?:-(\d+))?`)

	lines := strings.Split(input, "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("invalid LOOM_EDIT format: too few lines")
	}

	// Parse header line
	headerMatches := headerRegex.FindStringSubmatch(lines[0])
	if headerMatches == nil {
		return nil, fmt.Errorf("invalid LOOM_EDIT header format: %s", lines[0])
	}

	// For debugging: Print the matched groups
	if len(headerMatches) < 4 {
		return nil, fmt.Errorf("invalid LOOM_EDIT header: insufficient match groups: %v", headerMatches)
	}

	cmd := &EditCommand{
		File:   headerMatches[1],
		Action: headerMatches[2],
	}

	// Parse start line number
	start, err := strconv.Atoi(headerMatches[3])
	if err != nil {
		return nil, fmt.Errorf("invalid start line number: %v", err)
	}
	cmd.Start = start

	// Parse end line number (optional for some operations)
	if len(headerMatches) > 4 && headerMatches[4] != "" {
		end, err := strconv.Atoi(headerMatches[4])
		if err != nil {
			return nil, fmt.Errorf("invalid end line number: %v", err)
		}
		cmd.End = end
	} else {
		// For INSERT_AFTER and INSERT_BEFORE, end equals start
		// For REPLACE and DELETE, end should be specified, but we'll default to start for safety
		cmd.End = start
		
		// Warn if REPLACE or DELETE doesn't specify an end line
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

	cmd.NewText = strings.Join(newTextLines, "\n")

	return cmd, nil
}

func HashContent(content string) string {
	return fmt.Sprintf("%x", sha1.Sum([]byte(content)))
}

// ApplyEdit applies an EditCommand to a file
func ApplyEdit(filePath string, cmd *EditCommand) error {
	// Read the current file content
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	// Normalize line endings to LF and detect trailing newline
	contentStr := strings.ReplaceAll(string(content), "\r\n", "\n")
	contentStr = strings.ReplaceAll(contentStr, "\r", "\n")
	hasFinalNewline := strings.HasSuffix(contentStr, "\n")

	// Split content into lines (using normalized content)
	lines := strings.Split(contentStr, "\n")

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
