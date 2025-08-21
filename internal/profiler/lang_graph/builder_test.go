package lang_graph

import (
	"fmt"
	"testing"

	"github.com/loom/loom/internal/profiler/shared"
)

func TestNewBuilder(t *testing.T) {
	builder := NewBuilder("/test/root")
	if builder == nil {
		t.Fatal("NewBuilder returned nil")
	}
	if builder.root != "/test/root" {
		t.Errorf("Expected root '/test/root', got %s", builder.root)
	}
	if builder.graph == nil {
		t.Error("Expected graph to be initialized")
	}
	if builder.tsBuilder == nil {
		t.Error("Expected tsBuilder to be initialized")
	}
	if builder.goBuilder == nil {
		t.Error("Expected goBuilder to be initialized")
	}
	if builder.phpBuilder == nil {
		t.Error("Expected phpBuilder to be initialized")
	}
	if builder.genericBuilder == nil {
		t.Error("Expected genericBuilder to be initialized")
	}
}

func TestBuilder_Build(t *testing.T) {
	builder := NewBuilder("/test/root")

	data := &shared.LangGraphData{
		TSFiles: []string{
			"src/main.ts",
			"src/utils.ts",
			"src/app.tsx",
		},
		GoFiles: []string{
			"main.go",
			"utils.go",
			"internal/handler.go",
		},
		PHPFiles: []string{
			"index.php",
			"src/Controller.php",
		},
		TSConfig: map[string]interface{}{
			"compilerOptions": map[string]interface{}{
				"baseUrl": ".",
				"paths": map[string]interface{}{
					"@/*": []string{"src/*"},
				},
			},
		},
		ComposerPSR: map[string]string{
			"App\\": "src/",
		},
		ScriptRefs: map[string][]string{
			"build": {"src/main.ts"},
			"test":  {"src/utils.ts"},
		},
		CIRefs: map[string][]string{
			"workflow": {"main.go"},
		},
		DocRefs: []string{
			"src/main.ts",
			"README.md",
		},
	}

	graph, err := builder.Build(data)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if graph == nil {
		t.Fatal("Build returned nil graph")
	}

	// Check that graph has vertices
	vertices := graph.GetVertices()
	if len(vertices) == 0 {
		t.Error("Expected graph to have vertices")
	}

	// Check that edges were created
	hasEdges := false
	for _, edges := range graph.Edges {
		if len(edges) > 0 {
			hasEdges = true
			break
		}
	}
	if !hasEdges {
		t.Error("Expected graph to have edges")
	}
}

func TestBuilder_Build_EmptyData(t *testing.T) {
	builder := NewBuilder("/test/root")

	data := &shared.LangGraphData{
		TSFiles:     []string{},
		GoFiles:     []string{},
		PHPFiles:    []string{},
		TSConfig:    make(map[string]interface{}),
		ComposerPSR: make(map[string]string),
		ScriptRefs:  make(map[string][]string),
		CIRefs:      make(map[string][]string),
		DocRefs:     []string{},
	}

	graph, err := builder.Build(data)
	if err != nil {
		t.Fatalf("Build failed with empty data: %v", err)
	}

	if graph == nil {
		t.Fatal("Build returned nil graph")
	}

	// Graph should be mostly empty but valid
	vertices := graph.GetVertices()
	// May have some vertices from script/doc refs processing
	if len(vertices) > 10 {
		t.Errorf("Expected few or no vertices for empty data, got %d", len(vertices))
	}
}

func TestBuilder_Build_TypeScriptOnly(t *testing.T) {
	builder := NewBuilder("/test/root")

	data := &shared.LangGraphData{
		TSFiles: []string{
			"src/main.ts",
			"src/utils.ts",
			"src/components/App.tsx",
		},
		TSConfig: map[string]interface{}{
			"compilerOptions": map[string]interface{}{
				"baseUrl": ".",
			},
		},
		ScriptRefs: map[string][]string{
			"build": {"src/main.ts"},
		},
		DocRefs: []string{"src/main.ts"},
	}

	graph, err := builder.Build(data)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	vertices := graph.GetVertices()
	if len(vertices) == 0 {
		t.Error("Expected graph to have vertices for TypeScript files")
	}

	// Check that TypeScript files are in the graph
	foundTS := false
	for _, vertex := range vertices {
		if vertex == "src/main.ts" {
			foundTS = true
			break
		}
	}
	if !foundTS {
		t.Error("Expected src/main.ts to be in graph vertices")
	}
}

func TestBuilder_Build_GoOnly(t *testing.T) {
	builder := NewBuilder("/test/root")

	data := &shared.LangGraphData{
		GoFiles: []string{
			"main.go",
			"internal/server.go",
			"pkg/utils.go",
		},
		CIRefs: map[string][]string{
			"test": {"main.go"},
		},
	}

	graph, err := builder.Build(data)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	vertices := graph.GetVertices()
	if len(vertices) == 0 {
		t.Error("Expected graph to have vertices for Go files")
	}

	// Check that Go files are in the graph
	foundGo := false
	for _, vertex := range vertices {
		if vertex == "main.go" {
			foundGo = true
			break
		}
	}
	if !foundGo {
		t.Error("Expected main.go to be in graph vertices")
	}
}

