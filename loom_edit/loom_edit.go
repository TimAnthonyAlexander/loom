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
	FileSHA string // SHA of the entire file when LLM read it
	Action  string // REPLACE, INSERT_AFTER, INSERT_BEFORE, or DELETE
	Start   int    // 1-based inclusive start line number
	End     int    // 1-based inclusive end line number
	OldHash string // SHA of the old slice for validation
	NewText string // The replacement/insertion text
}

// ParseEditCommand parses a LOOM_EDIT command block into an EditCommand struct
func ParseEditCommand(input string) (*EditCommand, error) {
	// Regex to match the LOOM_EDIT header line
	headerRegex := regexp.MustCompile(`>>LOOM_EDIT file=([^\s]+) v=([^\s]+) (REPLACE|INSERT_AFTER|INSERT_BEFORE|DELETE) (\d+)(?:-(\d+))?`)
	
	// Regex to match the OLD_HASH line
	hashRegex := regexp.MustCompile(`#OLD_HASH:([a-fA-F0-9]+)`)
	
	lines := strings.Split(input, "\n")
	if len(lines) < 3 {
		return nil, fmt.Errorf("invalid LOOM_EDIT format: too few lines")
	}
	
	// Parse header line
	headerMatches := headerRegex.FindStringSubmatch(lines[0])
	if headerMatches == nil {
		return nil, fmt.Errorf("invalid LOOM_EDIT header format")
	}
	
	cmd := &EditCommand{
		File:   headerMatches[1],
		FileSHA: headerMatches[2],
		Action: headerMatches[3],
	}
	
	// Parse start line number
	start, err := strconv.Atoi(headerMatches[4])
	if err != nil {
		return nil, fmt.Errorf("invalid start line number: %v", err)
	}
	cmd.Start = start
	
	// Parse end line number (optional for some operations)
	if headerMatches[5] != "" {
		end, err := strconv.Atoi(headerMatches[5])
		if err != nil {
			return nil, fmt.Errorf("invalid end line number: %v", err)
		}
		cmd.End = end
	} else {
		// For INSERT_AFTER and INSERT_BEFORE, end equals start
		cmd.End = start
	}
	
	// Parse OLD_HASH line
	hashMatches := hashRegex.FindStringSubmatch(lines[1])
	if hashMatches == nil {
		return nil, fmt.Errorf("invalid OLD_HASH format")
	}
	cmd.OldHash = hashMatches[1]
	
	// Extract new text (everything between OLD_HASH line and <<LOOM_EDIT)
	var newTextLines []string
	inBody := false
	for i, line := range lines {
		if i == 0 || strings.HasPrefix(line, "#OLD_HASH:") {
			if strings.HasPrefix(line, "#OLD_HASH:") {
				inBody = true
			}
			continue
		}
		if line == "<<LOOM_EDIT" {
			break
		}
		if inBody {
			newTextLines = append(newTextLines, line)
		}
	}
	
	cmd.NewText = strings.Join(newTextLines, "\n")
	
	return cmd, nil
}

// HashContent computes SHA1 hash of the given content
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
	
	// Validate file SHA
	currentFileSHA := HashContent(string(content))
	if currentFileSHA != cmd.FileSHA {
		return fmt.Errorf("file SHA mismatch: expected %s, got %s", cmd.FileSHA, currentFileSHA)
	}
	
	// Split content into lines
	lines := strings.Split(string(content), "\n")
	
	// Validate line range
	if cmd.Start < 1 || cmd.Start > len(lines) {
		return fmt.Errorf("start line %d is out of range (1-%d)", cmd.Start, len(lines))
	}
	if cmd.End < cmd.Start || cmd.End > len(lines) {
		return fmt.Errorf("end line %d is out of range (%d-%d)", cmd.End, cmd.Start, len(lines))
	}
	
	// Extract old slice and validate hash
	var oldSlice []string
	if cmd.Action == "DELETE" || cmd.Action == "REPLACE" {
		oldSlice = lines[cmd.Start-1:cmd.End]
	} else if cmd.Action == "INSERT_AFTER" || cmd.Action == "INSERT_BEFORE" {
		// For insert operations, we still need to validate the reference line
		oldSlice = lines[cmd.Start-1:cmd.Start]
	}
	
	oldSliceContent := strings.Join(oldSlice, "\n")
	oldSliceHash := HashContent(oldSliceContent)
	if oldSliceHash != cmd.OldHash {
		return fmt.Errorf("old slice hash mismatch: expected %s, got %s", cmd.OldHash, oldSliceHash)
	}
	
	// Apply the edit based on action
	var newLines []string
	
	switch cmd.Action {
	case "REPLACE":
		// Replace lines Start through End with NewText
		newLines = append(newLines, lines[:cmd.Start-1]...)
		if cmd.NewText != "" {
			newLines = append(newLines, strings.Split(cmd.NewText, "\n")...)
		}
		newLines = append(newLines, lines[cmd.End:]...)
		
	case "INSERT_AFTER":
		// Insert NewText after line Start
		newLines = append(newLines, lines[:cmd.Start]...)
		if cmd.NewText != "" {
			newLines = append(newLines, strings.Split(cmd.NewText, "\n")...)
		}
		newLines = append(newLines, lines[cmd.Start:]...)
		
	case "INSERT_BEFORE":
		// Insert NewText before line Start
		newLines = append(newLines, lines[:cmd.Start-1]...)
		if cmd.NewText != "" {
			newLines = append(newLines, strings.Split(cmd.NewText, "\n")...)
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
	err = ioutil.WriteFile(filePath, []byte(newContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}
	
	return nil
} 