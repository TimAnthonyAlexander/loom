package shared

import (
	"path/filepath"
	"strings"
)

// Graph represents the import/dependency graph
type Graph struct {
	Edges    map[string]map[string]float64 // adjacency map: from -> to -> weight
	Vertices map[string]bool               // all vertices in the graph
}

// NewGraph creates a new empty graph
func NewGraph() *Graph {
	return &Graph{
		Edges:    make(map[string]map[string]float64),
		Vertices: make(map[string]bool),
	}
}

// AddEdge adds a weighted edge from source to target
func (g *Graph) AddEdge(from, to string, weight float64) {
	// Normalize paths for cross-platform compatibility
	from = NormalizePath(from)
	to = NormalizePath(to)

	g.Vertices[from] = true
	g.Vertices[to] = true

	if g.Edges[from] == nil {
		g.Edges[from] = make(map[string]float64)
	}

	// Add to existing weight if edge already exists
	g.Edges[from][to] += weight
}

// GetVertices returns all vertices in the graph
func (g *Graph) GetVertices() []string {
	vertices := make([]string, 0, len(g.Vertices))
	for v := range g.Vertices {
		vertices = append(vertices, v)
	}
	return vertices
}

// NormalizePath converts a path to POSIX format for cross-platform storage
func NormalizePath(path string) string {
	// Convert all backslashes to forward slashes for POSIX compatibility
	normalized := strings.ReplaceAll(path, "\\", "/")

	// Clean the path to remove redundant separators and dots
	normalized = filepath.ToSlash(filepath.Clean(normalized))

	// Ensure no leading slash for relative paths
	if strings.HasPrefix(normalized, "/") && !filepath.IsAbs(path) {
		normalized = strings.TrimPrefix(normalized, "/")
	}

	return normalized
}

// PackageInfo represents a detected package in a monorepo
type PackageInfo struct {
	Root         string   `json:"root"`     // Package root directory
	Type         string   `json:"type"`     // "npm", "go", "composer", etc.
	ManifestPath string   `json:"manifest"` // Path to package.json, go.mod, etc.
	Files        []string `json:"files"`    // Files belonging to this package
}

// MonorepoInfo tracks packages in a monorepo
type MonorepoInfo struct {
	IsMonorepo bool          `json:"is_monorepo"`
	Packages   []PackageInfo `json:"packages"`
}

// FileInfo represents information about a scanned file
type FileInfo struct {
	Path        string
	Size        int64
	Extension   string
	Basename    string
	IsConfig    bool
	IsDoc       bool
	IsScript    bool
	IsGenerated bool
	IsVendored  bool
}

// LangGraphData contains the data needed by language graph builders
type LangGraphData struct {
	TSFiles     []string
	GoFiles     []string
	PHPFiles    []string
	TSConfig    map[string]interface{}
	ComposerPSR map[string]string
	ScriptRefs  map[string][]string
	CIRefs      map[string][]string
	DocRefs     []string
}

// Types needed by scoring system
type HeuristicWeights struct {
	GraphCentrality float64 `json:"graph_centrality"`
	GitRecency      float64 `json:"git_recency"`
	GitFrequency    float64 `json:"git_frequency"`
	ScriptRefs      float64 `json:"script_ci_refs"`
	DocMentions     float64 `json:"doc_mentions"`
	CapMax          float64 `json:"cap_max"`
}

type ImportantFile struct {
	Path        string             `json:"path"`
	Score       float64            `json:"score"`
	Reasons     []string           `json:"reasons"`
	Components  map[string]float64 `json:"components"`
	Penalties   map[string]float64 `json:"penalties"`
	Confidence  float64            `json:"confidence"`
	IsGenerated bool               `json:"is_generated"`
}

type SignalData struct {
	TSFiles        []string
	GoFiles        []string
	PHPFiles       []string
	TSConfig       map[string]interface{}
	ComposerPSR    map[string]string
	ScriptRefs     map[string][]string
	CIRefs         map[string][]string
	DocRefs        []string
	Manifests      []string
	Entrypoints    []EntryPoint
	Scripts        []Script
	CI             []CIConfig
	Configs        []ConfigFile
	Codegen        []CodegenSpec
	RoutesServices []RouteOrService
}

type EntryPoint struct {
	Path  string   `json:"path"`
	Kind  string   `json:"kind"`
	Hints []string `json:"hints"`
}

type Script struct {
	Name   string   `json:"name"`
	Source string   `json:"source"`
	Cmd    string   `json:"cmd"`
	Paths  []string `json:"paths"`
}

type CIConfig struct {
	Path string   `json:"path"`
	Jobs []string `json:"jobs"`
}

type ConfigFile struct {
	Tool string `json:"tool"`
	Path string `json:"path"`
}

type CodegenSpec struct {
	Tool  string   `json:"tool"`
	Paths []string `json:"paths"`
}

type RouteOrService struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
	Name string `json:"name"`
}

// InputSignature tracks hashes of key files for drift detection
type InputSignature struct {
	ManifestHashes map[string]string `json:"manifest_hashes"`
	TSConfigHash   string            `json:"tsconfig_hash"`
	ReadmeHash     string            `json:"readme_hash"`
	MtimeMax       int64             `json:"mtime_max"`
}

// GitStatsMode indicates how git statistics were computed
type GitStatsMode struct {
	Mode       string `json:"mode"` // "none", "gitdir", "touchlog"
	WindowDays int    `json:"window_days"`
}

// ProfilerMetrics tracks profiling performance and quality metrics
type ProfilerMetrics struct {
	Files         int     `json:"files"`
	Edges         int     `json:"edges"`
	PageRankIters int     `json:"pagerank_iters"`
	DurationMs    int64   `json:"duration_ms"`
	RankChurn     float64 `json:"rank_churn,omitempty"` // Jaccard distance from previous top-20
}

// Profile represents the complete analysis of a workspace
type Profile struct {
	WorkspaceRoot  string             `json:"workspace_root"`
	CreatedAtUnix  int64              `json:"created_at_unix"`
	Languages      []string           `json:"languages"`
	Entrypoints    []EntryPoint       `json:"entrypoints"`
	Scripts        []Script           `json:"scripts"`
	CI             []CIConfig         `json:"ci"`
	Configs        []ConfigFile       `json:"configs"`
	Codegen        []CodegenSpec      `json:"codegen"`
	RoutesServices []RouteOrService   `json:"routes_services"`
	ImportantFiles []ImportantFile    `json:"important_files"`
	Heuristics     HeuristicWeights   `json:"heuristics"`
	GitStats       GitStatsMode       `json:"gitstats"`
	GitWindowDays  int                `json:"git_window_days"` // deprecated, use GitStats.WindowDays
	InputSignature InputSignature     `json:"input_signature"`
	Metrics        ProfilerMetrics    `json:"metrics"`
	ManualBoosts   map[string]float64 `json:"manual_boosts,omitempty"` // path -> boost value
	Version        string             `json:"version"`
}
