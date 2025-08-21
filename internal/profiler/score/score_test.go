package score

import (
	"math"
	"testing"

	"github.com/loom/loom/internal/profiler/gitstats"
	"github.com/loom/loom/internal/profiler/shared"
)

func TestNewScorer(t *testing.T) {
	weights := shared.HeuristicWeights{
		GraphCentrality: 0.5,
		GitRecency:      0.3,
		GitFrequency:    0.2,
	}

	scorer := NewScorer(weights)
	if scorer == nil {
		t.Fatal("NewScorer returned nil")
	}
	if scorer.weights.GraphCentrality != 0.5 {
		t.Errorf("Expected GraphCentrality 0.5, got %f", scorer.weights.GraphCentrality)
	}
}

func TestScorer_Rank(t *testing.T) {
	weights := shared.HeuristicWeights{
		GraphCentrality: 0.4,
		GitRecency:      0.3,
		GitFrequency:    0.2,
		ScriptRefs:      0.05,
		DocMentions:     0.05,
		CapMax:          0.95,
	}

	scorer := NewScorer(weights)

	files := []*shared.FileInfo{
		{Path: "src/main.go", Size: 1024, IsConfig: false},
		{Path: "src/utils.go", Size: 512, IsConfig: false},
		{Path: "package.json", Size: 200, IsConfig: true},
		{Path: "README.md", Size: 1000, IsDoc: true},
	}

	centrality := map[string]float64{
		"src/main.go":  0.8,
		"src/utils.go": 0.6,
		"package.json": 0.4,
		"README.md":    0.2,
	}

	gitStats := &gitstats.GitStats{
		Recency: map[string]float64{
			"src/main.go":  0.9,
			"src/utils.go": 0.7,
			"package.json": 0.3,
			"README.md":    0.1,
		},
		Frequency: map[string]float64{
			"src/main.go":  0.8,
			"src/utils.go": 0.5,
			"package.json": 0.2,
			"README.md":    0.1,
		},
		Mode: "gitdir",
	}

	signals := &shared.SignalData{
		ScriptRefs: map[string][]string{
			"build": {"src/main.go"},
		},
		CIRefs:  map[string][]string{},
		DocRefs: []string{"src/main.go"},
	}

	importantFiles := scorer.Rank(files, centrality, gitStats, signals)

	if len(importantFiles) == 0 {
		t.Fatal("Expected some important files")
	}

	// Check that files are sorted by score
	for i := 1; i < len(importantFiles); i++ {
		if importantFiles[i-1].Score < importantFiles[i].Score {
			t.Errorf("Files not sorted by score: %f < %f",
				importantFiles[i-1].Score, importantFiles[i].Score)
		}
	}

	// Check that main.go has highest score (highest centrality + recency)
	if importantFiles[0].Path != "src/main.go" {
		t.Errorf("Expected src/main.go to have highest score, got %s", importantFiles[0].Path)
	}

	// Check that scores are reasonable
	for _, file := range importantFiles {
		if file.Score < 0 || file.Score > 1 {
			t.Errorf("Score out of range [0,1]: %f for %s", file.Score, file.Path)
		}
	}
}

func TestPageRank(t *testing.T) {
	graph := shared.NewGraph()
	graph.AddEdge("a", "b", 1.0)
	graph.AddEdge("b", "c", 1.0)
	graph.AddEdge("c", "a", 1.0)

	pageRank, iterations := PageRank(graph, 0.85, 1e-6, 100)

	if len(pageRank) != 3 {
		t.Errorf("Expected PageRank for 3 vertices, got %d", len(pageRank))
	}

	// In a symmetric cycle, all nodes should have equal PageRank (normalized to max 1.0)
	tolerance := 0.01
	for vertex, pr := range pageRank {
		expectedPR := 1.0 // All should be 1.0 since they're equal and normalized to max
		if math.Abs(pr-expectedPR) > tolerance {
			t.Errorf("Expected PageRank ~%f for %s, got %f", expectedPR, vertex, pr)
		}
	}

	if iterations <= 0 {
		t.Error("Expected positive number of iterations")
	}
}

func TestPageRank_EmptyGraph(t *testing.T) {
	graph := shared.NewGraph()
	pageRank, iterations := PageRank(graph, 0.85, 1e-6, 100)

	if len(pageRank) != 0 {
		t.Errorf("Expected empty PageRank for empty graph, got %d vertices", len(pageRank))
	}

	if iterations != 0 {
		t.Errorf("Expected 0 iterations for empty graph, got %d", iterations)
	}
}

