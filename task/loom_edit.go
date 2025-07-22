package task

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// LoomEditProcessor handles the new LOOM_EDIT system
type LoomEditProcessor struct {
	workspacePath string
}

// NewLoomEditProcessor creates a new LOOM_EDIT processor
func NewLoomEditProcessor(workspacePath string) *LoomEditProcessor {
	return &LoomEditProcessor{
		workspacePath: workspacePath,
	}
}

// EditBlock represents a single LOOM_EDIT block
type EditBlock struct {
	DiffContent string
	FilePath    string // extracted from the diff header
}

// editRE matches LOOM_EDIT fenced code blocks
var editRE = regexp.MustCompile(`(?s)` + "`" + `{3}LOOM_EDIT\n(.*?)` + "`" + `{3}`)

// ParseLoomEdits extracts all LOOM_EDIT blocks from an LLM message
func (p *LoomEditProcessor) ParseLoomEdits(message string) ([]EditBlock, error) {
	matches := editRE.FindAllStringSubmatch(message, -1)
	if len(matches) == 0 {
		return nil, nil // no edit blocks found
	}

	var blocks []EditBlock
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		diffContent := strings.TrimSpace(match[1])
		if diffContent == "" {
			continue
		}

		// Extract file path from diff header
		filePath, err := p.extractFilePathFromDiff(diffContent)
		if err != nil {
			return nil, fmt.Errorf("failed to extract file path from diff: %v", err)
		}

		blocks = append(blocks, EditBlock{
			DiffContent: diffContent,
			FilePath:    filePath,
		})
	}

	return blocks, nil
}

// extractFilePathFromDiff extracts the file path from unified diff headers
func (p *LoomEditProcessor) extractFilePathFromDiff(diffContent string) (string, error) {
	lines := strings.Split(diffContent, "\n")
	
	for _, line := range lines {
		if strings.HasPrefix(line, "+++ b/") {
			// Extract path after "+++ b/"
			path := strings.TrimPrefix(line, "+++ b/")
			return strings.TrimSpace(path), nil
		}
	}

	return "", fmt.Errorf("no valid +++ b/ header found in diff")
}

// ApplyEdits applies all LOOM_EDIT blocks using git apply
func (p *LoomEditProcessor) ApplyEdits(blocks []EditBlock) error {
	for i, block := range blocks {
		if err := p.applyEditBlock(block, i); err != nil {
			return fmt.Errorf("failed to apply edit block %d (file: %s): %v", i+1, block.FilePath, err)
		}
	}
	return nil
}

// applyEditBlock applies a single edit block using git apply
func (p *LoomEditProcessor) applyEditBlock(block EditBlock, blockIndex int) error {
	// Create temporary patch file
	tmpDir := os.TempDir()
	patchFile := filepath.Join(tmpDir, fmt.Sprintf("loom_edit_%d.patch", blockIndex))
	
	// Write the diff content to the temp file
	if err := os.WriteFile(patchFile, []byte(block.DiffContent), 0600); err != nil {
		return fmt.Errorf("failed to write patch file: %v", err)
	}
	
	// Clean up the temp file when done
	defer func() {
		os.Remove(patchFile)
	}()

	// Run git apply
	cmd := exec.Command("git", "apply", "--reject", "--whitespace=nowarn", patchFile)
	cmd.Dir = p.workspacePath
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git apply failed: %v\nOutput: %s", err, string(output))
	}

	return nil
}

// ProcessMessage is the main entry point that parses and applies LOOM_EDIT blocks
func (p *LoomEditProcessor) ProcessMessage(message string) (*LoomEditResult, error) {
	blocks, err := p.ParseLoomEdits(message)
	if err != nil {
		return nil, err
	}

	if len(blocks) == 0 {
		return &LoomEditResult{
			BlocksFound: 0,
			FilesEdited: []string{},
		}, nil
	}

	// Apply all edit blocks
	if err := p.ApplyEdits(blocks); err != nil {
		return nil, err
	}

	// Collect edited file paths
	var filePaths []string
	for _, block := range blocks {
		filePaths = append(filePaths, block.FilePath)
	}

	return &LoomEditResult{
		BlocksFound: len(blocks),
		FilesEdited: filePaths,
	}, nil
}

// LoomEditResult contains the result of processing LOOM_EDIT blocks
type LoomEditResult struct {
	BlocksFound int
	FilesEdited []string
}

// ValidateDiffFormat performs basic validation on a unified diff
func (p *LoomEditProcessor) ValidateDiffFormat(diffContent string) error {
	lines := strings.Split(diffContent, "\n")
	
	hasMinusHeader := false
	hasPlusHeader := false
	hasHunkHeader := false
	
	for _, line := range lines {
		if strings.HasPrefix(line, "--- a/") {
			hasMinusHeader = true
		} else if strings.HasPrefix(line, "+++ b/") {
			hasPlusHeader = true
		} else if strings.HasPrefix(line, "@@") && strings.HasSuffix(line, "@@") {
			hasHunkHeader = true
		}
	}
	
	if !hasMinusHeader {
		return fmt.Errorf("missing --- a/ header")
	}
	if !hasPlusHeader {
		return fmt.Errorf("missing +++ b/ header")
	}
	if !hasHunkHeader {
		return fmt.Errorf("missing @@ hunk header")
	}
	
	return nil
} 