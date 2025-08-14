package symbols

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Symbol represents an indexed symbol definition.
type Symbol struct {
	SID          string
	FilePath     string
	LineStart    int
	ColStart     int
	LineEnd      int
	ColEnd       int
	Lang         string
	Name         string
	Kind         string
	ContainerSID string
	Signature    string
	DocExcerpt   string
	Confidence   float64
	Version      string
}

// Relation represents a relationship between symbols (calls, defines, references, etc.).
type Relation struct {
	FromSID   string
	ToSID     string
	Kind      string
	FilePath  string
	LineStart int
	LineEnd   int
}

// SymbolCard represents a compact summary for LLM consumption.
type SymbolCard struct {
	SID        string  `json:"sid"`
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	File       string  `json:"file"`
	Span       [4]int  `json:"span"` // [line_start, col_start, line_end, col_end]
	Container  string  `json:"container,omitempty"`
	Signature  string  `json:"signature,omitempty"`
	DocExcerpt string  `json:"doc_excerpt,omitempty"`
	Confidence float64 `json:"confidence"`
	Lang       string  `json:"lang"`
	Why        string  `json:"why,omitempty"`
}

// RefSite represents a reference site returned to the LLM.
type RefSite struct {
	File      string `json:"file"`
	LineStart int    `json:"line_start"`
	LineEnd   int    `json:"line_end"`
	Kind      string `json:"kind"`
}

// FileSlice is a small code snippet for neighborhood/context.
type FileSlice struct {
	File    string `json:"file"`
	Range   [2]int `json:"range"` // [start_line, end_line]
	Snippet string `json:"snippet"`
	Reason  string `json:"reason"`
}

// OutlineNode is a hierarchical representation of a file's symbols
type OutlineNode struct {
	Name     string        `json:"name"`
	Kind     string        `json:"kind"`
	Span     [2]int        `json:"span"`
	Children []OutlineNode `json:"children,omitempty"`
}

// Service provides heuristic symbol indexing for a workspace.
type Service struct {
	workspacePath string
	mu            sync.RWMutex
	fileVersion   map[string]string // file -> content hash
	symbols       map[string]Symbol // sid -> symbol
	byFile        map[string][]string
	refs          map[string][]RefSite // sid -> refs
	lastIndex     time.Time
}

// NewService creates a new in-memory symbol service.
func NewService(workspacePath string) (*Service, error) {
	absspath, err := filepath.Abs(strings.TrimSpace(workspacePath))
	if err != nil {
		return nil, fmt.Errorf("abs workspace: %w", err)
	}
	return &Service{
		workspacePath: absspath,
		fileVersion:   make(map[string]string),
		symbols:       make(map[string]Symbol),
		byFile:        make(map[string][]string),
		refs:          make(map[string][]RefSite),
	}, nil
}

// Close releases resources (no-op for in-memory service).
func (s *Service) Close() error { return nil }

// StartIndexing performs a full scan. Incremental updates can be added later with fsnotify.
func (s *Service) StartIndexing(ctx context.Context) error { return s.IndexAll(ctx) }

