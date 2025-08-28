package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ActionType defines the type of advanced edit to perform.
type ActionType string

const (
    ActionCreate        ActionType = "CREATE"
    ActionReplaceLines  ActionType = "REPLACE"
    ActionInsertAfter   ActionType = "INSERT_AFTER"
    ActionInsertBefore  ActionType = "INSERT_BEFORE"
    ActionDeleteLines   ActionType = "DELETE"
    ActionSearchReplace ActionType = "SEARCH_REPLACE"
    // ActionAnchorReplace performs a content-anchored replacement that avoids line numbers.
    // It locates the target region by optional anchors (before/after) and/or a target block,
    // with optional whitespace normalization and fuzzy matching.
    ActionAnchorReplace ActionType = "ANCHOR_REPLACE"
)

// AdvancedEditRequest captures parameters for advanced edits.
type AdvancedEditRequest struct {
    FilePath string
    Action   ActionType
    // Common content payload for actions that add/replace text
    Content string
    // Line-based addressing (1-indexed, inclusive)
    StartLine int
    EndLine   int
    Line      int // Used for insert before/after
    // Search/replace payload
    OldString string
    NewString string

    // Anchor-based replace payload (for ANCHOR_REPLACE)
    // All anchors/target are optional, but at least one of target or an anchor must be provided.
    AnchorBefore        string  // text before the region to replace (not included in replacement)
    Target              string  // the text block intended to be replaced
    AnchorAfter         string  // text after the region to replace (not included in replacement)
    NormalizeWhitespace bool    // if true, collapse runs of spaces/tabs and match ignoring minor spacing
    FuzzyThreshold      float64 // 0..1, higher means stricter desired match; used to configure fuzzy matcher
    Occurrence          int     // 1-based occurrence selection for anchors (default 1)
}

