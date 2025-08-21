package profiler

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/loom/loom/internal/profiler/gitstats"
	"github.com/loom/loom/internal/profiler/lang_graph"
	"github.com/loom/loom/internal/profiler/score"
	"github.com/loom/loom/internal/profiler/shared"
	"github.com/loom/loom/internal/profiler/signals"
	"github.com/loom/loom/internal/profiler/write"
)

// Runner orchestrates the entire profiling pipeline
type Runner struct {
	root string
}

// NewRunner creates a new profiler runner
func NewRunner(root string) *Runner {
	return &Runner{root: root}
}

// Run executes the complete profiling pipeline
func (r *Runner) Run(ctx context.Context) (*Profile, error) {
	startTime := time.Now()

	// 1. File system scan
	scanner := NewFSScan(r.root)
	files, _, _ := scanner.Scan(ctx)

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// 2. Signal extraction
	signalCollector := signals.NewCollector(r.root)
	signalData := signalCollector.Collect(files)

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// 3. Detect monorepo structure
	monorepoInfo := r.detectMonorepo(files)

	// 4. Language graph building
	graphBuilder := lang_graph.NewBuilder(r.root)

	// Convert signal data to lang graph data
	langData := &shared.LangGraphData{
		TSFiles:     signalData.TSFiles,
		GoFiles:     signalData.GoFiles,
		PHPFiles:    signalData.PHPFiles,
		TSConfig:    signalData.TSConfig,
		ComposerPSR: signalData.ComposerPSR,
		ScriptRefs:  signalData.ScriptRefs,
		CIRefs:      signalData.CIRefs,
		DocRefs:     signalData.DocRefs,
	}

	var graph *shared.Graph
	if monorepoInfo.IsMonorepo {
		graph = r.buildMonorepoGraph(graphBuilder, langData, monorepoInfo)
	} else {
		var err error
		graph, err = graphBuilder.Build(langData)
		if err != nil {
			_ = err // Log error but continue - graph might be partial
		}
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// 5. Git statistics extraction
	gitStatsExtractor := gitstats.NewExtractor(r.root, 730) // 2 year window
	gitStats := gitStatsExtractor.Extract()

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// 6. PageRank calculation
	centrality, pageRankIters := score.PageRank(graph, 0.85, 1e-6, 50)

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// 7. File scoring
	weights := HeuristicWeights{
		GraphCentrality: 0.45,
		GitRecency:      0.20,
		GitFrequency:    0.15,
		ScriptRefs:      0.10,
		DocMentions:     0.10,
		CapMax:          0.98,
	}

	scorer := score.NewScorer(weights)
	importantFiles := scorer.Rank(files, centrality, gitStats, signalData)

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// 8. Assemble final profile
	duration := time.Since(startTime)
	edgeCount := r.countGraphEdges(graph)
	profile := r.assembleProfile(signalData, importantFiles, weights, gitStats, len(files), edgeCount, pageRankIters, duration)

	// 8. Write outputs
	writer := write.NewWriter(r.root)
	if err := writer.WriteAll(profile); err != nil {
		return nil, err
	}

	return profile, nil
}

// ShouldRun determines if the profiler should run based on input signature changes
func (r *Runner) ShouldRun() bool {
	writer := write.NewWriter(r.root)

	// Check if profile exists
	if !writer.CheckShouldRun() {
		return false
	}

	// Get existing profile to compare signatures
	existingProfile, err := writer.GetExistingProfile()
	if err != nil {
		return true // No existing profile or can't read it
	}

	// Calculate current input signature
	currentSignature := r.calculateInputSignature()

	// Compare with existing signature
	return !r.signaturesEqual(currentSignature, existingProfile.InputSignature)
}

// GetExistingProfile returns an existing profile if available
func (r *Runner) GetExistingProfile() (*Profile, error) {
	writer := write.NewWriter(r.root)
	return writer.GetExistingProfile()
}

// countGraphEdges counts the total number of edges in the graph
func (r *Runner) countGraphEdges(graph *shared.Graph) int {
	edgeCount := 0
	for _, edges := range graph.Edges {
		edgeCount += len(edges)
	}
	return edgeCount
}

// assembleProfile creates the final Profile struct
func (r *Runner) assembleProfile(signals *SignalData, importantFiles []ImportantFile, weights HeuristicWeights, gitStats *gitstats.GitStats, fileCount, edgeCount, pageRankIters int, duration time.Duration) *Profile {
	// Detect languages
	languages := r.detectLanguages(signals)

	// Calculate input signature for drift detection
	inputSignature := r.calculateInputSignature()

	// Get git stats mode from extraction results
	gitStatsMode := shared.GitStatsMode{
		Mode:       gitStats.Mode,
		WindowDays: 730,
	}

	// Load manual boosts if they exist
	manualBoosts := r.loadManualBoosts()

	// Apply manual boosts to important files
	r.applyManualBoosts(importantFiles, manualBoosts)

	// Calculate rank churn compared to previous profile
	var rankChurn float64
	if existingProfile, err := r.GetExistingProfile(); err == nil {
		rankChurn = score.DetectRankChurn(importantFiles, existingProfile.ImportantFiles)
	}

	// Create metrics
	metrics := shared.ProfilerMetrics{
		Files:         fileCount,
		Edges:         edgeCount,
		PageRankIters: pageRankIters,
		DurationMs:    duration.Milliseconds(),
		RankChurn:     rankChurn,
	}

	return &Profile{
		WorkspaceRoot:  r.root,
		CreatedAtUnix:  time.Now().Unix(),
		Languages:      languages,
		Entrypoints:    signals.Entrypoints,
		Scripts:        signals.Scripts,
		CI:             signals.CI,
		Configs:        signals.Configs,
		Codegen:        signals.Codegen,
		RoutesServices: signals.RoutesServices,
		ImportantFiles: importantFiles,
		Heuristics:     weights,
		GitStats:       gitStatsMode,
		GitWindowDays:  730, // Keep for backward compatibility
		InputSignature: inputSignature,
		Metrics:        metrics,
		ManualBoosts:   manualBoosts,
		Version:        "2", // Increment version due to schema changes
	}
}

// detectLanguages detects the primary languages in the project
func (r *Runner) detectLanguages(signals *SignalData) []string {
	var languages []string

	// Check for TypeScript/JavaScript
	if len(signals.TSFiles) > 0 {
		// Determine if primarily TypeScript or JavaScript
		tsCount := 0
		jsCount := 0

		for _, file := range signals.TSFiles {
			if strings.HasSuffix(file, ".ts") || strings.HasSuffix(file, ".tsx") {
				tsCount++
			} else {
				jsCount++
			}
		}

		if tsCount >= jsCount {
			languages = append(languages, "typescript")
		} else {
			languages = append(languages, "javascript")
		}
	}

	// Check for Go
	if len(signals.GoFiles) > 0 {
		languages = append(languages, "go")
	}

	// Check for PHP
	if len(signals.PHPFiles) > 0 {
		languages = append(languages, "php")
	}

	// Could extend to detect other languages from file extensions

	return languages
}

// GetHotlist returns the hotlist for quick access
func (r *Runner) GetHotlist() ([]string, error) {
	writer := write.NewWriter(r.root)
	return writer.GetExistingHotlist()
}

// GetRules returns the rules for quick access
func (r *Runner) GetRules() (string, error) {
	writer := write.NewWriter(r.root)
	return writer.GetExistingRules()
}

// RunQuick performs a quick check and returns basic information
func (r *Runner) RunQuick(ctx context.Context) (*QuickProfile, error) {
	// Quick scan for basic information without full analysis
	scanner := NewFSScan(r.root)
	files, _, _ := scanner.Scan(ctx)

	// Basic signal extraction
	signalCollector := signals.NewCollector(r.root)
	signalData := signalCollector.Collect(files)

	// Detect languages
	languages := r.detectLanguages(signalData)

	return &QuickProfile{
		Languages:   languages,
		Entrypoints: signalData.Entrypoints,
		Scripts:     signalData.Scripts,
		FileCount:   len(files),
	}, nil
}

// QuickProfile contains basic profile information for quick checks
type QuickProfile struct {
	Languages   []string     `json:"languages"`
	Entrypoints []EntryPoint `json:"entrypoints"`
	Scripts     []Script     `json:"scripts"`
	FileCount   int          `json:"file_count"`
}

// calculateInputSignature computes hashes of key files for drift detection
func (r *Runner) calculateInputSignature() shared.InputSignature {
	signature := shared.InputSignature{
		ManifestHashes: make(map[string]string),
	}

	// Key manifest files to track
	manifestFiles := []string{
		"package.json",
		"composer.json",
		"go.mod",
		"Cargo.toml",
		"pyproject.toml",
		"wails.json",
		"Makefile",
		"Dockerfile",
		"docker-compose.yml",
		".gitignore",
	}

	var maxMtime int64

	// Hash manifest files
	for _, file := range manifestFiles {
		filePath := filepath.Join(r.root, file)
		if info, err := os.Stat(filePath); err == nil {
			if hash, err := r.hashFile(filePath); err == nil {
				signature.ManifestHashes[file] = hash
			}
			if info.ModTime().Unix() > maxMtime {
				maxMtime = info.ModTime().Unix()
			}
		}
	}

	// Hash tsconfig.json if it exists
	tsconfigPath := filepath.Join(r.root, "tsconfig.json")
	if hash, err := r.hashFile(tsconfigPath); err == nil {
		signature.TSConfigHash = hash
	}

	// Hash README.md if it exists
	readmePath := filepath.Join(r.root, "README.md")
	if hash, err := r.hashFile(readmePath); err == nil {
		signature.ReadmeHash = hash
	}

	signature.MtimeMax = maxMtime
	return signature
}

// hashFile computes SHA256 hash of a file
func (r *Runner) hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

// signaturesEqual compares two input signatures
func (r *Runner) signaturesEqual(a, b shared.InputSignature) bool {
	// Compare manifest hashes
	if len(a.ManifestHashes) != len(b.ManifestHashes) {
		return false
	}

	for file, hash := range a.ManifestHashes {
		if b.ManifestHashes[file] != hash {
			return false
		}
	}

	// Compare other hashes
	return a.TSConfigHash == b.TSConfigHash &&
		a.ReadmeHash == b.ReadmeHash &&
		a.MtimeMax == b.MtimeMax
}

// loadManualBoosts loads manual boost configuration from file
func (r *Runner) loadManualBoosts() map[string]float64 {
	manualBoosts := make(map[string]float64)

	// Try to load from .loom/manual_boosts.json
	boostsPath := filepath.Join(r.root, ".loom", "manual_boosts.json")
	data, err := os.ReadFile(boostsPath)
	if err != nil {
		return manualBoosts // Return empty map if file doesn't exist
	}

	// Parse JSON
	var boosts map[string]float64
	if err := json.Unmarshal(data, &boosts); err != nil {
		return manualBoosts // Return empty map if parse fails
	}

	return boosts
}

// applyManualBoosts applies manual boosts to important files and re-sorts
func (r *Runner) applyManualBoosts(importantFiles []shared.ImportantFile, manualBoosts map[string]float64) {
	if len(manualBoosts) == 0 {
		return
	}

	// Apply boosts to matching files
	for i := range importantFiles {
		if boost, exists := manualBoosts[importantFiles[i].Path]; exists {
			importantFiles[i].Score += boost
		}
	}

	// Re-sort after applying boosts
	sort.Slice(importantFiles, func(i, j int) bool {
		if importantFiles[i].Score != importantFiles[j].Score {
			return importantFiles[i].Score > importantFiles[j].Score
		}
		return importantFiles[i].Path < importantFiles[j].Path
	})
}

// detectMonorepo identifies packages in a monorepo structure
func (r *Runner) detectMonorepo(files []*shared.FileInfo) shared.MonorepoInfo {
	var packages []shared.PackageInfo

	// Track package manifests by type
	npmManifests := make(map[string]string)
	goManifests := make(map[string]string)
	composerManifests := make(map[string]string)

	// Find all package manifests
	for _, file := range files {
		switch filepath.Base(file.Path) {
		case "package.json":
			dir := filepath.Dir(file.Path)
			if dir == "." {
				dir = ""
			}
			npmManifests[dir] = file.Path
		case "go.mod":
			dir := filepath.Dir(file.Path)
			if dir == "." {
				dir = ""
			}
			goManifests[dir] = file.Path
		case "composer.json":
			dir := filepath.Dir(file.Path)
			if dir == "." {
				dir = ""
			}
			composerManifests[dir] = file.Path
		}
	}

	// Create packages for each type
	r.createPackagesFromManifests(npmManifests, "npm", files, &packages)
	r.createPackagesFromManifests(goManifests, "go", files, &packages)
	r.createPackagesFromManifests(composerManifests, "composer", files, &packages)

	// Determine if this is a monorepo (more than one package)
	isMonorepo := len(packages) > 1

	return shared.MonorepoInfo{
		IsMonorepo: isMonorepo,
		Packages:   packages,
	}
}

// createPackagesFromManifests creates PackageInfo for each manifest
func (r *Runner) createPackagesFromManifests(manifests map[string]string, packageType string, files []*shared.FileInfo, packages *[]shared.PackageInfo) {
	for root, manifestPath := range manifests {
		pkg := shared.PackageInfo{
			Root:         root,
			Type:         packageType,
			ManifestPath: manifestPath,
		}

		// Assign files to this package
		for _, file := range files {
			if r.filebelongsToPackage(file.Path, root) {
				pkg.Files = append(pkg.Files, file.Path)
			}
		}

		*packages = append(*packages, pkg)
	}
}

// filebelongsToPackage determines if a file belongs to a specific package
func (r *Runner) filebelongsToPackage(filePath, packageRoot string) bool {
	if packageRoot == "" {
		// Root package - only include files not in subdirectories with their own manifests
		return !strings.Contains(filePath, "/")
	}

	// Check if file is within the package directory
	return strings.HasPrefix(filePath, packageRoot+"/") || filePath == packageRoot
}

// buildMonorepoGraph builds a graph with proper package partitioning for monorepos
func (r *Runner) buildMonorepoGraph(graphBuilder *lang_graph.Builder, langData *shared.LangGraphData, monorepoInfo shared.MonorepoInfo) *shared.Graph {
	graph := shared.NewGraph()

	// Build per-package subgraphs first
	for _, pkg := range monorepoInfo.Packages {
		// Filter language data to only include files from this package
		pkgLangData := r.filterLangDataForPackage(langData, pkg)

		// Build graph for this package
		pkgGraph, err := graphBuilder.Build(pkgLangData)
		if err != nil {
			continue // Skip packages with build errors
		}

		// Merge package graph into main graph
		r.mergeGraph(graph, pkgGraph)
	}

	// Add cross-package edges only for explicit imports
	r.addCrossPackageEdges(graph, langData, monorepoInfo)

	return graph
}

// filterLangDataForPackage filters language data to only include files from a specific package
func (r *Runner) filterLangDataForPackage(langData *shared.LangGraphData, pkg shared.PackageInfo) *shared.LangGraphData {
	pkgFiles := make(map[string]bool)
	for _, file := range pkg.Files {
		pkgFiles[file] = true
	}

	filtered := &shared.LangGraphData{
		TSConfig:    langData.TSConfig,
		ComposerPSR: langData.ComposerPSR,
		ScriptRefs:  make(map[string][]string),
		CIRefs:      make(map[string][]string),
		DocRefs:     []string{},
	}

	// Filter TypeScript files
	for _, file := range langData.TSFiles {
		if pkgFiles[file] {
			filtered.TSFiles = append(filtered.TSFiles, file)
		}
	}

	// Filter Go files
	for _, file := range langData.GoFiles {
		if pkgFiles[file] {
			filtered.GoFiles = append(filtered.GoFiles, file)
		}
	}

	// Filter PHP files
	for _, file := range langData.PHPFiles {
		if pkgFiles[file] {
			filtered.PHPFiles = append(filtered.PHPFiles, file)
		}
	}

	// Filter script and CI references
	for script, files := range langData.ScriptRefs {
		var filteredFiles []string
		for _, file := range files {
			if pkgFiles[file] {
				filteredFiles = append(filteredFiles, file)
			}
		}
		if len(filteredFiles) > 0 {
			filtered.ScriptRefs[script] = filteredFiles
		}
	}

	for ci, files := range langData.CIRefs {
		var filteredFiles []string
		for _, file := range files {
			if pkgFiles[file] {
				filteredFiles = append(filteredFiles, file)
			}
		}
		if len(filteredFiles) > 0 {
			filtered.CIRefs[ci] = filteredFiles
		}
	}

	// Filter doc references
	for _, file := range langData.DocRefs {
		if pkgFiles[file] {
			filtered.DocRefs = append(filtered.DocRefs, file)
		}
	}

	return filtered
}

// mergeGraph merges one graph into another
func (r *Runner) mergeGraph(target, source *shared.Graph) {
	for from, edges := range source.Edges {
		for to, weight := range edges {
			target.AddEdge(from, to, weight)
		}
	}
}

// addCrossPackageEdges adds edges between packages only for explicit cross-package imports
func (r *Runner) addCrossPackageEdges(graph *shared.Graph, langData *shared.LangGraphData, monorepoInfo shared.MonorepoInfo) {
	// Create package lookup map
	fileToPackage := make(map[string]*shared.PackageInfo)
	for i := range monorepoInfo.Packages {
		pkg := &monorepoInfo.Packages[i]
		for _, file := range pkg.Files {
			fileToPackage[file] = pkg
		}
	}

	// For now, add lightweight cross-package edges with reduced weight
	// This prevents over-connecting packages while still allowing some cross-package influence
	for _, pkg := range monorepoInfo.Packages {
		for _, otherPkg := range monorepoInfo.Packages {
			if pkg.Root != otherPkg.Root {
				// Add weak edge from each package manifest to others (workspace awareness)
				graph.AddEdge(pkg.ManifestPath, otherPkg.ManifestPath, 0.1)
			}
		}
	}
}
