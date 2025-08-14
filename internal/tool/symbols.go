package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/loom/loom/internal/symbols"
)

type SymbolsSearchArgs struct {
	Q          string `json:"q"`
	Kind       string `json:"kind,omitempty"`
	Lang       string `json:"lang,omitempty"`
	PathPrefix string `json:"path_prefix,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

type SymbolsDefArgs struct {
	SID string `json:"sid"`
}
type SymbolsRefsArgs struct {
	SID  string `json:"sid"`
	Kind string `json:"kind,omitempty"`
}
type SymbolsNeighborhoodArgs struct {
	SID    string `json:"sid"`
	Radius int    `json:"radius_lines,omitempty"`
}
type SymbolsOutlineArgs struct {
	File string `json:"file"`
}
type SymbolsContextPackArgs struct {
	SID         string `json:"sid"`
	BudgetLines int    `json:"budget_lines,omitempty"`
	MaxSlices   int    `json:"max_slices,omitempty"`
}

type ContextPack struct {
	Card       symbols.SymbolCard  `json:"card"`
	Definition symbols.FileSlice   `json:"definition"`
	Calls      []symbols.FileSlice `json:"calls"`
	TotalLines int                 `json:"total_lines"`
}

// SymbolService defines the API needed by the symbol tools.
type SymbolService interface {
	Search(ctx context.Context, q, kind, lang, pathPrefix string, limit int) ([]symbols.SymbolCard, error)
	Def(ctx context.Context, sid string) (*symbols.SymbolCard, error)
	Refs(ctx context.Context, sid, kind string) ([]symbols.RefSite, error)
	Neighborhood(ctx context.Context, sid string, radius int) ([]symbols.FileSlice, error)
	Outline(ctx context.Context, relPath string) ([]symbols.OutlineNode, error)
	Workspace() string
}

// RegisterSymbols registers all symbol tools.
func RegisterSymbols(registry *Registry, svc SymbolService) error {
	// symbols_search
	if err := registry.Register(Definition{
		Name:        "symbols_search",
		Description: "Search indexed project symbols by name or doc excerpt",
		Safe:        true,
		JSONSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"q":           map[string]any{"type": "string", "description": "query string (name or text)"},
				"kind":        map[string]any{"type": "string", "description": "optional symbol kind (func, class, var, ...)"},
				"lang":        map[string]any{"type": "string", "description": "optional language filter"},
				"path_prefix": map[string]any{"type": "string", "description": "limit to files under this prefix"},
				"limit":       map[string]any{"type": "integer", "description": "max results (default 20)"},
			},
			"required": []any{"q"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			var args SymbolsSearchArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, fmt.Errorf("parse args: %w", err)
			}
			return svc.Search(ctx, args.Q, args.Kind, args.Lang, args.PathPrefix, args.Limit)
		},
	}); err != nil {
		return err
	}

	// symbols_def
	if err := registry.Register(Definition{
		Name:        "symbols_def",
		Description: "Get a symbol card by id",
		Safe:        true,
		JSONSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"sid": map[string]any{"type": "string"}},
			"required":   []any{"sid"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			var args SymbolsDefArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
			return svc.Def(ctx, args.SID)
		},
	}); err != nil {
		return err
	}

	// symbols_refs
	if err := registry.Register(Definition{
		Name:        "symbols_refs",
		Description: "Find reference/call/import sites for a symbol",
		Safe:        true,
		JSONSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"sid": map[string]any{"type": "string"}, "kind": map[string]any{"type": "string"}},
			"required":   []any{"sid"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			var args SymbolsRefsArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
			return svc.Refs(ctx, args.SID, args.Kind)
		},
	}); err != nil {
		return err
	}

	// symbols_neighborhood
	if err := registry.Register(Definition{
		Name:        "symbols_neighborhood",
		Description: "Get a small code slice around the symbol definition",
		Safe:        true,
		JSONSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"sid": map[string]any{"type": "string"}, "radius_lines": map[string]any{"type": "integer"}},
			"required":   []any{"sid"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			var args SymbolsNeighborhoodArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
			return svc.Neighborhood(ctx, args.SID, args.Radius)
		},
	}); err != nil {
		return err
	}

	// symbols_outline
	if err := registry.Register(Definition{
		Name:        "symbols_outline",
		Description: "Return a hierarchical outline of a file",
		Safe:        true,
		JSONSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"file": map[string]any{"type": "string"}},
			"required":   []any{"file"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			var args SymbolsOutlineArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
			return svc.Outline(ctx, args.File)
		},
	}); err != nil {
		return err
	}

	// symbols_context_pack
	if err := registry.Register(Definition{
		Name:        "symbols_context_pack",
		Description: "Pack a symbol definition slice and limited callsite slices into a compact context within a line budget",
		Safe:        true,
		JSONSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"sid":          map[string]any{"type": "string"},
				"budget_lines": map[string]any{"type": "integer", "description": "max total lines across all slices (default 200)"},
				"max_slices":   map[string]any{"type": "integer", "description": "max number of callsite slices (default 5)"},
			},
			"required": []any{"sid"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			var args SymbolsContextPackArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
			if args.BudgetLines <= 0 {
				args.BudgetLines = 200
			}
			if args.MaxSlices <= 0 {
				args.MaxSlices = 5
			}
			card, err := svc.Def(ctx, args.SID)
			if err != nil {
				return nil, err
			}
			neigh, err := svc.Neighborhood(ctx, args.SID, 40)
			if err != nil || len(neigh) == 0 {
				return nil, fmt.Errorf("failed to fetch definition slice")
			}
			pack := ContextPack{Card: *card, Definition: neigh[0]}
			remaining := args.BudgetLines - (neigh[0].Range[1] - neigh[0].Range[0] + 1)
			if remaining < 0 {
				remaining = 0
			}
			// Rank refs by proximity (same file first), then by line distance to def
			refs, _ := svc.Refs(ctx, args.SID, "")
			// Convert to slices around each ref (radius 12) until budget exhausted
			workspace := svc.Workspace()
			for _, r := range refs {
				if len(pack.Calls) >= args.MaxSlices || remaining <= 0 {
					break
				}
				slice, lines := readSliceWithNumbers(workspace, r.File, max(1, r.LineStart-12), r.LineEnd+12, "callsite")
				if lines <= 0 {
					continue
				}
				if lines > remaining {
					continue
				}
				pack.Calls = append(pack.Calls, slice)
				remaining -= lines
			}
			pack.TotalLines = args.BudgetLines - remaining
			return pack, nil
		},
	}); err != nil {
		return err
	}

	return nil
}

func readSliceWithNumbers(workspace, rel string, start, end int, reason string) (symbols.FileSlice, int) {
	abs := rel
	if !strings.HasPrefix(rel, string(filepath.Separator)) {
		abs = filepath.Join(workspace, rel)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return symbols.FileSlice{}, 0
	}
	lines := strings.Split(string(data), "\n")
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
	snippet := strings.TrimSuffix(b.String(), "\n")
	return symbols.FileSlice{File: rel, Range: [2]int{start, end}, Snippet: snippet, Reason: reason}, (end - start + 1)
}