func TestPageRank_SingleVertex(t *testing.T) {
	graph := shared.NewGraph()
	graph.AddEdge("a", "a", 1.0) // Self-loop

	pageRank, iterations := PageRank(graph, 0.85, 1e-6, 100)

	if len(pageRank) != 1 {
		t.Errorf("Expected PageRank for 1 vertex, got %d", len(pageRank))
	}

	if pageRank["a"] != 1.0 {
		t.Errorf("Expected PageRank 1.0 for single vertex, got %f", pageRank["a"])
	}

	if iterations <= 0 {
		t.Error("Expected positive number of iterations")
	}
}

func TestPageRank_Hub(t *testing.T) {
	graph := shared.NewGraph()
	// Create hub structure: hub -> {a, b, c}, all point back to hub
	graph.AddEdge("hub", "a", 1.0)
	graph.AddEdge("hub", "b", 1.0)
	graph.AddEdge("hub", "c", 1.0)
	graph.AddEdge("a", "hub", 1.0)
	graph.AddEdge("b", "hub", 1.0)
	graph.AddEdge("c", "hub", 1.0)

	pageRank, _ := PageRank(graph, 0.85, 1e-6, 100)

	// Hub should have higher PageRank than spokes
	hubPR := pageRank["hub"]
	spokePR := pageRank["a"]

	if hubPR <= spokePR {
		t.Errorf("Expected hub PageRank (%f) > spoke PageRank (%f)", hubPR, spokePR)
	}
}

func TestScorer_isTestFile(t *testing.T) {
	scorer := NewScorer(shared.HeuristicWeights{})

	tests := []struct {
		path     string
		expected bool
	}{
		{"src/main_test.go", true},
		{"src/component.test.ts", true},
		{"src/component.spec.tsx", true},
		{"/tests/unit.js", true},
		{"__tests__/component.test.jsx", true},
		{"/spec/helper.rb", true},
		{"src/main.go", false},
		{"src/component.tsx", false},
		{"package.json", false},
	}

	for _, tt := range tests {
		result := scorer.isTestFile(tt.path)
		if result != tt.expected {
			t.Errorf("isTestFile(%q) = %v, want %v", tt.path, result, tt.expected)
		}
	}
}

func TestScorer_isLikelyEntrypoint(t *testing.T) {
	scorer := NewScorer(shared.HeuristicWeights{})

	tests := []struct {
		path     string
		expected bool
	}{
		{"main.go", true},
		{"src/main.ts", true},
		{"app/index.tsx", true},
		{"server.js", true},
		{"cmd/cli/main.go", true},
		{"src/utils.go", false},
		{"components/Button.tsx", false},
		{"package.json", false},
	}

	for _, tt := range tests {
		result := scorer.isLikelyEntrypoint(tt.path)
		if result != tt.expected {
			t.Errorf("isLikelyEntrypoint(%q) = %v, want %v", tt.path, result, tt.expected)
		}
	}
}

func TestScorer_calculateScriptRefScores(t *testing.T) {
	scorer := NewScorer(shared.HeuristicWeights{})

	scriptRefs := map[string][]string{
		"build": {"src/main.go", "src/utils.go"},
		"test":  {"src/main.go"},
	}

	ciRefs := map[string][]string{
		"ci": {"src/main.go"},
	}

	scores := scorer.calculateScriptRefScores(scriptRefs, ciRefs)

	// src/main.go should have highest score (3 references)
	if scores["src/main.go"] != 1.0 {
		t.Errorf("Expected max score 1.0 for src/main.go, got %f", scores["src/main.go"])
	}

	// src/utils.go should have lower score (1 reference)
	expectedUtilsScore := 1.0 / 3.0
	if math.Abs(scores["src/utils.go"]-expectedUtilsScore) > 0.01 {
		t.Errorf("Expected score %f for src/utils.go, got %f", expectedUtilsScore, scores["src/utils.go"])
	}
}

func TestScorer_calculateDocMentionScores(t *testing.T) {
	scorer := NewScorer(shared.HeuristicWeights{})

	docRefs := []string{
		"src/main.go",
		"src/main.go", // mentioned twice
		"src/utils.go",
	}

	scores := scorer.calculateDocMentionScores(docRefs)

	// src/main.go should have max score (2 mentions)
	if scores["src/main.go"] != 1.0 {
		t.Errorf("Expected max score 1.0 for src/main.go, got %f", scores["src/main.go"])
	}

	// src/utils.go should have half score (1 mention)
	if scores["src/utils.go"] != 0.5 {
		t.Errorf("Expected score 0.5 for src/utils.go, got %f", scores["src/utils.go"])
	}
}

