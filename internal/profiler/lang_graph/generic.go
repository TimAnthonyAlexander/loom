package lang_graph

import (
	"github.com/loom/loom/internal/profiler/shared"
)

// GenericGraphBuilder handles script references and cross-language edges
type GenericGraphBuilder struct {
	graph *shared.Graph
}

// NewGenericGraphBuilder creates a new generic graph builder
func NewGenericGraphBuilder(graph *shared.Graph) *GenericGraphBuilder {
	return &GenericGraphBuilder{
		graph: graph,
	}
}

// AddScriptRefs adds edges from scripts to files they reference
func (g *GenericGraphBuilder) AddScriptRefs(scriptRefs map[string][]string) {
	for scriptName, paths := range scriptRefs {
		// Create a virtual script node
		scriptNode := "script:" + scriptName

		for _, path := range paths {
			// Add edge from script to referenced file
			g.graph.AddEdge(scriptNode, path, 1.0)
		}
	}
}

// AddCIRefs adds edges from CI jobs to files they reference
func (g *GenericGraphBuilder) AddCIRefs(ciRefs map[string][]string) {
	for jobName, paths := range ciRefs {
		// Create a virtual CI job node
		ciNode := "ci:" + jobName

		for _, path := range paths {
			// Add edge from CI job to referenced file
			g.graph.AddEdge(ciNode, path, 1.0)
		}
	}
}

// AddDocRefs adds edges from documentation to referenced files
func (g *GenericGraphBuilder) AddDocRefs(docRefs []string) {
	// Create a virtual documentation node
	docNode := "docs:references"

	for _, path := range docRefs {
		// Add edge from docs to referenced file
		g.graph.AddEdge(docNode, path, 0.5) // Lower weight for doc references
	}
}

// AddConfigRefs adds edges between configuration files and related source files
func (g *GenericGraphBuilder) AddConfigRefs(configs []shared.ConfigFile) {
	for _, config := range configs {
		configNode := "config:" + config.Tool

		// Add edges based on config type
		switch config.Tool {
		case "vite", "webpack", "rollup":
			// Bundler configs are related to frontend files
			g.addConfigToPatternEdges(configNode, []string{
				"src/**/*.ts", "src/**/*.tsx", "src/**/*.js", "src/**/*.jsx",
				"ui/**/*.ts", "ui/**/*.tsx", "ui/**/*.js", "ui/**/*.jsx",
			})
		case "eslint", "prettier":
			// Linter configs are related to JS/TS files
			g.addConfigToPatternEdges(configNode, []string{
				"**/*.ts", "**/*.tsx", "**/*.js", "**/*.jsx",
			})
		case "typescript":
			// TypeScript configs are related to TS files
			g.addConfigToPatternEdges(configNode, []string{
				"**/*.ts", "**/*.tsx",
			})
		case "go":
			// Go configs are related to Go files
			g.addConfigToPatternEdges(configNode, []string{
				"**/*.go",
			})
		case "composer":
			// Composer configs are related to PHP files
			g.addConfigToPatternEdges(configNode, []string{
				"**/*.php",
			})
		case "docker", "docker-compose":
			// Docker configs might be related to main entry points
			g.addConfigToPatternEdges(configNode, []string{
				"main.go", "ui/main.go", "cmd/**/main.go",
				"Dockerfile*", "docker-compose.*",
			})
		}
	}
}

// addConfigToPatternEdges adds edges from config to files matching patterns
func (g *GenericGraphBuilder) addConfigToPatternEdges(configNode string, patterns []string) {
	// This is a simplified version - in a full implementation,
	// you'd want to actually match the patterns against the file list
	// For now, we'll just create the node
	g.graph.Vertices[configNode] = true
}

// AddEntrypointEdges adds special edges for entrypoints
func (g *GenericGraphBuilder) AddEntrypointEdges(entrypoints []shared.EntryPoint) {
	for _, entrypoint := range entrypoints {
		// Mark entrypoints as important by adding self-loops with high weight
		g.graph.AddEdge(entrypoint.Path, entrypoint.Path, 2.0)

		// Create virtual entrypoint type nodes
		typeNode := "entrypoint:" + entrypoint.Kind
		g.graph.AddEdge(typeNode, entrypoint.Path, 1.5)
	}
}

// AddRouteServiceEdges adds edges for routes and services
func (g *GenericGraphBuilder) AddRouteServiceEdges(routesServices []shared.RouteOrService) {
	for _, rs := range routesServices {
		// Create virtual nodes for routes and services
		typeNode := rs.Kind + ":" + rs.Name
		g.graph.AddEdge(typeNode, rs.Path, 1.0)

		// Routes and services are important, add self-loop
		g.graph.AddEdge(rs.Path, rs.Path, 1.2)
	}
}

// NormalizeEdgeWeights normalizes all edge weights in the graph
func (g *GenericGraphBuilder) NormalizeEdgeWeights() {
	// Find max weight
	maxWeight := 0.0
	for _, edges := range g.graph.Edges {
		for _, weight := range edges {
			if weight > maxWeight {
				maxWeight = weight
			}
		}
	}

	if maxWeight == 0 {
		return
	}

	// Normalize all weights to [0, 1]
	for from, edges := range g.graph.Edges {
		for to, weight := range edges {
			g.graph.Edges[from][to] = weight / maxWeight
		}
	}
}

// RemoveVirtualNodes removes virtual nodes that were created for analysis
func (g *GenericGraphBuilder) RemoveVirtualNodes() {
	virtualPrefixes := []string{"script:", "ci:", "docs:", "config:", "entrypoint:", "route:", "service:"}

	// Remove virtual nodes from vertices
	for vertex := range g.graph.Vertices {
		for _, prefix := range virtualPrefixes {
			if hasPrefix(vertex, prefix) {
				delete(g.graph.Vertices, vertex)
				break
			}
		}
	}

	// Remove edges from/to virtual nodes
	for from := range g.graph.Edges {
		isVirtual := false
		for _, prefix := range virtualPrefixes {
			if hasPrefix(from, prefix) {
				isVirtual = true
				break
			}
		}
		if isVirtual {
			delete(g.graph.Edges, from)
			continue
		}

		// Remove edges to virtual nodes
		for to := range g.graph.Edges[from] {
			for _, prefix := range virtualPrefixes {
				if hasPrefix(to, prefix) {
					delete(g.graph.Edges[from], to)
					break
				}
			}
		}
	}
}

// hasPrefix checks if a string has any of the given prefixes
func hasPrefix(s string, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
