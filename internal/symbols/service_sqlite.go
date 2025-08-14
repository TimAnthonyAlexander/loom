package symbols

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bep/debounce"
	"github.com/fsnotify/fsnotify"
	_ "modernc.org/sqlite"
)

// SQLiteService persists symbols and relations in a per-project SQLite DB with FTS5.
type SQLiteService struct {
	workspacePath string
	db            *sql.DB
	mu            sync.RWMutex
	watcher       *fsnotify.Watcher
	debounceIndex func(func())
}

// NewSQLiteService creates the DB and initializes schema.
func NewSQLiteService(workspacePath string) (*SQLiteService, error) {
	ws, err := filepath.Abs(strings.TrimSpace(workspacePath))
	if err != nil {
		return nil, fmt.Errorf("abs: %w", err)
	}
	// ~/.loom/projects/<id>/symbols.db
	id := hashPath(ws)
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".loom", "projects", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	dsn := fmt.Sprintf("file:%s?_busy_timeout=8000&_fk=1", filepath.ToSlash(filepath.Join(dir, "symbols.db")))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := initSQLiteSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return &SQLiteService{workspacePath: ws, db: db, watcher: w, debounceIndex: debounce.New(500 * time.Millisecond)}, nil
}

func initSQLiteSchema(db *sql.DB) error {
	stmts := []string{
		`PRAGMA journal_mode=WAL;`,
		`CREATE TABLE IF NOT EXISTS symbols (
            sid TEXT PRIMARY KEY,
            file_path TEXT,
            line_start INTEGER,
            col_start INTEGER,
            line_end INTEGER,
            col_end INTEGER,
            lang TEXT,
            name TEXT,
            kind TEXT,
            container_sid TEXT,
            signature TEXT,
            doc_excerpt TEXT,
            confidence REAL,
            version TEXT
        );`,
		`CREATE TABLE IF NOT EXISTS relations (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            from_sid TEXT,
            to_sid TEXT,
            kind TEXT,
            file_path TEXT,
            line_start INTEGER,
            line_end INTEGER
        );`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS symbols_fts USING fts5(sid, name, doc_excerpt, file_path);`,
		`CREATE INDEX IF NOT EXISTS idx_symbols_file ON symbols(file_path);`,
		`CREATE INDEX IF NOT EXISTS idx_symbols_name ON symbols(name);`,
		`CREATE INDEX IF NOT EXISTS idx_rel_to ON relations(to_sid);`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}
	return nil
}

// Close closes watcher and DB.
func (s *SQLiteService) Close() error {
	if s.watcher != nil {
		_ = s.watcher.Close()
	}
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// StartIndexing performs initial full index and starts fsnotify watchers for incremental updates.
func (s *SQLiteService) StartIndexing(ctx context.Context) error {
	if err := s.IndexAll(ctx); err != nil {
		return err
	}
	if err := s.addWatchesRecursive(s.workspacePath); err != nil {
		return err
	}
	go s.watchLoop(ctx)
	return nil
}

func (s *SQLiteService) addWatchesRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if ignoreDirName(d.Name()) {
				return filepath.SkipDir
			}
			return s.watcher.Add(path)
		}
		return nil
	})
}

func (s *SQLiteService) watchLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-s.watcher.Events:
			if !ok {
				return
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) != 0 {
				rel, _ := filepath.Rel(s.workspacePath, ev.Name)
				if rel == "." || ignorePath(rel) {
					continue
				}
				s.debounceIndex(func() { _ = s.IndexFile(ctx, rel) })
			}
		case <-s.watcher.Errors:
			// ignore
		}
	}
}

// IndexAll walks workspace, deletes per-file rows and reinserts.
func (s *SQLiteService) IndexAll(ctx context.Context) error {
	return filepath.WalkDir(s.workspacePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
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
		return s.IndexFile(ctx, rel)
	})
}

// IndexFile indexes a single file (relative path).
func (s *SQLiteService) IndexFile(ctx context.Context, relPath string) error {
	abs := filepath.Join(s.workspacePath, relPath)
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() || info.Size() > 1_500_000 {
		return nil
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil
	}
	version := hashBytes(data)
	lang := detectLanguage(abs)
	syms, rels := parseFile(relPath, string(data), lang)
	for i := range syms {
		syms[i].Version = version
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM relations WHERE file_path = ?`, relPath); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM symbols WHERE file_path = ?`, relPath); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM symbols_fts WHERE file_path = ?`, relPath); err != nil {
		return err
	}
	for _, srec := range syms {
		if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO symbols (sid,file_path,line_start,col_start,line_end,col_end,lang,name,kind,container_sid,signature,doc_excerpt,confidence,version) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, srec.SID, srec.FilePath, srec.LineStart, srec.ColStart, srec.LineEnd, srec.ColEnd, srec.Lang, srec.Name, srec.Kind, srec.ContainerSID, srec.Signature, srec.DocExcerpt, srec.Confidence, srec.Version); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO symbols_fts (sid,name,doc_excerpt,file_path) VALUES (?,?,?,?)`, srec.SID, srec.Name, srec.DocExcerpt, srec.FilePath); err != nil {
			return err
		}
	}
	for _, r := range rels {
		if _, err := tx.ExecContext(ctx, `INSERT INTO relations (from_sid,to_sid,kind,file_path,line_start,line_end) VALUES (?,?,?,?,?,?)`, r.FromSID, r.ToSID, r.Kind, r.FilePath, r.LineStart, r.LineEnd); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// Search queries FTS with optional filters and ranking.
func (s *SQLiteService) Search(ctx context.Context, q, kind, lang, pathPrefix string, limit int) ([]SymbolCard, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, nil
	}
	// Primary path: FTS against name/doc_excerpt/file_path
	var rows *sql.Rows
	var err error
	if kind == "" && lang == "" && pathPrefix == "" {
		rows, err = s.db.QueryContext(ctx, `
            SELECT s.sid,s.name,s.kind,s.file_path,s.line_start,s.col_start,s.line_end,s.col_end,s.container_sid,s.signature,s.doc_excerpt,s.confidence,s.lang
            FROM symbols_fts f JOIN symbols s ON s.sid=f.sid
            WHERE symbols_fts MATCH ?
            LIMIT ?`, q, limit)
	} else {
		base := `SELECT sid,name,kind,file_path,line_start,col_start,line_end,col_end,container_sid,signature,doc_excerpt,confidence,lang FROM symbols WHERE (name LIKE ? OR file_path LIKE ?)`
		args := []any{"%" + q + "%", "%" + q + "%"}
		if kind != "" {
			base += ` AND kind = ?`
			args = append(args, kind)
		}
		if lang != "" {
			base += ` AND lang = ?`
			args = append(args, lang)
		}
		if pathPrefix != "" {
			base += ` AND file_path LIKE ?`
			args = append(args, pathPrefix+"%")
		}
		base += ` ORDER BY confidence DESC LIMIT ?`
		args = append(args, limit)
		rows, err = s.db.QueryContext(ctx, base, args...)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SymbolCard
	for rows.Next() {
		var c SymbolCard
		var ls, cs, le, ce int
		var container string
		if err := rows.Scan(&c.SID, &c.Name, &c.Kind, &c.File, &ls, &cs, &le, &ce, &container, &c.Signature, &c.DocExcerpt, &c.Confidence, &c.Lang); err != nil {
			return nil, err
		}
		c.Span = [4]int{ls, cs, le, ce}
		c.Container = container
		// Why signal (best-effort)
		if strings.EqualFold(c.Name, q) {
			c.Why = "matched name"
		} else if strings.Contains(strings.ToLower(c.DocExcerpt), strings.ToLower(q)) {
			c.Why = "matched doc"
		} else {
			c.Why = "matched path"
		}
		out = append(out, c)
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
	return out, nil
}

// Def loads a single symbol card.
func (s *SQLiteService) Def(ctx context.Context, sid string) (*SymbolCard, error) {
	row := s.db.QueryRowContext(ctx, `SELECT sid,name,kind,file_path,line_start,col_start,line_end,col_end,container_sid,signature,doc_excerpt,confidence,lang FROM symbols WHERE sid = ?`, sid)
	var c SymbolCard
	var ls, cs, le, ce int
	var container string
	if err := row.Scan(&c.SID, &c.Name, &c.Kind, &c.File, &ls, &cs, &le, &ce, &container, &c.Signature, &c.DocExcerpt, &c.Confidence, &c.Lang); err != nil {
		return nil, err
	}
	c.Span = [4]int{ls, cs, le, ce}
	c.Container = container
	return &c, nil
}

// Refs returns reference sites for a given symbol id.
func (s *SQLiteService) Refs(ctx context.Context, sid, kind string) ([]RefSite, error) {
	q := `SELECT file_path,line_start,line_end,kind FROM relations WHERE to_sid = ?`
	args := []any{sid}
	if kind != "" {
		q += ` AND kind = ?`
		args = append(args, kind)
	}
	q += ` ORDER BY file_path,line_start`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RefSite
	for rows.Next() {
		var r RefSite
		if err := rows.Scan(&r.File, &r.LineStart, &r.LineEnd, &r.Kind); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// Neighborhood returns a small snippet around the symbol definition lines.
func (s *SQLiteService) Neighborhood(ctx context.Context, sid string, radius int) ([]FileSlice, error) {
	if radius <= 0 {
		radius = 40
	}
	row := s.db.QueryRowContext(ctx, `SELECT file_path,line_start,line_end FROM symbols WHERE sid = ?`, sid)
	var file string
	var ls, le int
	if err := row.Scan(&file, &ls, &le); err != nil {
		return nil, err
	}
	abs := filepath.Join(s.workspacePath, file)
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	start := max(1, ls-radius)
	end := min(len(lines), le+radius)
	snippet := sliceWithLineNumbers(lines, start, end)
	return []FileSlice{{File: file, Range: [2]int{start, end}, Snippet: snippet, Reason: "definition neighborhood"}}, nil
}

// Outline returns a hierarchical outline of a file's symbols using on-the-fly parse.
func (s *SQLiteService) Outline(ctx context.Context, relPath string) ([]OutlineNode, error) {
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
		} else {
			parent := idToNode[sy.ContainerSID]
			parent.Children = append(parent.Children, *n)
		}
	}
	var out []OutlineNode
	for _, r := range roots {
		out = append(out, *r)
	}
	return out, nil
}

// Workspace returns the root path.
func (s *SQLiteService) Workspace() string { return s.workspacePath }

// ===== Shared helpers (duplicated small ones to avoid import cycles) =====
func hashPath(p string) string {
	sum := sha256.Sum256([]byte(p))
	return hex.EncodeToString(sum[:])[:16]
}