func TestScorer_rebalanceWeightsForGitMode(t *testing.T) {
	scorer := NewScorer(shared.HeuristicWeights{
		GraphCentrality: 0.4,
		GitRecency:      0.3,
		GitFrequency:    0.2,
		ScriptRefs:      0.05,
		DocMentions:     0.05,
	})

	// Test with git available
	weights := scorer.rebalanceWeightsForGitMode("gitdir")
	if weights.GitRecency != 0.3 {
		t.Errorf("Expected git weights unchanged when git available, got %f", weights.GitRecency)
	}

	// Test with no git
	weights = scorer.rebalanceWeightsForGitMode("none")
	if weights.GitRecency != 0 {
		t.Errorf("Expected git recency weight 0 when no git, got %f", weights.GitRecency)
	}
	if weights.GitFrequency != 0 {
		t.Errorf("Expected git frequency weight 0 when no git, got %f", weights.GitFrequency)
	}

	// Other weights should be increased
	if weights.GraphCentrality <= 0.4 {
		t.Errorf("Expected graph centrality weight increased, got %f", weights.GraphCentrality)
	}
}

func TestCapComponents(t *testing.T) {
	components := map[string]float64{
		"centrality": 0.8,
		"recency":    0.1,
		"frequency":  0.1,
	}

	// Test capping at 65%
	capped := capComponents(components, 0.65)

	totalScore := capped["centrality"] + capped["recency"] + capped["frequency"]
	maxAllowed := totalScore * 0.65

	if capped["centrality"] > maxAllowed+0.001 { // Small tolerance for floating point
		t.Errorf("Expected centrality capped at %f, got %f", maxAllowed, capped["centrality"])
	}

	// Total should remain approximately the same
	originalTotal := 0.8 + 0.1 + 0.1
	if math.Abs(totalScore-originalTotal) > 0.01 {
		t.Errorf("Expected total score preserved ~%f, got %f", originalTotal, totalScore)
	}
}

func TestCalculateConfidence(t *testing.T) {
	gitStats := &gitstats.GitStats{Mode: "gitdir"}
	signals := &shared.SignalData{
		DocRefs: []string{"README.md"},
		Scripts: []shared.Script{{Name: "build"}},
		TSFiles: []string{"src/main.ts"},
	}

	confidence := calculateConfidence(gitStats, signals)
	if confidence != 1.0 {
		t.Errorf("Expected full confidence with all signals, got %f", confidence)
	}

	// Test with missing git
	gitStats.Mode = "none"
	confidence = calculateConfidence(gitStats, signals)
	if confidence >= 1.0 {
		t.Errorf("Expected reduced confidence without git, got %f", confidence)
	}

	// Test with missing docs
	signals.DocRefs = []string{}
	confidence = calculateConfidence(gitStats, signals)
	if confidence >= 0.8 {
		t.Errorf("Expected further reduced confidence without docs, got %f", confidence)
	}
}

func TestApplyTieBreakers(t *testing.T) {
	tests := []struct {
		path     string
		expected float64
	}{
		{"main.go", 0.01},
		{"app.tsx", 0.01},
		{"index.ts", 0.01},
		{"package.json", 0.005},
		{"go.mod", 0.005},
		{"routes/api.php", 0.01},
		{"src/utils.go", 0.0},
		{"random.txt", 0.0},
	}

	for _, tt := range tests {
		result := applyTieBreakers(tt.path)
		if result != tt.expected {
			t.Errorf("applyTieBreakers(%q) = %f, want %f", tt.path, result, tt.expected)
		}
	}
}