func TestBuilder_Build_PHPOnly(t *testing.T) {
	builder := NewBuilder("/test/root")

	data := &shared.LangGraphData{
		PHPFiles: []string{
			"index.php",
			"src/Controller.php",
			"src/Model.php",
		},
		ComposerPSR: map[string]string{
			"App\\": "src/",
		},
	}

	graph, err := builder.Build(data)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	vertices := graph.GetVertices()
	if len(vertices) == 0 {
		t.Error("Expected graph to have vertices for PHP files")
	}

	// Check that PHP files are in the graph
	foundPHP := false
	for _, vertex := range vertices {
		if vertex == "index.php" {
			foundPHP = true
			break
		}
	}
	if !foundPHP {
		t.Error("Expected index.php to be in graph vertices")
	}
}

func TestBuilder_Build_MultiLanguage(t *testing.T) {
	builder := NewBuilder("/test/root")

	data := &shared.LangGraphData{
		TSFiles: []string{
			"frontend/src/main.ts",
			"frontend/src/api.ts",
		},
		GoFiles: []string{
			"backend/main.go",
			"backend/handler.go",
		},
		PHPFiles: []string{
			"api/index.php",
			"api/controller.php",
		},
		TSConfig: map[string]interface{}{
			"compilerOptions": map[string]interface{}{
				"baseUrl": ".",
			},
		},
		ComposerPSR: map[string]string{
			"Api\\": "api/src/",
		},
		ScriptRefs: map[string][]string{
			"build-frontend": {"frontend/src/main.ts"},
			"build-backend":  {"backend/main.go"},
		},
		CIRefs: map[string][]string{
			"test-all": {"backend/main.go", "frontend/src/main.ts", "api/index.php"},
		},
		DocRefs: []string{
			"backend/main.go",
			"frontend/src/main.ts",
		},
	}

	graph, err := builder.Build(data)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	vertices := graph.GetVertices()
	if len(vertices) == 0 {
		t.Error("Expected graph to have vertices for multi-language project")
	}

	// Check that files from all languages are represented
	hasTS := false
	hasGo := false
	hasPHP := false

	for _, vertex := range vertices {
		if vertex == "frontend/src/main.ts" {
			hasTS = true
		}
		if vertex == "backend/main.go" {
			hasGo = true
		}
		if vertex == "api/index.php" {
			hasPHP = true
		}
	}

	if !hasTS {
		t.Error("Expected TypeScript files to be in graph")
	}
	if !hasGo {
		t.Error("Expected Go files to be in graph")
	}
	if !hasPHP {
		t.Error("Expected PHP files to be in graph")
	}
}

func TestBuilder_GetGraph(t *testing.T) {
	builder := NewBuilder("/test/root")

	graph := builder.GetGraph()
	if graph == nil {
		t.Error("GetGraph returned nil")
	}

	// Should be the same instance as the builder's graph
	if graph != builder.graph {
		t.Error("GetGraph should return the same graph instance")
	}
}

func TestBuilder_AnalyzeLanguages(t *testing.T) {
	builder := NewBuilder("/test/root")

	tests := []struct {
		name     string
		data     *shared.LangGraphData
		expected []string
	}{
		{
			name: "TypeScript only",
			data: &shared.LangGraphData{
				TSFiles: []string{"src/main.ts"},
			},
			expected: []string{"typescript"},
		},
		{
			name: "Go only",
			data: &shared.LangGraphData{
				GoFiles: []string{"main.go"},
			},
			expected: []string{"go"},
		},
		{
			name: "PHP only",
			data: &shared.LangGraphData{
				PHPFiles: []string{"index.php"},
			},
			expected: []string{"php"},
		},
		{
			name: "Multi-language",
			data: &shared.LangGraphData{
				TSFiles:  []string{"src/main.ts"},
				GoFiles:  []string{"main.go"},
				PHPFiles: []string{"index.php"},
			},
			expected: []string{"typescript", "go", "php"},
		},
		{
			name:     "No languages",
			data:     &shared.LangGraphData{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			languages := builder.AnalyzeLanguages(tt.data)

			if len(languages) != len(tt.expected) {
				t.Errorf("Expected %d languages, got %d", len(tt.expected), len(languages))
				return
			}

			languageSet := make(map[string]bool)
			for _, lang := range languages {
				languageSet[lang] = true
			}

			for _, expected := range tt.expected {
				if !languageSet[expected] {
					t.Errorf("Expected language %s not found", expected)
				}
			}
		})
	}
}