// ProposeAdvancedEdit validates and constructs an EditPlan based on an AdvancedEditRequest.
func ProposeAdvancedEdit(workspacePath string, req AdvancedEditRequest) (*EditPlan, error) {
	// Normalize and validate file path
	absPath, err := validatePath(workspacePath, req.FilePath)
	if err != nil {
		return nil, err
	}

	// Determine file existence
	fileInfo, statErr := os.Stat(absPath)
	fileExists := statErr == nil
	if statErr != nil && !os.IsNotExist(statErr) {
		return nil, ValidationError{
			Message: fmt.Sprintf("Failed to access file: %v", statErr),
			Code:    "FILE_ACCESS_ERROR",
		}
	}

	// Disallow directory edits
	if fileExists && fileInfo.IsDir() {
		return nil, ValidationError{
			Message: "Cannot edit a directory",
			Code:    "IS_DIRECTORY",
		}
	}

	switch req.Action {
	case ActionCreate:
		// CREATE new file
		if fileExists {
			return nil, ValidationError{
				Message: "File already exists",
				Code:    "FILE_EXISTS",
			}
		}
		lineCount := 1
		if req.Content != "" {
			lineCount = 1 + strings.Count(req.Content, "\n")
		}
		return &EditPlan{
			FilePath:   absPath,
			OldContent: "",
			NewContent: req.Content,
			Diff:       generateDiff("", req.Content, filepath.Base(absPath)),
			IsCreation: true,
			ChangedLines: LineRange{
				StartLine: 1,
				EndLine:   lineCount,
			},
		}, nil

    case ActionReplaceLines, ActionInsertAfter, ActionInsertBefore, ActionDeleteLines, ActionSearchReplace, ActionAnchorReplace:
        if !fileExists {
            return nil, ValidationError{
                Message: "File does not exist",
                Code:    "FILE_NOT_EXIST",
            }
        }

		// Load current content
		bytes, err := os.ReadFile(absPath)
		if err != nil {
			return nil, ValidationError{
				Message: fmt.Sprintf("Failed to read file: %v", err),
				Code:    "FILE_READ_ERROR",
			}
		}

		oldContent := string(bytes)
		lines := splitToLinesPreserveEOF(oldContent)

		var newContent string
		var changed LineRange

        switch req.Action {
        case ActionReplaceLines:
			if req.StartLine <= 0 || req.EndLine <= 0 || req.StartLine > req.EndLine {
				return nil, ValidationError{
					Message: "Invalid line range for REPLACE",
					Code:    "INVALID_RANGE",
				}
			}
			startIdx := req.StartLine - 1
			endIdx := req.EndLine - 1
			if startIdx >= len(lines) || endIdx >= len(lines) {
				return nil, ValidationError{
					Message: "Line range out of bounds",
					Code:    "RANGE_OOB",
				}
			}
			replacement := strings.Split(req.Content, "\n")
			// Perform replacement
			var merged []string
			merged = append(merged, lines[:startIdx]...)
			merged = append(merged, replacement...)
			merged = append(merged, lines[endIdx+1:]...)
			newContent = strings.Join(merged, "\n")
			changed = LineRange{StartLine: req.StartLine, EndLine: req.StartLine + len(replacement) - 1}

		case ActionInsertBefore:
			if req.Line <= 0 {
				return nil, ValidationError{
					Message: "Line must be >= 1 for INSERT_BEFORE",
					Code:    "INVALID_LINE",
				}
			}
			insertIdx := req.Line - 1
			if insertIdx > len(lines) { // allow at most len(lines) for inserting at EOF
				return nil, ValidationError{
					Message: "Insert position out of bounds",
					Code:    "LINE_OOB",
				}
			}
			insertion := strings.Split(req.Content, "\n")
			var merged []string
			merged = append(merged, lines[:insertIdx]...)
			merged = append(merged, insertion...)
			merged = append(merged, lines[insertIdx:]...)
			newContent = strings.Join(merged, "\n")
			changed = LineRange{StartLine: req.Line, EndLine: req.Line + len(insertion) - 1}

		case ActionInsertAfter:
			if req.Line < 0 {
				return nil, ValidationError{
					Message: "Line must be >= 0 for INSERT_AFTER",
					Code:    "INVALID_LINE",
				}
			}
			insertion := strings.Split(req.Content, "\n")
			insertIdx := req.Line // because it's after the given line (1-indexed)
			if insertIdx < 0 {
				insertIdx = 0
			}
			if insertIdx > len(lines) {
				insertIdx = len(lines)
			}
			var merged []string
			merged = append(merged, lines[:insertIdx]...)
			merged = append(merged, insertion...)
			merged = append(merged, lines[insertIdx:]...)
			newContent = strings.Join(merged, "\n")
			changed = LineRange{StartLine: req.Line + 1, EndLine: req.Line + len(insertion)}

		case ActionDeleteLines:
			if req.StartLine <= 0 || req.EndLine <= 0 || req.StartLine > req.EndLine {
				return nil, ValidationError{
					Message: "Invalid line range for DELETE",
					Code:    "INVALID_RANGE",
				}
			}
			startIdx := req.StartLine - 1
			endIdx := req.EndLine - 1
			if startIdx >= len(lines) || endIdx >= len(lines) {
				return nil, ValidationError{
					Message: "Line range out of bounds",
					Code:    "RANGE_OOB",
				}
			}
			var merged []string
			merged = append(merged, lines[:startIdx]...)
			if endIdx+1 < len(lines) {
				merged = append(merged, lines[endIdx+1:]...)
			}
			newContent = strings.Join(merged, "\n")
			changed = LineRange{StartLine: req.StartLine, EndLine: req.EndLine}

        case ActionSearchReplace:
            if req.OldString == "" {
                return nil, ValidationError{
                    Message: "old_string cannot be empty for SEARCH_REPLACE",
                    Code:    "EMPTY_OLD_STRING",
                }
            }
            occurrences := strings.Count(oldContent, req.OldString)
            if occurrences == 0 {
                return nil, ValidationError{
                    Message: "Old string not found in file",
                    Code:    "STRING_NOT_FOUND",
                }
            }
            newContent = strings.ReplaceAll(oldContent, req.OldString, req.NewString)

            // Determine affected line range (min..max lines that contained the old string)
            minLine := 0
            maxLine := 0
            for i, ln := range strings.Split(oldContent, "\n") {
                if strings.Contains(ln, req.OldString) {
                    lineNum := i + 1
                    if minLine == 0 || lineNum < minLine {
                        minLine = lineNum
                    }
                    if lineNum > maxLine {
                        maxLine = lineNum
                    }
                }
            }
            if minLine == 0 {
                minLine = 1
            }
            if maxLine == 0 {
                maxLine = strings.Count(oldContent, "\n") + 1
            }
            changed = LineRange{StartLine: minLine, EndLine: maxLine}

        case ActionAnchorReplace:
            // Robust anchored replace: find region using anchors/target with optional normalization and fuzzy match
            if strings.TrimSpace(req.AnchorBefore) == "" && strings.TrimSpace(req.AnchorAfter) == "" && strings.TrimSpace(req.Target) == "" {
                return nil, ValidationError{Message: "ANCHOR_REPLACE requires at least target or one anchor", Code: "MISSING_ANCHORS"}
            }
            // Defaults
            occ := req.Occurrence
            if occ <= 0 {
                occ = 1
            }
            // Prepare normalized text and index mapping
            normText, idxMap := normalizeWithMap(oldContent, req.NormalizeWhitespace)
            // Helper to normalize a pattern consistently
            norm := func(s string) string {
                ns, _ := normalizeWithMap(s, req.NormalizeWhitespace)
                return ns
            }
            // Locate anchors in normalized space
            winStartNorm := 0
            winEndNorm := len(normText)
            if strings.TrimSpace(req.AnchorBefore) != "" {
                pos := findNth(normText, norm(req.AnchorBefore), occ)
                if pos < 0 {
                    return nil, ValidationError{Message: "anchor_before not found", Code: "ANCHOR_BEFORE_NOT_FOUND"}
                }
                winStartNorm = pos + len(norm(req.AnchorBefore))
                // If the anchor ends at end-of-line, preserve the newline by starting after it
                if winStartNorm < len(normText) && normText[winStartNorm] == '\n' {
                    winStartNorm++
                }
            }
            if strings.TrimSpace(req.AnchorAfter) != "" {
                pos := findNth(normText, norm(req.AnchorAfter), occ)
                if pos < 0 {
                    return nil, ValidationError{Message: "anchor_after not found", Code: "ANCHOR_AFTER_NOT_FOUND"}
                }
                winEndNorm = pos
            }
            if winStartNorm > winEndNorm {
                return nil, ValidationError{Message: "anchor window invalid: before occurs after after", Code: "ANCHOR_WINDOW_INVALID"}
            }
            // Determine target region in normalized window
            regionStartNorm := winStartNorm
            regionEndNorm := winEndNorm
            if strings.TrimSpace(req.Target) != "" {
                tgtNorm := norm(req.Target)
                if tgtNorm == "" {
                    return nil, ValidationError{Message: "empty target after normalization", Code: "EMPTY_TARGET"}
                }
                window := normText[winStartNorm:winEndNorm]
                // Exact search first
                rel := strings.Index(window, tgtNorm)
                if rel < 0 {
                    // Fuzzy search using diff-match-patch
                    // Convert fuzzy threshold (high=strict) to dmp threshold (low=strict)
                    dmpThreshold := 0.5
                    if req.FuzzyThreshold > 0 {
                        ft := req.FuzzyThreshold
                        if ft < 0 {
                            ft = 0
                        } else if ft > 1 {
                            ft = 1
                        }
                        dmpThreshold = 1.0 - ft
                    }
                    // Perform MatchMain
                    rel = fuzzyMatch(window, tgtNorm, dmpThreshold)
                    if rel < 0 {
                        return nil, ValidationError{Message: "target not found within anchors", Code: "TARGET_NOT_FOUND"}
                    }
                }
                regionStartNorm = winStartNorm + rel
                regionEndNorm = regionStartNorm + len(tgtNorm)
            }
            // Map normalized indices back to original indices (exclusive end)
            startOrig := mapNormToOrig(idxMap, regionStartNorm, len(normText), len(oldContent))
            endOrig := mapNormToOrig(idxMap, regionEndNorm, len(normText), len(oldContent))
            if startOrig < 0 || endOrig < 0 || startOrig > endOrig || endOrig > len(oldContent) {
                return nil, ValidationError{Message: "failed to map indices to original content", Code: "INDEX_MAP_ERROR"}
            }
            // Two-phase replace: delete region then insert new content
            newContent = oldContent[:startOrig] + req.Content + oldContent[endOrig:]
            // Determine affected line range
            startLine := 1 + strings.Count(oldContent[:startOrig], "\n")
            endLine := 1 + strings.Count(oldContent[:endOrig], "\n")
            // After replacement, compute new end based on inserted content
            insLines := 0
            if req.Content != "" {
                insLines = strings.Count(req.Content, "\n") + 1
            } else {
                insLines = 0
            }
            _ = endLine // unused but kept for potential future use
            changed = LineRange{StartLine: startLine, EndLine: startLine + insLines - 1}
        }

        return &EditPlan{
            FilePath:     absPath,
            OldContent:   oldContent,
            NewContent:   newContent,
            Diff:         generateDiff(oldContent, newContent, filepath.Base(absPath)),
            ChangedLines: changed,
        }, nil

	default:
		return nil, ValidationError{
			Message: fmt.Sprintf("Unsupported action: %s", req.Action),
			Code:    "UNSUPPORTED_ACTION",
		}
	}
}

// splitToLinesPreserveEOF splits into lines without dropping the last empty line when the file ends with a newline.
func splitToLinesPreserveEOF(content string) []string {
	// Using strings.Split preserves trailing empty segment when content ends with a newline
	return strings.Split(content, "\n")
}