func TestDetectRankChurn(t *testing.T) {
	current := []shared.ImportantFile{
		{Path: "a"}, {Path: "b"}, {Path: "c"}, {Path: "d"}, {Path: "e"},
	}

	previous := []shared.ImportantFile{
		{Path: "a"}, {Path: "b"}, {Path: "c"}, {Path: "d"}, {Path: "e"},
	}

	// Identical rankings should have 0 churn
	churn := DetectRankChurn(current, previous)
	if churn != 0.0 {
		t.Errorf("Expected 0 churn for identical rankings, got %f", churn)
	}

	// Completely different rankings should have high churn
	previous = []shared.ImportantFile{
		{Path: "f"}, {Path: "g"}, {Path: "h"}, {Path: "i"}, {Path: "j"},
	}

	churn = DetectRankChurn(current, previous)
	if churn != 1.0 {
		t.Errorf("Expected 1.0 churn for completely different rankings, got %f", churn)
	}

	// Partial overlap should have moderate churn
	previous = []shared.ImportantFile{
		{Path: "a"}, {Path: "b"}, {Path: "f"}, {Path: "g"}, {Path: "h"},
	}

	churn = DetectRankChurn(current, previous)
	if churn <= 0.0 || churn >= 1.0 {
		t.Errorf("Expected moderate churn for partial overlap, got %f", churn)
	}
}

func TestScorer_calculateFileScoreWithProvenance(t *testing.T) {
	weights := shared.HeuristicWeights{
		GraphCentrality: 0.5,
		GitRecency:      0.3,
		GitFrequency:    0.2,
		CapMax:          0.95,
	}

	scorer := NewScorer(weights)

	file := &shared.FileInfo{
		Path: "src/main.go",
		Size: 1024,
	}

	centrality := map[string]float64{"src/main.go": 0.8}
	gitStats := &gitstats.GitStats{
		Recency:   map[string]float64{"src/main.go": 0.9},
		Frequency: map[string]float64{"src/main.go": 0.7},
	}
	scriptRefs := map[string]float64{}
	docMentions := map[string]float64{}

	score, components, penalties := scorer.calculateFileScoreWithProvenance(
		file, centrality, gitStats, scriptRefs, docMentions, weights)

	// Check that score is reasonable
	if score <= 0 || score > 1 {
		t.Errorf("Score out of range: %f", score)
	}

	// Check that components are properly weighted
	expectedCentrality := weights.GraphCentrality * 0.8
	if math.Abs(components["centrality"]-expectedCentrality) > 0.01 {
		t.Errorf("Expected centrality component %f, got %f",
			expectedCentrality, components["centrality"])
	}

	// Check that penalties are tracked
	if penalties == nil {
		t.Error("Expected penalties map to be initialized")
	}
}

func TestScorer_PenaltyApplication(t *testing.T) {
	weights := shared.HeuristicWeights{
		GraphCentrality: 1.0, // Only centrality for simplicity
		CapMax:          0.95,
	}

	scorer := NewScorer(weights)

	// Test generated file penalty
	generatedFile := &shared.FileInfo{
		Path:        "generated/types.pb.go",
		Size:        1024,
		IsGenerated: true,
	}

	centrality := map[string]float64{"generated/types.pb.go": 0.8}
	gitStats := &gitstats.GitStats{
		Recency:   map[string]float64{},
		Frequency: map[string]float64{},
	}

	score, _, penalties := scorer.calculateFileScoreWithProvenance(
		generatedFile, centrality, gitStats, map[string]float64{}, map[string]float64{}, weights)

	// Generated files should have reduced score
	expectedScore := 0.104 // Based on actual calculation
	if math.Abs(score-expectedScore) > 0.01 {
		t.Errorf("Expected generated file score %f, got %f", expectedScore, score)
	}

	// Penalty should be recorded
	if penalties["vendored"] <= 0 {
		t.Errorf("Expected vendored penalty to be recorded, got %f", penalties["vendored"])
	}

	// Test large file penalty
	largeFile := &shared.FileInfo{
		Path: "large.txt",
		Size: 1024 * 1024, // 1MB
	}

	score, _, _ = scorer.calculateFileScoreWithProvenance(
		largeFile, map[string]float64{"large.txt": 0.8}, gitStats,
		map[string]float64{}, map[string]float64{}, weights)

	// Large files should have reduced score
	expectedScore = 0.052 // Based on actual calculation
	if math.Abs(score-expectedScore) > 0.01 {
		t.Errorf("Expected large file score %f, got %f", expectedScore, score)
	}

	// Test test file penalty
	testFile := &shared.FileInfo{
		Path: "src/main_test.go",
		Size: 1024,
	}

	score, _, _ = scorer.calculateFileScoreWithProvenance(
		testFile, map[string]float64{"src/main_test.go": 0.8}, gitStats,
		map[string]float64{}, map[string]float64{}, weights)

	// Test files should have reduced score
	expectedScore = 0.364 // Based on actual calculation
	if math.Abs(score-expectedScore) > 0.01 {
		t.Errorf("Expected test file score %f, got %f", expectedScore, score)
	}
}