// IndexAll walks the workspace and (re)indexes changed files.
func (s *Service) IndexAll(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Reset caches
	s.fileVersion = make(map[string]string)
	s.symbols = make(map[string]Symbol)
	s.byFile = make(map[string][]string)
	s.refs = make(map[string][]RefSite)
	// Walk
	return filepath.WalkDir(s.workspacePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(s.workspacePath, path)
		if d.IsDir() {
			if ignoreDirName(d.Name()) || ignorePath(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if ignorePath(rel) {
			return nil
		}
		return s.indexFileUnlocked(ctx, rel)
	})
}

// IndexFile reindexes a single file.
func (s *Service) IndexFile(ctx context.Context, relPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.indexFileUnlocked(ctx, relPath)
}

func (s *Service) indexFileUnlocked(ctx context.Context, relPath string) error {
	abs := filepath.Join(s.workspacePath, relPath)
	info, err := os.Stat(abs)
	if err != nil {
		return nil
	}
	if info.IsDir() {
		return nil
	}
	if info.Size() > 1_500_000 {
		return nil
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil
	}
	version := hashBytes(data)
	if s.fileVersion[relPath] == version {
		return nil
	}
	lang := detectLanguage(abs)
	syms, rels := parseFile(relPath, string(data), lang)
	// Remove old
	if old, ok := s.byFile[relPath]; ok {
		for _, sid := range old {
			delete(s.symbols, sid)
			delete(s.refs, sid)
		}
	}
	// Insert
	var ids []string
	for _, sym := range syms {
		sym.Version = version
		s.symbols[sym.SID] = sym
		ids = append(ids, sym.SID)
	}
	s.byFile[relPath] = ids
	for _, r := range rels {
		s.refs[r.ToSID] = append(s.refs[r.ToSID], RefSite{File: r.FilePath, LineStart: r.LineStart, LineEnd: r.LineEnd, Kind: r.Kind})
	}
	s.fileVersion[relPath] = version
	s.lastIndex = time.Now()
	return nil
}

// Search returns top matching symbol cards.
func (s *Service) Search(ctx context.Context, q, kind, lang, pathPrefix string, limit int) ([]SymbolCard, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, nil
	}
	// Lazy reindex if older than 2 minutes
	if time.Since(s.lastIndex) > 2*time.Minute {
		_ = s.IndexAll(ctx)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []SymbolCard
	for _, sym := range s.symbols {
		if pathPrefix != "" && !strings.HasPrefix(sym.FilePath, pathPrefix) {
			continue
		}
		if kind != "" && sym.Kind != kind {
			continue
		}
		if lang != "" && sym.Lang != lang {
			continue
		}
		ql := strings.ToLower(q)
		nameMatch := strings.EqualFold(sym.Name, q) || strings.Contains(strings.ToLower(sym.Name), ql)
		docMatch := strings.Contains(strings.ToLower(sym.DocExcerpt), ql)
		pathMatch := strings.Contains(strings.ToLower(sym.FilePath), ql)
		if !nameMatch && !docMatch && !pathMatch {
			continue
		}
		why := ""
		if nameMatch {
			why = "matched name"
		} else if docMatch {
			why = "matched doc"
		} else if pathMatch {
			why = "matched path"
		}
		out = append(out, SymbolCard{SID: sym.SID, Name: sym.Name, Kind: sym.Kind, File: sym.FilePath, Span: [4]int{sym.LineStart, sym.ColStart, sym.LineEnd, sym.ColEnd}, Container: sym.ContainerSID, Signature: sym.Signature, DocExcerpt: sym.DocExcerpt, Confidence: sym.Confidence, Lang: sym.Lang, Why: why})
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if strings.EqualFold(a.Name, q) && !strings.EqualFold(b.Name, q) {
			return true
		}
		if a.Confidence != b.Confidence {
			return a.Confidence > b.Confidence
		}
		return a.Name < b.Name
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// Def returns a single symbol card.
func (s *Service) Def(ctx context.Context, sid string) (*SymbolCard, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sy, ok := s.symbols[sid]
	if !ok {
		return nil, fmt.Errorf("symbol not found")
	}
	c := SymbolCard{SID: sy.SID, Name: sy.Name, Kind: sy.Kind, File: sy.FilePath, Span: [4]int{sy.LineStart, sy.ColStart, sy.LineEnd, sy.ColEnd}, Container: sy.ContainerSID, Signature: sy.Signature, DocExcerpt: sy.DocExcerpt, Confidence: sy.Confidence, Lang: sy.Lang}
	return &c, nil
}

// Refs returns reference sites for a given symbol id.
func (s *Service) Refs(ctx context.Context, sid, kind string) ([]RefSite, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	refs := s.refs[sid]
	if kind == "" {
		return refs, nil
	}
	var out []RefSite
	for _, r := range refs {
		if r.Kind == kind {
			out = append(out, r)
		}
	}
	return out, nil
}

// Neighborhood returns a small snippet around the symbol definition lines.
func (s *Service) Neighborhood(ctx context.Context, sid string, radius int) ([]FileSlice, error) {
	if radius <= 0 {
		radius = 40
	}
	s.mu.RLock()
	sy, ok := s.symbols[sid]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("symbol not found")
	}
	abs := filepath.Join(s.workspacePath, sy.FilePath)
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	start := max(1, sy.LineStart-radius)
	end := min(len(lines), sy.LineEnd+radius)
	snippet := sliceWithLineNumbers(lines, start, end)
	return []FileSlice{{File: sy.FilePath, Range: [2]int{start, end}, Snippet: snippet, Reason: "definition neighborhood"}}, nil
}

// Outline returns a hierarchical outline of a file's symbols.
func (s *Service) Outline(ctx context.Context, relPath string) ([]OutlineNode, error) {
	abs := filepath.Join(s.workspacePath, relPath)
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	lang := detectLanguage(abs)
	syms, _ := parseFile(relPath, string(data), lang)
	idToNode := make(map[string]*OutlineNode)
	var roots []*OutlineNode
	for _, sy := range syms {
		n := &OutlineNode{Name: sy.Name, Kind: sy.Kind, Span: [2]int{sy.LineStart, sy.LineEnd}}
		idToNode[sy.SID] = n
	}
	for _, sy := range syms {
		n := idToNode[sy.SID]
		if sy.ContainerSID == "" || idToNode[sy.ContainerSID] == nil {
			roots = append(roots, n)
			continue
		}
		parent := idToNode[sy.ContainerSID]
		parent.Children = append(parent.Children, *n)
	}
	var out []OutlineNode
	for _, r := range roots {
		out = append(out, *r)
	}
	return out, nil
}

// Workspace returns the absolute workspace path backing this service.
func (s *Service) Workspace() string { return s.workspacePath }

// Helpers
func ignorePath(rel string) bool {
	name := filepath.Base(rel)
	if ignoreDirName(name) {
		return true
	}
	if strings.HasSuffix(strings.ToLower(name), ".min.js") {
		return true
	}
	if strings.HasPrefix(name, ".") && name != ".env" && name != ".gitignore" {
		return true
	}
	return false
}

func ignoreDirName(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", "dist", "build", ".next", "out", "target", "bin", "obj", "coverage":
		return true
	default:
		return false
	}
}
func detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".py":
		return "python"
	case ".java":
		return "java"
	case ".rs":
		return "rust"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".c", ".h", ".cpp", ".hpp", ".cc":
		return "c++"
	default:
		return "text"
	}
}

