package shared

import (
	"testing"
)

func TestNewGraph(t *testing.T) {
	graph := NewGraph()
	if graph == nil {
		t.Fatal("NewGraph returned nil")
	}
	if graph.Edges == nil {
		t.Error("Expected Edges to be initialized")
	}
	if graph.Vertices == nil {
		t.Error("Expected Vertices to be initialized")
	}
}

func TestGraph_AddEdge(t *testing.T) {
	graph := NewGraph()

	graph.AddEdge("a", "b", 1.0)

	// Check vertices were added
	if !graph.Vertices["a"] {
		t.Error("Expected vertex 'a' to be added")
	}
	if !graph.Vertices["b"] {
		t.Error("Expected vertex 'b' to be added")
	}

	// Check edge was added
	if graph.Edges["a"] == nil {
		t.Error("Expected edges from 'a' to be initialized")
	}
	if graph.Edges["a"]["b"] != 1.0 {
		t.Errorf("Expected edge weight 1.0, got %f", graph.Edges["a"]["b"])
	}
}

func TestGraph_AddEdge_Accumulative(t *testing.T) {
	graph := NewGraph()

	// Add same edge twice
	graph.AddEdge("a", "b", 1.0)
	graph.AddEdge("a", "b", 0.5)

	// Weight should be accumulated
	if graph.Edges["a"]["b"] != 1.5 {
		t.Errorf("Expected accumulated weight 1.5, got %f", graph.Edges["a"]["b"])
	}
}

func TestGraph_AddEdge_PathNormalization(t *testing.T) {
	graph := NewGraph()

	// Add edges with different path formats
	graph.AddEdge("src\\main.go", "src\\utils.go", 1.0)
	graph.AddEdge("src/main.go", "src/utils.go", 0.5)

	// Should be accumulated due to path normalization
	normalizedFrom := "src/main.go"
	normalizedTo := "src/utils.go"

	if graph.Edges[normalizedFrom][normalizedTo] != 1.5 {
		t.Errorf("Expected accumulated weight 1.5 after normalization, got %f",
			graph.Edges[normalizedFrom][normalizedTo])
	}
}

func TestGraph_GetVertices(t *testing.T) {
	graph := NewGraph()

	graph.AddEdge("a", "b", 1.0)
	graph.AddEdge("b", "c", 1.0)
	graph.AddEdge("c", "a", 1.0)

	vertices := graph.GetVertices()

	if len(vertices) != 3 {
		t.Errorf("Expected 3 vertices, got %d", len(vertices))
	}

	// Check all vertices are present
	vertexSet := make(map[string]bool)
	for _, v := range vertices {
		vertexSet[v] = true
	}

	expected := []string{"a", "b", "c"}
	for _, v := range expected {
		if !vertexSet[v] {
			t.Errorf("Expected vertex %s to be in result", v)
		}
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"posix path", "src/main.go", "src/main.go"},
		{"windows path", "src\\main.go", "src/main.go"},
		{"mixed separators", "src\\utils/helper.go", "src/utils/helper.go"},
		{"relative with dots", "./src/../src/main.go", "src/main.go"},
		{"current directory", ".", "."},
		{"empty string", "", "."},
		{"absolute path", "/src/main.go", "/src/main.go"},
		{"multiple slashes", "src//main.go", "src/main.go"},
		{"trailing slash", "src/", "src"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizePath(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizePath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizePath_AbsolutePaths(t *testing.T) {
	// Absolute paths should retain their leading slash
	tests := []struct {
		input    string
		expected string
	}{
		{"/usr/local/bin", "/usr/local/bin"},
		{"/home/user/project", "/home/user/project"},
	}

	for _, tt := range tests {
		result := NormalizePath(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizePath(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGraph_ComplexScenario(t *testing.T) {
	graph := NewGraph()

	// Build a small dependency graph
	graph.AddEdge("main.go", "utils.go", 1.0)
	graph.AddEdge("main.go", "config.go", 0.5)
	graph.AddEdge("utils.go", "common.go", 1.0)
	graph.AddEdge("config.go", "common.go", 0.8)
	graph.AddEdge("test.go", "utils.go", 0.3)

	// Check total vertices
	vertices := graph.GetVertices()
	if len(vertices) != 5 {
		t.Errorf("Expected 5 vertices, got %d", len(vertices))
	}

	// Check specific edge weights
	if graph.Edges["main.go"]["utils.go"] != 1.0 {
		t.Error("Expected main.go -> utils.go edge weight 1.0")
	}

	if graph.Edges["config.go"]["common.go"] != 0.8 {
		t.Error("Expected config.go -> common.go edge weight 0.8")
	}

	// Check that non-existent edges don't exist
	if graph.Edges["common.go"] != nil && graph.Edges["common.go"]["main.go"] != 0 {
		t.Error("Expected no edge from common.go to main.go")
	}
}

func TestGraph_EmptyGraph(t *testing.T) {
	graph := NewGraph()

	vertices := graph.GetVertices()
	if len(vertices) != 0 {
		t.Errorf("Expected empty graph to have 0 vertices, got %d", len(vertices))
	}
}

func TestGraph_SelfLoops(t *testing.T) {
	graph := NewGraph()

	// Add self-loop
	graph.AddEdge("main.go", "main.go", 2.0)

	// Check self-loop is recorded
	if graph.Edges["main.go"]["main.go"] != 2.0 {
		t.Errorf("Expected self-loop weight 2.0, got %f", graph.Edges["main.go"]["main.go"])
	}

	// Check vertex count
	vertices := graph.GetVertices()
	if len(vertices) != 1 {
		t.Errorf("Expected 1 vertex for self-loop, got %d", len(vertices))
	}
}

func TestGraph_LargeWeights(t *testing.T) {
	graph := NewGraph()

	// Test with large weights
	largeWeight := 999999.99
	graph.AddEdge("a", "b", largeWeight)

	if graph.Edges["a"]["b"] != largeWeight {
		t.Errorf("Expected large weight %f, got %f", largeWeight, graph.Edges["a"]["b"])
	}
}

func TestGraph_ZeroWeights(t *testing.T) {
	graph := NewGraph()

	// Test with zero weight
	graph.AddEdge("a", "b", 0.0)

	if graph.Edges["a"]["b"] != 0.0 {
		t.Errorf("Expected zero weight, got %f", graph.Edges["a"]["b"])
	}

	// Vertices should still be added
	if !graph.Vertices["a"] || !graph.Vertices["b"] {
		t.Error("Expected vertices to be added even with zero weight")
	}
}

func TestGraph_NegativeWeights(t *testing.T) {
	graph := NewGraph()

	// Test with negative weight
	graph.AddEdge("a", "b", -1.0)

	if graph.Edges["a"]["b"] != -1.0 {
		t.Errorf("Expected negative weight -1.0, got %f", graph.Edges["a"]["b"])
	}
}

func TestGraph_ManyEdgesFromOneVertex(t *testing.T) {
	graph := NewGraph()

	// Add many edges from one vertex
	sources := []string{"target1", "target2", "target3", "target4", "target5"}
	for i, target := range sources {
		graph.AddEdge("hub", target, float64(i+1))
	}

	// Check all edges exist
	if len(graph.Edges["hub"]) != 5 {
		t.Errorf("Expected 5 edges from hub, got %d", len(graph.Edges["hub"]))
	}

	// Check specific weights
	for i, target := range sources {
		expectedWeight := float64(i + 1)
		if graph.Edges["hub"][target] != expectedWeight {
			t.Errorf("Expected weight %f for hub -> %s, got %f",
				expectedWeight, target, graph.Edges["hub"][target])
		}
	}
}
