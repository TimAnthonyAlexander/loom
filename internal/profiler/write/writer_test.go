package write

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/loom/loom/internal/profiler/shared"
)

func TestNewWriter(t *testing.T) {
	writer := NewWriter("/test/root")
	if writer == nil {
		t.Fatal("NewWriter returned nil")
	}
	if writer.root != "/test/root" {
		t.Errorf("Expected root '/test/root', got %s", writer.root)
	}
	if writer.profileWriter == nil {
		t.Error("Expected profileWriter to be initialized")
	}
	if writer.hotlistWriter == nil {
		t.Error("Expected hotlistWriter to be initialized")
	}
	if writer.rulesWriter == nil {
		t.Error("Expected rulesWriter to be initialized")
	}
}

func TestWriter_WriteAll(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "writer_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	writer := NewWriter(tmpDir)

	profile := &shared.Profile{
		WorkspaceRoot: tmpDir,
		CreatedAtUnix: 1234567890,
		Languages:     []string{"go", "typescript"},
		ImportantFiles: []shared.ImportantFile{
			{Path: "src/main.go", Score: 0.9},
			{Path: "src/utils.ts", Score: 0.8},
		},
		Heuristics: shared.HeuristicWeights{
			GraphCentrality: 0.5,
			GitRecency:      0.3,
		},
		Version: "2",
	}

	err = writer.WriteAll(profile)
	if err != nil {
		t.Fatalf("WriteAll failed: %v", err)
	}

	// Check that .loom directory was created
	loomDir := filepath.Join(tmpDir, ".loom")
	if _, err := os.Stat(loomDir); os.IsNotExist(err) {
		t.Error("Expected .loom directory to be created")
	}

	// Check that profile file was written
	profilePath := filepath.Join(loomDir, "project_profile.json")
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		t.Error("Expected project_profile.json to be created")
	}

	// Verify profile content
	data, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatal(err)
	}

	var writtenProfile shared.Profile
	err = json.Unmarshal(data, &writtenProfile)
	if err != nil {
		t.Fatal(err)
	}

	if writtenProfile.WorkspaceRoot != profile.WorkspaceRoot {
		t.Errorf("Expected workspace root %s, got %s",
			profile.WorkspaceRoot, writtenProfile.WorkspaceRoot)
	}

	if len(writtenProfile.ImportantFiles) != len(profile.ImportantFiles) {
		t.Errorf("Expected %d important files, got %d",
			len(profile.ImportantFiles), len(writtenProfile.ImportantFiles))
	}

	// Check that hotlist file was written
	hotlistPath := filepath.Join(loomDir, "hotlist.txt")
	if _, err := os.Stat(hotlistPath); os.IsNotExist(err) {
		t.Error("Expected hotlist.txt to be created")
	}

	// Check that rules file was written
	rulesPath := filepath.Join(loomDir, "rules.md")
	if _, err := os.Stat(rulesPath); os.IsNotExist(err) {
		t.Error("Expected rules.md to be created")
	}
}