func hashBytes(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func sliceWithLineNumbers(lines []string, start, end int) string {
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	var b strings.Builder
	for i := start; i <= end; i++ {
		b.WriteString(fmt.Sprintf("L%d: %s\n", i, lines[i-1]))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// ================= Heuristic Parsing =================

var (
	goFuncRe     = regexp.MustCompile(`^\s*func\s+(\([^)]*\)\s*)?(?P<name>[A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	tsFuncRe     = regexp.MustCompile(`^\s*(export\s+)?(async\s+)?function\s+(?P<name>[A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	tsConstRe    = regexp.MustCompile(`^\s*(export\s+)?(const|let|var)\s+(?P<name>[A-Za-z_][A-Za-z0-9_]*)\s*=\s*(async\s*)?\(?[A-Za-z0-9_,\s]*\)?\s*=>`)
	jsClassRe    = regexp.MustCompile(`^\s*(export\s+)?class\s+(?P<name>[A-Za-z_][A-Za-z0-9_]*)`)
	pyDefRe      = regexp.MustCompile(`^\s*def\s+(?P<name>[A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	pyClassRe    = regexp.MustCompile(`^\s*class\s+(?P<name>[A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	javaClassRe  = regexp.MustCompile(`^\s*(public|private|protected)?\s*(abstract\s+|final\s+)?class\s+(?P<name>[A-Za-z_][A-Za-z0-9_]*)`)
	javaMethodRe = regexp.MustCompile(`^\s*(public|private|protected)?[\w\<\>\[\]\s]+\s+(?P<name>[A-Za-z_][A-Za-z0-9_]*)\s*\(`)
)

// reserved for future container/outline improvements
// type partialSym struct {
// 	name      string
// 	kind      string
// 	startLine int
// 	startCol  int
// 	endLine   int
// 	endCol    int
// 	signature string
// 	doc       string
// 	container string
// }

// parseFile extracts symbols and relations from a file's contents.
func parseFile(relPath, content, lang string) ([]Symbol, []Relation) {
	lines := strings.Split(content, "\n")
	var out []Symbol
	var rels []Relation
	for i, line := range lines {
		lineNo := i + 1
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		var name, kind, sig string
		confidence := 0.6
		switch lang {
		case "go":
			if goFuncRe.MatchString(line) {
				name = extractNamedGroup(goFuncRe, line, "name")
				kind = "func"
				sig = strings.TrimSpace(line)
				confidence = 0.9
			}
		case "typescript", "javascript":
			if tsFuncRe.MatchString(line) {
				name = extractNamedGroup(tsFuncRe, line, "name")
				kind = "func"
				sig = strings.TrimSpace(line)
				confidence = 0.85
			}
			if name == "" && tsConstRe.MatchString(line) {
				name = extractNamedGroup(tsConstRe, line, "name")
				kind = "var"
				sig = strings.TrimSpace(line)
				confidence = 0.7
			}
			if name == "" && jsClassRe.MatchString(line) {
				name = extractNamedGroup(jsClassRe, line, "name")
				kind = "class"
				sig = strings.TrimSpace(line)
				confidence = 0.8
			}
		case "python":
			if pyDefRe.MatchString(line) {
				name = extractNamedGroup(pyDefRe, line, "name")
				kind = "func"
				sig = strings.TrimSpace(line)
				confidence = 0.85
			}
			if name == "" && pyClassRe.MatchString(line) {
				name = extractNamedGroup(pyClassRe, line, "name")
				kind = "class"
				sig = strings.TrimSpace(line)
				confidence = 0.8
			}
		case "java":
			if javaClassRe.MatchString(line) {
				name = extractNamedGroup(javaClassRe, line, "name")
				kind = "class"
				sig = strings.TrimSpace(line)
				confidence = 0.8
			}
			if name == "" && javaMethodRe.MatchString(line) {
				name = extractNamedGroup(javaMethodRe, line, "name")
				kind = "func"
				sig = strings.TrimSpace(line)
				confidence = 0.7
			}
		default:
			if strings.Contains(line, "(") && strings.Contains(strings.ToLower(line), "function") {
				kind = "func"
				confidence = 0.5
				name = guessNameFromSignature(line)
				sig = strings.TrimSpace(line)
			}
			if name == "" && strings.HasPrefix(trim, "class ") {
				kind = "class"
				confidence = 0.5
				parts := strings.Fields(trim)
				if len(parts) > 1 {
					name = parts[1]
				}
				sig = trim
			}
		}
		if name == "" {
			continue
		}
		doc := gatherDocAbove(lines, i)
		endLine, endCol := estimateEnd(lines, i, lang)
		sym := Symbol{
			SID:        makeSID(relPath, lineNo, kind, name),
			FilePath:   relPath,
			LineStart:  lineNo,
			ColStart:   1,
			LineEnd:    endLine,
			ColEnd:     endCol,
			Lang:       lang,
			Name:       name,
			Kind:       kind,
			Signature:  sig,
			DocExcerpt: doc,
			Confidence: confidence,
		}
		out = append(out, sym)
		refs := sampleRefs(lines, name, relPath)
		for _, r := range refs {
			rels = append(rels, Relation{FromSID: sym.SID, ToSID: sym.SID, Kind: r.Kind, FilePath: relPath, LineStart: r.LineStart, LineEnd: r.LineEnd})
		}
	}
	return out, rels
}

func makeSID(relPath string, startLine int, kind, name string) string {
	base := fmt.Sprintf("%s:%d:%s:%s", relPath, startLine, kind, name)
	s := sha256.Sum256([]byte(base))
	return hex.EncodeToString(s[:])[:24]
}
func estimateEnd(lines []string, idx int, lang string) (int, int) {
	maxScan := min(len(lines)-1, idx+200)
	braceDepth := 0
	indentBase := leadingSpaces(lines[idx])
	for i := idx; i <= maxScan; i++ {
		line := lines[i]
		trim := strings.TrimSpace(line)
		if lang == "python" {
			if i > idx && leadingSpaces(line) <= indentBase && (strings.HasPrefix(trim, "def ") || strings.HasPrefix(trim, "class ")) {
				return i, len(line)
			}
			continue
		}
		braceDepth += strings.Count(line, "{") - strings.Count(line, "}")
		if i > idx && braceDepth <= 0 && (strings.Contains(trim, "func ") || strings.HasPrefix(trim, "class ") || strings.HasPrefix(trim, "function ")) {
			return i, len(line)
		}
	}
	return min(len(lines), idx+40), len(lines[min(len(lines)-1, idx+40)])
}

func leadingSpaces(s string) int { return len(s) - len(strings.TrimLeft(s, " \t")) }
func gatherDocAbove(lines []string, idx int) string {
	start := max(0, idx-3)
	var b []string
	for i := start; i < idx; i++ {
		trim := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trim, "//") || strings.HasPrefix(trim, "#") || strings.HasPrefix(trim, "/*") || strings.HasPrefix(trim, "*") {
			b = append(b, strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(trim, "//"), "#"), "*")))
		}
	}
	return strings.TrimSpace(strings.Join(b, "\n"))
}

func sampleRefs(lines []string, name, relPath string) []RefSite {
	var out []RefSite
	pat := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b\s*(\(|=)`) // call or assignment
	for i, line := range lines {
		if pat.FindStringIndex(line) != nil {
			ln := i + 1
			out = append(out, RefSite{File: relPath, LineStart: ln, LineEnd: ln, Kind: "references"})
		}
		if len(out) >= 8 {
			break
		}
	}
	return out
}

func extractNamedGroup(re *regexp.Regexp, s, name string) string {
	if re == nil {
		return ""
	}
	idx := re.SubexpIndex(name)
	if idx < 0 {
		return ""
	}
	match := re.FindStringSubmatch(s)
	if match == nil || idx >= len(match) {
		return ""
	}
	return match[idx]
}

func guessNameFromSignature(line string) string {
	pos := strings.Index(line, "(")
	if pos <= 0 {
		return ""
	}
	pre := strings.TrimSpace(line[:pos])
	parts := strings.Fields(pre)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
