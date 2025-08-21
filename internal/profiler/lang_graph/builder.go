package lang_graph

import (
	"github.com/loom/loom/internal/profiler/shared"
)

// Builder orchestrates all language-specific graph builders
type Builder struct {
	root           string
	graph          *shared.Graph
	tsBuilder      *TSGraphBuilder
	goBuilder      *GoGraphBuilder
	phpBuilder     *PHPGraphBuilder
	genericBuilder *GenericGraphBuilder
}

// NewBuilder creates a new graph builder
func NewBuilder(root string) *Builder {
	graph := shared.NewGraph()

	return &Builder{
		root:           root,
		graph:          graph,
		tsBuilder:      NewTSGraphBuilder(root, graph),
		goBuilder:      NewGoGraphBuilder(root, graph),
		phpBuilder:     NewPHPGraphBuilder(root, graph),
		genericBuilder: NewGenericGraphBuilder(graph),
	}
}

// Build builds the complete import/dependency graph
func (b *Builder) Build(data *shared.LangGraphData) (*shared.Graph, error) {
	// Build TypeScript/JavaScript graph
	if len(data.TSFiles) > 0 {
		_ = b.tsBuilder.Build(data.TSFiles, data.TSConfig)
	}

	// Build Go graph
	if len(data.GoFiles) > 0 {
		_ = b.goBuilder.Build(data.GoFiles)
	}

	// Build PHP graph
	if len(data.PHPFiles) > 0 {
		_ = b.phpBuilder.Build(data.PHPFiles, data.ComposerPSR)
	}

	// Add cross-language and script references
	b.genericBuilder.AddScriptRefs(data.ScriptRefs)
	b.genericBuilder.AddCIRefs(data.CIRefs)
	b.genericBuilder.AddDocRefs(data.DocRefs)

	// Clean up the graph
	b.finalizeGraph()

	return b.graph, nil
}

// finalizeGraph performs final cleanup and normalization
func (b *Builder) finalizeGraph() {
	// Remove self-loops except for important files
	b.removeTrivialSelfLoops()

	// Normalize edge weights
	b.genericBuilder.NormalizeEdgeWeights()

	// Remove virtual nodes used during analysis
	b.genericBuilder.RemoveVirtualNodes()

	// Cap edges per file to avoid pathological graphs
	b.capEdgesPerFile(500)
}

// removeTrivialSelfLoops removes self-loops that aren't marking important files
func (b *Builder) removeTrivialSelfLoops() {
	for from, edges := range b.graph.Edges {
		if weight, hasSelfLoop := edges[from]; hasSelfLoop {
			// Keep self-loops with weight > 1.0 (these mark important files)
			if weight <= 1.0 {
				delete(edges, from)
			}
		}
	}
}

// capEdgesPerFile limits the number of outgoing edges per file
func (b *Builder) capEdgesPerFile(maxEdges int) {
	for from, edges := range b.graph.Edges {
		if len(edges) <= maxEdges {
			continue
		}

		// Keep only the highest weighted edges
		type edge struct {
			to     string
			weight float64
		}

		var edgeList []edge
		for to, weight := range edges {
			edgeList = append(edgeList, edge{to: to, weight: weight})
		}

		// Sort by weight (descending)
		for i := 0; i < len(edgeList)-1; i++ {
			for j := i + 1; j < len(edgeList); j++ {
				if edgeList[j].weight > edgeList[i].weight {
					edgeList[i], edgeList[j] = edgeList[j], edgeList[i]
				}
			}
		}

		// Keep only top maxEdges
		newEdges := make(map[string]float64)
		for i := 0; i < maxEdges && i < len(edgeList); i++ {
			newEdges[edgeList[i].to] = edgeList[i].weight
		}

		b.graph.Edges[from] = newEdges
	}
}

// GetGraph returns the built graph
func (b *Builder) GetGraph() *shared.Graph {
	return b.graph
}

// AnalyzeLanguages returns detected languages in the project
func (b *Builder) AnalyzeLanguages(data *shared.LangGraphData) []string {
	var languages []string

	if len(data.TSFiles) > 0 {
		languages = append(languages, "typescript")
	}

	if len(data.GoFiles) > 0 {
		languages = append(languages, "go")
	}

	if len(data.PHPFiles) > 0 {
		languages = append(languages, "php")
	}

	// This would need to be implemented with file scanning
	// For now, just return the languages we actively support
	return languages
}

// GetGraphStatistics returns statistics about the built graph
func (b *Builder) GetGraphStatistics() GraphStats {
	stats := GraphStats{
		TotalVertices: len(b.graph.Vertices),
		TotalEdges:    0,
		MaxOutDegree:  0,
		AvgOutDegree:  0.0,
	}

	totalOutDegree := 0
	for _, edges := range b.graph.Edges {
		edgeCount := len(edges)
		stats.TotalEdges += edgeCount
		totalOutDegree += edgeCount

		if edgeCount > stats.MaxOutDegree {
			stats.MaxOutDegree = edgeCount
		}
	}

	if len(b.graph.Edges) > 0 {
		stats.AvgOutDegree = float64(totalOutDegree) / float64(len(b.graph.Edges))
	}

	return stats
}

// GraphStats contains statistics about the graph
type GraphStats struct {
	TotalVertices int
	TotalEdges    int
	MaxOutDegree  int
	AvgOutDegree  float64
}