func TestWriter_CheckShouldRun(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "writer_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	writer := NewWriter(tmpDir)

	// Should run when no profile exists
	if !writer.CheckShouldRun() {
		t.Error("Expected CheckShouldRun to return true when no profile exists")
	}

	// Create .loom directory and profile
	loomDir := filepath.Join(tmpDir, ".loom")
	err = os.MkdirAll(loomDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	profile := &shared.Profile{
		WorkspaceRoot: tmpDir,
		CreatedAtUnix: 1234567890,
		Languages:     []string{"go"},
		Version:       "2",
	}

	profilePath := filepath.Join(loomDir, "project_profile.json")
	profileData, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(profilePath, profileData, 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Now should not run (profile exists and is not stale)
	if writer.CheckShouldRun() {
		t.Error("Expected CheckShouldRun to return false when fresh profile exists")
	}
}

func TestWriter_CheckShouldRun_VersionIncompatible(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "writer_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	writer := NewWriter(tmpDir)

	// Create .loom directory and old version profile
	loomDir := filepath.Join(tmpDir, ".loom")
	err = os.MkdirAll(loomDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	oldProfile := map[string]interface{}{
		"workspace_root": tmpDir,
		"created_at":     1234567890,
		"version":        "1", // Old version
	}

	profilePath := filepath.Join(loomDir, "project_profile.json")
	profileData, err := json.Marshal(oldProfile)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(profilePath, profileData, 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Should run due to version incompatibility
	if !writer.CheckShouldRun() {
		t.Error("Expected CheckShouldRun to return true for version incompatible profile")
	}
}

func TestWriter_GetExistingProfile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "writer_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	writer := NewWriter(tmpDir)

	// Should return error when no profile exists
	_, err = writer.GetExistingProfile()
	if err == nil {
		t.Error("Expected error when no profile exists")
	}

	// Create profile
	loomDir := filepath.Join(tmpDir, ".loom")
	err = os.MkdirAll(loomDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	expectedProfile := &shared.Profile{
		WorkspaceRoot: tmpDir,
		CreatedAtUnix: 1234567890,
		Languages:     []string{"go", "typescript"},
		ImportantFiles: []shared.ImportantFile{
			{Path: "src/main.go", Score: 0.9},
		},
		Version: "2",
	}

	profilePath := filepath.Join(loomDir, "project_profile.json")
	profileData, err := json.Marshal(expectedProfile)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(profilePath, profileData, 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Should return the profile
	profile, err := writer.GetExistingProfile()
	if err != nil {
		t.Fatalf("GetExistingProfile failed: %v", err)
	}

	if profile.WorkspaceRoot != expectedProfile.WorkspaceRoot {
		t.Errorf("Expected workspace root %s, got %s",
			expectedProfile.WorkspaceRoot, profile.WorkspaceRoot)
	}

	if len(profile.ImportantFiles) != len(expectedProfile.ImportantFiles) {
		t.Errorf("Expected %d important files, got %d",
			len(expectedProfile.ImportantFiles), len(profile.ImportantFiles))
	}

	if profile.ImportantFiles[0].Path != expectedProfile.ImportantFiles[0].Path {
		t.Errorf("Expected important file path %s, got %s",
			expectedProfile.ImportantFiles[0].Path, profile.ImportantFiles[0].Path)
	}
}

func TestWriter_GetExistingHotlist(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "writer_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	writer := NewWriter(tmpDir)

	// Should return error when no hotlist exists
	_, err = writer.GetExistingHotlist()
	if err == nil {
		t.Error("Expected error when no hotlist exists")
	}

	// Create hotlist
	loomDir := filepath.Join(tmpDir, ".loom")
	err = os.MkdirAll(loomDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	expectedHotlist := []string{
		"src/main.go",
		"src/utils.ts",
		"package.json",
	}

	hotlistPath := filepath.Join(loomDir, "hotlist.txt")
	hotlistContent := ""
	for _, file := range expectedHotlist {
		hotlistContent += file + "\n"
	}

	err = os.WriteFile(hotlistPath, []byte(hotlistContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Should return the hotlist
	hotlist, err := writer.GetExistingHotlist()
	if err != nil {
		t.Fatalf("GetExistingHotlist failed: %v", err)
	}

	if len(hotlist) != len(expectedHotlist) {
		t.Errorf("Expected %d hotlist entries, got %d", len(expectedHotlist), len(hotlist))
	}

	for i, expected := range expectedHotlist {
		if i >= len(hotlist) || hotlist[i] != expected {
			t.Errorf("Expected hotlist entry %s at index %d, got %s",
				expected, i, hotlist[i])
		}
	}
}

func TestWriter_GetExistingRules(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "writer_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	writer := NewWriter(tmpDir)

	// Should return error when no rules exist
	_, err = writer.GetExistingRules()
	if err == nil {
		t.Error("Expected error when no rules exist")
	}

	// Create rules
	loomDir := filepath.Join(tmpDir, ".loom")
	err = os.MkdirAll(loomDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	expectedRules := `# Project Rules

## Code Style
- Use consistent formatting
- Add comments for complex logic

## Architecture
- Keep components small and focused
- Use dependency injection
`

	rulesPath := filepath.Join(loomDir, "rules.md")
	err = os.WriteFile(rulesPath, []byte(expectedRules), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Should return the rules
	rules, err := writer.GetExistingRules()
	if err != nil {
		t.Fatalf("GetExistingRules failed: %v", err)
	}

	if rules != expectedRules {
		t.Errorf("Expected rules content to match, got:\n%s\nExpected:\n%s",
			rules, expectedRules)
	}
}

func TestWriter_WriteAll_ErrorHandling(t *testing.T) {
	// Test with read-only directory to trigger write errors
	tmpDir, err := os.MkdirTemp("", "writer_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create .loom directory and make it read-only
	loomDir := filepath.Join(tmpDir, ".loom")
	err = os.MkdirAll(loomDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.Chmod(loomDir, 0444) // Read-only
	if err != nil {
		t.Fatal(err)
	}

	// Restore permissions for cleanup
	defer func() {
		_ = os.Chmod(loomDir, 0755)
	}()

	writer := NewWriter(tmpDir)

	profile := &shared.Profile{
		WorkspaceRoot: tmpDir,
		Languages:     []string{"go"},
		ImportantFiles: []shared.ImportantFile{
			{Path: "src/main.go", Score: 0.9},
		},
		Version: "2",
	}

	err = writer.WriteAll(profile)
	if err == nil {
		t.Error("Expected WriteAll to fail with read-only directory")
	}
}

func TestWriter_Integration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "writer_integration_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	writer := NewWriter(tmpDir)

	// Create a comprehensive profile
	profile := &shared.Profile{
		WorkspaceRoot: tmpDir,
		CreatedAtUnix: 1234567890,
		Languages:     []string{"go", "typescript", "php"},
		Entrypoints: []shared.EntryPoint{
			{Path: "main.go", Kind: "backend", Hints: []string{"go-main"}},
			{Path: "src/index.ts", Kind: "frontend", Hints: []string{"vite"}},
		},
		Scripts: []shared.Script{
			{Name: "build", Source: "package.json", Cmd: "tsc"},
			{Name: "test", Source: "go", Cmd: "go test ./..."},
		},
		ImportantFiles: []shared.ImportantFile{
			{
				Path:    "src/main.go",
				Score:   0.95,
				Reasons: []string{"graph_central", "recent_changes"},
				Components: map[string]float64{
					"centrality":  0.8,
					"git_recency": 0.15,
				},
				Confidence: 0.9,
			},
			{
				Path:    "src/utils.ts",
				Score:   0.82,
				Reasons: []string{"frequently_changed"},
				Components: map[string]float64{
					"centrality":    0.6,
					"git_frequency": 0.22,
				},
				Confidence: 0.85,
			},
		},
		Heuristics: shared.HeuristicWeights{
			GraphCentrality: 0.45,
			GitRecency:      0.25,
			GitFrequency:    0.20,
			ScriptRefs:      0.05,
			DocMentions:     0.05,
			CapMax:          0.95,
		},
		GitStats: shared.GitStatsMode{
			Mode:       "gitdir",
			WindowDays: 730,
		},
		Metrics: shared.ProfilerMetrics{
			Files:         150,
			Edges:         320,
			PageRankIters: 12,
			DurationMs:    2500,
			RankChurn:     0.15,
		},
		Version: "2",
	}

	// Write the profile
	err = writer.WriteAll(profile)
	if err != nil {
		t.Fatalf("WriteAll failed: %v", err)
	}

	// Debug: check what was actually written
	profilePath := filepath.Join(tmpDir, ".loom", "project_profile.json")
	writtenData, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("Failed to read written profile: %v", err)
	}
	t.Logf("Written profile version check: %s", string(writtenData)[:200])

	// Verify we can read everything back
	readProfile, err := writer.GetExistingProfile()
	if err != nil {
		t.Fatalf("GetExistingProfile failed: %v", err)
	}

	// Verify profile structure
	if readProfile.WorkspaceRoot != profile.WorkspaceRoot {
		t.Error("Workspace root mismatch")
	}

	if len(readProfile.Languages) != len(profile.Languages) {
		t.Error("Languages count mismatch")
	}

	if len(readProfile.ImportantFiles) != len(profile.ImportantFiles) {
		t.Error("Important files count mismatch")
	}

	if readProfile.Metrics.Files != profile.Metrics.Files {
		t.Error("Metrics files count mismatch")
	}

	// Verify hotlist
	hotlist, err := writer.GetExistingHotlist()
	if err != nil {
		t.Fatalf("GetExistingHotlist failed: %v", err)
	}

	if len(hotlist) != len(profile.ImportantFiles) {
		t.Error("Hotlist length mismatch")
	}

	// Verify rules
	rules, err := writer.GetExistingRules()
	if err != nil {
		t.Fatalf("GetExistingRules failed: %v", err)
	}

	if len(rules) == 0 {
		t.Error("Expected rules content to be non-empty")
	}

	// Test CheckShouldRun with existing profile
	if writer.CheckShouldRun() {
		t.Error("Expected CheckShouldRun to return false with fresh profile")
	}
}
