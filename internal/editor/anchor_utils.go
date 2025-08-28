package editor

import (
    "strings"

    "github.com/sergi/go-diff/diffmatchpatch"
)

// normalizeWithMap optionally normalizes whitespace in s and returns the normalized
// string along with a mapping from each normalized index to the corresponding
// original index in s. Newlines are preserved and CR is removed. Runs of spaces/tabs
// are collapsed to a single space when normalization is enabled.
func normalizeWithMap(s string, normalize bool) (string, []int) {
    if !normalize {
        // Identity mapping
        idx := make([]int, len(s))
        for i := range s {
            idx[i] = i
        }
        return s, idx
    }
    var b strings.Builder
    // Pre-size to avoid excessive allocations
    b.Grow(len(s))
    indexMap := make([]int, 0, len(s))
    prevSpace := false
    for i := 0; i < len(s); i++ {
        ch := s[i]
        // Normalize CRLF to LF by dropping CR
        if ch == '\r' {
            continue
        }
        // Collapse runs of spaces/tabs
        if ch == ' ' || ch == '\t' {
            if prevSpace {
                continue
            }
            prevSpace = true
            b.WriteByte(' ')
            indexMap = append(indexMap, i)
            continue
        }
        prevSpace = false
        b.WriteByte(ch)
        indexMap = append(indexMap, i)
    }
    return b.String(), indexMap
}

// mapNormToOrig converts a normalized index back to the original index.
// If normIdx == normLen, returns origLen (exclusive end).
func mapNormToOrig(indexMap []int, normIdx int, normLen int, origLen int) int {
    if normIdx == normLen {
        return origLen
    }
    if normIdx < 0 || normIdx >= len(indexMap) {
        return -1
    }
    return indexMap[normIdx]
}

// findNth returns the 0-based index of the nth (1-based) occurrence of pattern in text.
// Returns -1 if not found.
func findNth(text, pattern string, occurrence int) int {
    if occurrence <= 0 {
        occurrence = 1
    }
    if pattern == "" {
        return -1
    }
    start := 0
    found := -1
    for i := 0; i < occurrence; i++ {
        idx := strings.Index(text[start:], pattern)
        if idx < 0 {
            return -1
        }
        found = start + idx
        start = found + len(pattern)
    }
    return found
}

// fuzzyMatch uses diff-match-patch to find the best match location for pattern within text.
// threshold is dmp.MatchThreshold (lower is stricter). Returns -1 if no match.
func fuzzyMatch(text, pattern string, threshold float64) int {
    dmp := diffmatchpatch.New()
    if threshold > 0 {
        dmp.MatchThreshold = threshold
    }
    // dmp.MatchMain returns index or -1
    idx := dmp.MatchMain(text, pattern, 0)
    return idx
}