func TestBuilder_GetGraphStatistics(t *testing.T) {
	builder := NewBuilder("/test/root")

	// Build a simple graph first
	data := &shared.LangGraphData{
		TSFiles: []string{
			"src/main.ts",
			"src/utils.ts",
			"src/app.tsx",
		},
		ScriptRefs: map[string][]string{
			"build": {"src/main.ts"},
		},
	}

	_, err := builder.Build(data)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	stats := builder.GetGraphStatistics()

	if stats.TotalVertices < 0 {
		t.Error("Expected non-negative total vertices")
	}

	if stats.TotalEdges < 0 {
		t.Error("Expected non-negative total edges")
	}

	if stats.MaxOutDegree < 0 {
		t.Error("Expected non-negative max out degree")
	}

	if stats.AvgOutDegree < 0 {
		t.Error("Expected non-negative average out degree")
	}

	// Check consistency
	if stats.TotalVertices > 0 && stats.TotalEdges > 0 {
		if stats.MaxOutDegree > stats.TotalEdges {
			t.Error("Max out degree cannot exceed total edges")
		}
	}
}

func TestBuilder_GetGraphStatistics_EmptyGraph(t *testing.T) {
	builder := NewBuilder("/test/root")

	stats := builder.GetGraphStatistics()

	if stats.TotalVertices != 0 {
		t.Errorf("Expected 0 vertices for empty graph, got %d", stats.TotalVertices)
	}

	if stats.TotalEdges != 0 {
		t.Errorf("Expected 0 edges for empty graph, got %d", stats.TotalEdges)
	}

	if stats.MaxOutDegree != 0 {
		t.Errorf("Expected 0 max out degree for empty graph, got %d", stats.MaxOutDegree)
	}

	if stats.AvgOutDegree != 0 {
		t.Errorf("Expected 0 average out degree for empty graph, got %f", stats.AvgOutDegree)
	}
}

func TestBuilder_finalizeGraph(t *testing.T) {
	builder := NewBuilder("/test/root")

	// Add some edges to test finalization
	builder.graph.AddEdge("a", "b", 1.0)
	builder.graph.AddEdge("b", "c", 1.0)
	builder.graph.AddEdge("c", "a", 1.0)
	builder.graph.AddEdge("a", "a", 0.5) // Self-loop that should be removed

	// Add many edges from one vertex to test capping
	for i := 0; i < 600; i++ {
		builder.graph.AddEdge("hub", fmt.Sprintf("target_%d", i), 1.0)
	}

	builder.finalizeGraph()

	// Check that trivial self-loop was removed
	if builder.graph.Edges["a"]["a"] != 0 {
		t.Error("Expected trivial self-loop to be removed")
	}

	// Check that edge capping worked
	if len(builder.graph.Edges["hub"]) > 500 {
		t.Errorf("Expected max 500 edges from hub, got %d", len(builder.graph.Edges["hub"]))
	}
}

func TestBuilder_removeTrivialSelfLoops(t *testing.T) {
	builder := NewBuilder("/test/root")

	// Add self-loops with different weights
	builder.graph.AddEdge("a", "a", 0.5) // Should be removed
	builder.graph.AddEdge("b", "b", 1.0) // Should be removed
	builder.graph.AddEdge("c", "c", 2.0) // Should be kept (important file marker)
	builder.graph.AddEdge("a", "b", 1.0) // Regular edge

	builder.removeTrivialSelfLoops()

	// Check results
	if builder.graph.Edges["a"]["a"] != 0 {
		t.Error("Expected trivial self-loop a->a to be removed")
	}

	if builder.graph.Edges["b"]["b"] != 0 {
		t.Error("Expected trivial self-loop b->b to be removed")
	}

	if builder.graph.Edges["c"]["c"] != 2.0 {
		t.Error("Expected important self-loop c->c to be kept")
	}

	if builder.graph.Edges["a"]["b"] != 1.0 {
		t.Error("Expected regular edge a->b to be kept")
	}
}

func TestBuilder_capEdgesPerFile(t *testing.T) {
	builder := NewBuilder("/test/root")

	// Add many edges from one vertex
	for i := 0; i < 100; i++ {
		weight := float64(100 - i) // Higher weights for lower indices
		builder.graph.AddEdge("hub", fmt.Sprintf("target_%d", i), weight)
	}

	// Cap at 50 edges
	builder.capEdgesPerFile(50)

	// Check that only 50 edges remain
	if len(builder.graph.Edges["hub"]) != 50 {
		t.Errorf("Expected 50 edges after capping, got %d", len(builder.graph.Edges["hub"]))
	}

	// Check that highest weight edges were kept
	if _, exists := builder.graph.Edges["hub"]["target_0"]; !exists {
		t.Error("Expected highest weight edge target_0 to be kept")
	}

	if _, exists := builder.graph.Edges["hub"]["target_49"]; !exists {
		t.Error("Expected target_49 to be kept (within top 50)")
	}

	if _, exists := builder.graph.Edges["hub"]["target_99"]; exists {
		t.Error("Expected lowest weight edge target_99 to be removed")
	}
}
