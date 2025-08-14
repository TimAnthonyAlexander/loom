package editor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ApplyEdit applies an edit plan to the filesystem.
func ApplyEdit(plan *EditPlan) error {
	// Create the directory structure if needed
	dir := filepath.Dir(plan.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// For deletion, remove the file
	if plan.IsDeletion {
		if err := os.Remove(plan.FilePath); err != nil {
			return fmt.Errorf("failed to delete file: %w", err)
		}
		return nil
	}

	// For creation or modification, write the new content
	if err := os.WriteFile(plan.FilePath, []byte(plan.NewContent), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// GenerateGitDiff creates a git-style diff for file changes.
func GenerateGitDiff(oldContent, newContent, filePath string) (string, error) {
	// For special cases like new files or deleted files
	if oldContent == "" {
		return fmt.Sprintf("New file: %s\n\n%s", filePath, newContent), nil
	}

	if newContent == "" {
		return fmt.Sprintf("Deleted file: %s\n\n%s", filePath, oldContent), nil
	}

	// Create temporary files for diffing
	dir, err := os.MkdirTemp("", "loom-diff")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	oldFile := filepath.Join(dir, "old")
	newFile := filepath.Join(dir, "new")

	if err := os.WriteFile(oldFile, []byte(oldContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.WriteFile(newFile, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}

	// Run git diff
	cmd := exec.Command("git", "diff", "--no-index", "--color=never", oldFile, newFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// git diff returns exit code 1 if files differ, which is expected
		if strings.Contains(string(output), "diff --git") {
			// This is fine, git diff found differences
		} else {
			return "", fmt.Errorf("diff command failed: %w", err)
		}
	}

	// Process the output to make it more readable
	diffText := string(output)

	// Replace the temp file paths with actual file name
	diffText = strings.ReplaceAll(diffText, "--- a/old", fmt.Sprintf("--- a/%s", filepath.Base(filePath)))
	diffText = strings.ReplaceAll(diffText, "+++ b/new", fmt.Sprintf("+++ b/%s", filepath.Base(filePath)))

	return diffText, nil
}

// ValidateEditSafety performs additional checks on the edit.
func ValidateEditSafety(plan *EditPlan) error {
	// Check for suspicious patterns in the new content
	if containsSuspiciousPatterns(plan.NewContent) {
		return ValidationError{
			Message: "Edit contains potentially harmful patterns",
			Code:    "SECURITY_RISK",
		}
	}

	// Check for suspicious file extensions
	ext := strings.ToLower(filepath.Ext(plan.FilePath))
	if isSuspiciousExtension(ext) {
		return ValidationError{
			Message: fmt.Sprintf("Cannot edit files with extension %s", ext),
			Code:    "FORBIDDEN_EXTENSION",
		}
	}

	return nil
}

// containsSuspiciousPatterns checks for potentially harmful content.
func containsSuspiciousPatterns(content string) bool {
	suspiciousPatterns := []string{
		// Malicious looking commands/scripts
		"rm -rf /",
		"chmod 777",
		"curl | bash",
		"wget | bash",
		// Potentially harmful payloads
		"<script>evil",
		"eval(",
		"system(",
		"exec(",
		"subprocess",
		"child_process",
		// Potential secrets or keys
		"password=",
		"apikey=",
		"secret=",
		"token=",
	}

	content = strings.ToLower(content)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(content, pattern) {
			return true
		}
	}

	return false
}

// isSuspiciousExtension checks if the file extension is potentially harmful.
func isSuspiciousExtension(ext string) bool {
	suspiciousExtensions := []string{
		".exe", ".dll", ".so", ".dylib",
		".sh", ".bash", ".bat", ".cmd",
		".jar", ".war", ".php", ".cgi",
		".py", ".rb", ".pl",
	}

	for _, susExt := range suspiciousExtensions {
		if ext == susExt {
			return true
		}
	}

	return false
}
