package profiler

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/loom/loom/internal/profiler/gitstats"
	"github.com/loom/loom/internal/profiler/shared"
)

func TestNewRunner(t *testing.T) {
	runner := NewRunner("/test/path")
	if runner == nil {
		t.Fatal("NewRunner returned nil")
	}
	if runner.root != "/test/path" {
		t.Errorf("Expected root to be '/test/path', got %s", runner.root)
	}
}

func TestRunner_ShouldRun(t *testing.T) {
	// Create temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "profiler_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	runner := NewRunner(tmpDir)

	// Should run when no profile exists
	if !runner.ShouldRun() {
		t.Error("Expected ShouldRun to return true when no profile exists")
	}
}

func TestRunner_RunQuick(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "profiler_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.go")
	err = os.WriteFile(testFile, []byte("package main\n\nfunc main() {}\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	runner := NewRunner(tmpDir)
	ctx := context.Background()

	profile, err := runner.RunQuick(ctx)
	if err != nil {
		t.Fatalf("RunQuick failed: %v", err)
	}

	if profile == nil {
		t.Fatal("RunQuick returned nil profile")
	}

	if len(profile.Languages) == 0 {
		t.Error("Expected languages to be detected")
	}

	if profile.FileCount <= 0 {
		t.Error("Expected file count to be greater than 0")
	}
}

func TestRunner_calculateInputSignature(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "profiler_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create a package.json file
	packageJSON := filepath.Join(tmpDir, "package.json")
	err = os.WriteFile(packageJSON, []byte(`{"name": "test", "version": "1.0.0"}`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	runner := NewRunner(tmpDir)
	signature := runner.calculateInputSignature()

	if len(signature.ManifestHashes) == 0 {
		t.Error("Expected manifest hashes to be populated")
	}

	if _, exists := signature.ManifestHashes["package.json"]; !exists {
		t.Error("Expected package.json to be in manifest hashes")
	}
}

func TestRunner_signaturesEqual(t *testing.T) {
	runner := NewRunner("/test")

	sig1 := shared.InputSignature{
		ManifestHashes: map[string]string{
			"package.json": "hash1",
		},
		TSConfigHash: "config1",
		ReadmeHash:   "readme1",
		MtimeMax:     123456789,
	}

	sig2 := shared.InputSignature{
		ManifestHashes: map[string]string{
			"package.json": "hash1",
		},
		TSConfigHash: "config1",
		ReadmeHash:   "readme1",
		MtimeMax:     123456789,
	}

	if !runner.signaturesEqual(sig1, sig2) {
		t.Error("Expected identical signatures to be equal")
	}

	sig2.ManifestHashes["package.json"] = "hash2"
	if runner.signaturesEqual(sig1, sig2) {
		t.Error("Expected different signatures to not be equal")
	}
}

func TestRunner_detectLanguages(t *testing.T) {
	runner := NewRunner("/test")

	signals := &SignalData{
		TSFiles:  []string{"src/main.ts", "src/app.tsx"},
		GoFiles:  []string{"main.go", "cmd/server/main.go"},
		PHPFiles: []string{"index.php", "src/Controller.php"},
	}

	languages := runner.detectLanguages(signals)

	expectedLanguages := []string{"typescript", "go", "php"}
	if len(languages) != len(expectedLanguages) {
		t.Errorf("Expected %d languages, got %d", len(expectedLanguages), len(languages))
	}

	for _, expected := range expectedLanguages {
		found := false
		for _, lang := range languages {
			if lang == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected language %s not found", expected)
		}
	}
}

func TestRunner_detectLanguages_TypeScript(t *testing.T) {
	runner := NewRunner("/test")

	// Test TypeScript predominance
	signals := &SignalData{
		TSFiles: []string{"src/main.ts", "src/app.tsx", "src/utils.js"},
	}

	languages := runner.detectLanguages(signals)
	if len(languages) != 1 || languages[0] != "typescript" {
		t.Errorf("Expected TypeScript to be detected, got %v", languages)
	}

	// Test JavaScript predominance
	signals = &SignalData{
		TSFiles: []string{"src/main.js", "src/app.jsx", "src/utils.js"},
	}

	languages = runner.detectLanguages(signals)
	if len(languages) != 1 || languages[0] != "javascript" {
		t.Errorf("Expected JavaScript to be detected, got %v", languages)
	}
}

func TestRunner_loadManualBoosts(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "profiler_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create .loom directory
	loomDir := filepath.Join(tmpDir, ".loom")
	err = os.MkdirAll(loomDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Create manual boosts file
	boostsFile := filepath.Join(loomDir, "manual_boosts.json")
	boostsContent := `{"src/important.go": 0.5, "src/critical.ts": 0.8}`
	err = os.WriteFile(boostsFile, []byte(boostsContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	runner := NewRunner(tmpDir)
	boosts := runner.loadManualBoosts()

	if len(boosts) != 2 {
		t.Errorf("Expected 2 manual boosts, got %d", len(boosts))
	}

	if boosts["src/important.go"] != 0.5 {
		t.Errorf("Expected boost 0.5 for src/important.go, got %f", boosts["src/important.go"])
	}

	if boosts["src/critical.ts"] != 0.8 {
		t.Errorf("Expected boost 0.8 for src/critical.ts, got %f", boosts["src/critical.ts"])
	}
}

func TestRunner_applyManualBoosts(t *testing.T) {
	runner := NewRunner("/test")

	files := []shared.ImportantFile{
		{Path: "src/main.go", Score: 0.5},
		{Path: "src/utils.go", Score: 0.3},
		{Path: "src/test.go", Score: 0.7},
	}

	manualBoosts := map[string]float64{
		"src/utils.go": 0.2,
	}

	runner.applyManualBoosts(files, manualBoosts)

	// Check that the boost was applied
	if files[1].Score != 0.5 { // 0.3 + 0.2
		t.Errorf("Expected boosted score 0.5, got %f", files[1].Score)
	}

	// Check that files are re-sorted after boost
	if files[0].Path != "src/test.go" {
		t.Errorf("Expected highest score file to be first, got %s", files[0].Path)
	}
}

func TestRunner_assembleProfile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "profiler_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	runner := NewRunner(tmpDir)

	signals := &SignalData{
		TSFiles: []string{"src/main.ts"},
		Scripts: []Script{{Name: "build", Cmd: "tsc"}},
	}

	importantFiles := []ImportantFile{
		{Path: "src/main.ts", Score: 0.8},
	}

	weights := HeuristicWeights{
		GraphCentrality: 0.5,
		GitRecency:      0.3,
	}

	gitStats := &gitstats.GitStats{
		Mode:      "gitdir",
		Recency:   make(map[string]float64),
		Frequency: make(map[string]float64),
	}

	profile := runner.assembleProfile(signals, importantFiles, weights, gitStats, 10, 5, 3, time.Minute)

	if profile.WorkspaceRoot != tmpDir {
		t.Errorf("Expected workspace root %s, got %s", tmpDir, profile.WorkspaceRoot)
	}

	if len(profile.Languages) == 0 {
		t.Error("Expected languages to be detected")
	}

	if len(profile.ImportantFiles) != 1 {
		t.Errorf("Expected 1 important file, got %d", len(profile.ImportantFiles))
	}

	if profile.Metrics.Files != 10 {
		t.Errorf("Expected file count 10, got %d", profile.Metrics.Files)
	}

	if profile.Metrics.Edges != 5 {
		t.Errorf("Expected edge count 5, got %d", profile.Metrics.Edges)
	}
}

func TestRunner_detectMonorepo(t *testing.T) {
	runner := NewRunner("/test")

	// Single package scenario
	files := []*shared.FileInfo{
		{Path: "package.json"},
		{Path: "src/main.ts"},
	}

	monorepoInfo := runner.detectMonorepo(files)
	if monorepoInfo.IsMonorepo {
		t.Error("Expected single package not to be detected as monorepo")
	}

	// Multiple packages scenario
	files = []*shared.FileInfo{
		{Path: "package.json"},
		{Path: "frontend/package.json"},
		{Path: "backend/go.mod"},
		{Path: "src/main.ts"},
		{Path: "frontend/src/app.tsx"},
		{Path: "backend/main.go"},
	}

	monorepoInfo = runner.detectMonorepo(files)
	if !monorepoInfo.IsMonorepo {
		t.Error("Expected multiple packages to be detected as monorepo")
	}

	if len(monorepoInfo.Packages) != 3 {
		t.Errorf("Expected 3 packages, got %d", len(monorepoInfo.Packages))
	}
}

func TestRunner_countGraphEdges(t *testing.T) {
	runner := NewRunner("/test")

	graph := shared.NewGraph()
	graph.AddEdge("a", "b", 1.0)
	graph.AddEdge("a", "c", 1.0)
	graph.AddEdge("b", "c", 1.0)

	edgeCount := runner.countGraphEdges(graph)
	if edgeCount != 3 {
		t.Errorf("Expected 3 edges, got %d", edgeCount)
	}
}

func TestRunner_filebelongsToPackage(t *testing.T) {
	runner := NewRunner("/test")

	// Root package
	if !runner.filebelongsToPackage("main.go", "") {
		t.Error("Expected main.go to belong to root package")
	}

	if runner.filebelongsToPackage("frontend/app.tsx", "") {
		t.Error("Expected frontend/app.tsx not to belong to root package")
	}

	// Subdirectory package
	if !runner.filebelongsToPackage("frontend/src/app.tsx", "frontend") {
		t.Error("Expected frontend/src/app.tsx to belong to frontend package")
	}

	if runner.filebelongsToPackage("backend/main.go", "frontend") {
		t.Error("Expected backend/main.go not to belong to frontend package")
	}
}
