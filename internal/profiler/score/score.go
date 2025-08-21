package score

import (
	"math"
	"path/filepath"
	"sort"
	"strings"

	"github.com/loom/loom/internal/profiler/gitstats"
	"github.com/loom/loom/internal/profiler/shared"
)

// Scorer handles file importance scoring
type Scorer struct {
	weights shared.HeuristicWeights
}

// NewScorer creates a new scorer with the given weights
func NewScorer(weights shared.HeuristicWeights) *Scorer {
	return &Scorer{
		weights: weights,
	}
}

// Rank calculates importance scores for all files and returns them sorted
func (s *Scorer) Rank(files []*shared.FileInfo, centrality map[string]float64, gitStats *gitstats.GitStats, signals *shared.SignalData) []shared.ImportantFile {
	var importantFiles []shared.ImportantFile

	// Rebalance weights if git stats are not available
	weights := s.rebalanceWeightsForGitMode(gitStats.Mode)

	// Calculate script/CI reference scores
	scriptRefs := s.calculateScriptRefScores(signals.ScriptRefs, signals.CIRefs)

	// Calculate doc mention scores
	docMentions := s.calculateDocMentionScores(signals.DocRefs)

	// Calculate confidence once for all files
	confidence := calculateConfidence(gitStats, signals)

	for _, file := range files {
		score, components, penalties := s.calculateFileScoreWithProvenance(file, centrality, gitStats, scriptRefs, docMentions, weights)
		if score > 0 {
			reasons := s.getScoreReasons(file, centrality, gitStats, scriptRefs, docMentions)

			importantFiles = append(importantFiles, shared.ImportantFile{
				Path:        file.Path,
				Score:       score,
				Reasons:     reasons,
				Components:  components,
				Penalties:   penalties,
				Confidence:  confidence,
				IsGenerated: file.IsGenerated,
			})
		}
	}

	// Sort by score (descending) then by path (ascending) for stability
	sort.Slice(importantFiles, func(i, j int) bool {
		if importantFiles[i].Score != importantFiles[j].Score {
			return importantFiles[i].Score > importantFiles[j].Score
		}
		return importantFiles[i].Path < importantFiles[j].Path
	})

	// Return top N files
	maxFiles := 200
	if len(importantFiles) > maxFiles {
		importantFiles = importantFiles[:maxFiles]
	}

	return importantFiles
}

// calculateFileScoreWithProvenance calculates the overall score for a file with detailed component breakdown
func (s *Scorer) calculateFileScoreWithProvenance(file *shared.FileInfo, centrality map[string]float64, gitStats *gitstats.GitStats, scriptRefs, docMentions map[string]float64, weights shared.HeuristicWeights) (float64, map[string]float64, map[string]float64) {
	// Get base component scores
	centralityScore := centrality[file.Path]
	recencyScore := gitStats.Recency[file.Path]
	frequencyScore := gitStats.Frequency[file.Path]
	scriptRefScore := scriptRefs[file.Path]
	docMentionScore := docMentions[file.Path]

	// Calculate weighted components
	components := map[string]float64{
		"centrality":     weights.GraphCentrality * centralityScore,
		"git_recency":    weights.GitRecency * recencyScore,
		"git_frequency":  weights.GitFrequency * frequencyScore,
		"script_ci_refs": weights.ScriptRefs * scriptRefScore,
		"doc_mentions":   weights.DocMentions * docMentionScore,
	}

	// Cap single-signal dominance at 65%
	components = capComponents(components, 0.65)

	// Calculate base score
	score := components["centrality"] + components["git_recency"] + components["git_frequency"] + components["script_ci_refs"] + components["doc_mentions"]

	// Track penalties applied
	penalties := map[string]float64{
		"vendored":  0,
		"generated": 0,
		"large":     0,
		"test":      0,
	}

	originalScore := score

	// Apply penalties and track them
	if file.IsGenerated || file.IsVendored {
		penalties["vendored"] = 1.0 - 0.2 // 80% penalty
		score *= 0.2
	}

	if file.Size > 512*1024 && !file.IsConfig {
		penalties["large"] = 1.0 - 0.1 // 90% penalty
		score *= 0.1
	}

	if s.isTestFile(file.Path) {
		penalties["test"] = 1.0 - 0.7 // 30% penalty
		score *= 0.7
	}

	// Apply tie-breakers for known important files
	tieBreaker := applyTieBreakers(file.Path)
	score += tieBreaker

	// Convert penalty values to actual penalty amounts
	for key, penaltyFactor := range penalties {
		if penaltyFactor > 0 {
			penalties[key] = originalScore * penaltyFactor
		}
	}

	// Cap the score
	if score > weights.CapMax {
		score = weights.CapMax
	}

	return score, components, penalties
}

// isTestFile checks if a file is a test file
func (s *Scorer) isTestFile(path string) bool {
	lowerPath := strings.ToLower(path)

	testPatterns := []string{
		"_test.go",
		".test.ts",
		".test.tsx",
		".test.js",
		".test.jsx",
		".spec.ts",
		".spec.tsx",
		".spec.js",
		".spec.jsx",
		"/test/",
		"/tests/",
		"/__tests__/",
		"/spec/",
	}

	for _, pattern := range testPatterns {
		if strings.Contains(lowerPath, pattern) {
			return true
		}
	}

	return false
}

// calculateScriptRefScores calculates scores based on script and CI references
func (s *Scorer) calculateScriptRefScores(scriptRefs, ciRefs map[string][]string) map[string]float64 {
	scores := make(map[string]float64)

	// Count total references per file
	refCounts := make(map[string]int)

	for _, paths := range scriptRefs {
		for _, path := range paths {
			refCounts[path]++
		}
	}

	for _, paths := range ciRefs {
		for _, path := range paths {
			refCounts[path]++
		}
	}

	// Find max count for normalization
	maxCount := 0
	for _, count := range refCounts {
		if count > maxCount {
			maxCount = count
		}
	}

	// Normalize scores
	if maxCount > 0 {
		for path, count := range refCounts {
			scores[path] = float64(count) / float64(maxCount)
		}
	}

	return scores
}

// calculateDocMentionScores calculates scores based on documentation mentions
func (s *Scorer) calculateDocMentionScores(docRefs []string) map[string]float64 {
	scores := make(map[string]float64)

	// Count mentions per file
	mentions := make(map[string]int)
	for _, path := range docRefs {
		mentions[path]++
	}

	// Find max mentions for normalization
	maxMentions := 0
	for _, count := range mentions {
		if count > maxMentions {
			maxMentions = count
		}
	}

	// Normalize scores
	if maxMentions > 0 {
		for path, count := range mentions {
			scores[path] = float64(count) / float64(maxMentions)
		}
	}

	return scores
}

// getScoreReasons returns the reasons why a file has its score
func (s *Scorer) getScoreReasons(file *shared.FileInfo, centrality map[string]float64, gitStats *gitstats.GitStats, scriptRefs, docMentions map[string]float64) []string {
	var reasons []string

	// Check centrality
	if centrality[file.Path] > 0.3 {
		reasons = append(reasons, "graph_central")
	}

	// Check git activity
	if gitStats.Recency[file.Path] > 0.5 {
		reasons = append(reasons, "recent_changes")
	}
	if gitStats.Frequency[file.Path] > 0.5 {
		reasons = append(reasons, "frequently_changed")
	}

	// Check references
	if scriptRefs[file.Path] > 0.3 {
		reasons = append(reasons, "script_ref")
	}
	if docMentions[file.Path] > 0.3 {
		reasons = append(reasons, "doc_mention")
	}

	// Check if it's an entrypoint (would need to be passed in)
	if s.isLikelyEntrypoint(file.Path) {
		reasons = append(reasons, "entrypoint")
	}

	// Check if it's a config file
	if file.IsConfig {
		reasons = append(reasons, "config")
	}

	return reasons
}

// isLikelyEntrypoint checks if a file looks like an entrypoint
func (s *Scorer) isLikelyEntrypoint(path string) bool {
	lowerPath := strings.ToLower(path)

	entrypointPatterns := []string{
		"main.go",
		"main.ts",
		"main.tsx",
		"main.js",
		"index.ts",
		"index.tsx",
		"index.js",
		"app.ts",
		"app.tsx",
		"app.js",
		"server.ts",
		"server.js",
		"/cmd/",
	}

	for _, pattern := range entrypointPatterns {
		if strings.Contains(lowerPath, pattern) {
			return true
		}
	}

	return false
}

// PageRank implements the PageRank algorithm for the import graph
func PageRank(graph *shared.Graph, damping float64, epsilon float64, maxIterations int) (map[string]float64, int) {
	vertices := graph.GetVertices()
	n := len(vertices)

	if n == 0 {
		return make(map[string]float64), 0
	}

	// Sort vertices for deterministic order
	sort.Strings(vertices)

	// Initialize PageRank values
	pageRank := make(map[string]float64)
	newPageRank := make(map[string]float64)

	initialValue := 1.0 / float64(n)
	for _, vertex := range vertices {
		pageRank[vertex] = initialValue
	}

	// Calculate out-degrees
	outDegree := make(map[string]float64)
	for vertex := range graph.Edges {
		for _, weight := range graph.Edges[vertex] {
			outDegree[vertex] += weight
		}
	}

	// Power iteration
	var finalIteration int
	for iteration := 0; iteration < maxIterations; iteration++ {
		finalIteration = iteration + 1

		// Reset new PageRank values
		for _, vertex := range vertices {
			newPageRank[vertex] = (1.0 - damping) / float64(n)
		}

		// Calculate contributions from incoming edges
		for from, edges := range graph.Edges {
			fromPR := pageRank[from]
			fromOutDegree := outDegree[from]

			if fromOutDegree > 0 {
				for to, weight := range edges {
					contribution := damping * fromPR * (weight / fromOutDegree)
					newPageRank[to] += contribution
				}
			} else {
				// Handle dangling nodes by distributing equally
				danglingContribution := damping * fromPR / float64(n)
				for _, vertex := range vertices {
					newPageRank[vertex] += danglingContribution
				}
			}
		}

		// Check for convergence
		converged := true
		for _, vertex := range vertices {
			if math.Abs(newPageRank[vertex]-pageRank[vertex]) > epsilon {
				converged = false
				break
			}
		}

		// Update PageRank values
		for _, vertex := range vertices {
			pageRank[vertex] = newPageRank[vertex]
		}

		if converged {
			break
		}
	}

	// Normalize to [0, 1]
	maxPR := 0.0
	for _, pr := range pageRank {
		if pr > maxPR {
			maxPR = pr
		}
	}

	if maxPR > 0 {
		for vertex := range pageRank {
			pageRank[vertex] /= maxPR
		}
	}

	return pageRank, finalIteration
}

// rebalanceWeightsForGitMode adjusts weights based on git stats availability
func (s *Scorer) rebalanceWeightsForGitMode(gitMode string) shared.HeuristicWeights {
	weights := s.weights

	// If no git stats available, redistribute git weight to other signals
	if gitMode == "none" {
		totalGitWeight := weights.GitRecency + weights.GitFrequency

		// Zero out git weights
		weights.GitRecency = 0
		weights.GitFrequency = 0

		// Redistribute to other signals proportionally
		remainingWeight := weights.GraphCentrality + weights.ScriptRefs + weights.DocMentions
		if remainingWeight > 0 {
			factor := (remainingWeight + totalGitWeight) / remainingWeight
			weights.GraphCentrality *= factor
			weights.ScriptRefs *= factor
			weights.DocMentions *= factor
		}
	}

	return weights
}

// capComponents prevents any single component from dominating the score
func capComponents(components map[string]float64, maxDominance float64) map[string]float64 {
	totalScore := 0.0
	for _, value := range components {
		totalScore += value
	}

	if totalScore == 0 {
		return components
	}

	maxAllowed := totalScore * maxDominance
	cappedComponents := make(map[string]float64)
	excessTotal := 0.0

	// First pass: cap components and track excess
	for key, value := range components {
		if value > maxAllowed {
			cappedComponents[key] = maxAllowed
			excessTotal += value - maxAllowed
		} else {
			cappedComponents[key] = value
		}
	}

	// If no capping occurred, return original
	if excessTotal == 0 {
		return components
	}

	// Second pass: redistribute excess to non-capped components proportionally
	nonCappedTotal := 0.0
	for _, value := range components {
		if value <= maxAllowed {
			nonCappedTotal += value
		}
	}

	if nonCappedTotal > 0 {
		redistribution := excessTotal / nonCappedTotal
		for key, value := range components {
			if value <= maxAllowed {
				cappedComponents[key] = value * (1 + redistribution)
			}
		}
	}

	return cappedComponents
}

// calculateConfidence computes confidence based on available signals
func calculateConfidence(gitStats *gitstats.GitStats, signals *shared.SignalData) float64 {
	confidence := 1.0

	// Reduce confidence if git stats unavailable
	if gitStats.Mode == "none" {
		confidence -= 0.2
	}

	// Reduce confidence if docs are empty
	if len(signals.DocRefs) == 0 {
		confidence -= 0.1
	}

	// Reduce confidence if scripts are empty
	if len(signals.Scripts) == 0 && len(signals.CIRefs) == 0 {
		confidence -= 0.1
	}

	// Reduce confidence if language graph is missing
	if len(signals.TSFiles) == 0 && len(signals.GoFiles) == 0 && len(signals.PHPFiles) == 0 {
		confidence -= 0.1
	}

	// Ensure confidence doesn't go negative
	if confidence < 0 {
		confidence = 0
	}

	return confidence
}

// applyTieBreakers adds small deterministic bumps for known important files
func applyTieBreakers(filePath string) float64 {
	fileName := filepath.Base(filePath)
	lowerPath := strings.ToLower(filePath)
	lowerFile := strings.ToLower(fileName)

	// Framework entrypoints get a small boost
	entrypointPatterns := map[string]bool{
		"main.go":   true,
		"app.tsx":   true,
		"app.ts":    true,
		"app.js":    true,
		"index.tsx": true,
		"index.ts":  true,
		"index.js":  true,
		"server.ts": true,
		"server.js": true,
	}

	if entrypointPatterns[lowerFile] {
		return 0.01
	}

	// Laravel/PHP routes
	if strings.Contains(lowerPath, "routes/") && strings.HasSuffix(lowerPath, ".php") {
		return 0.01
	}

	// Important config files
	configPatterns := map[string]bool{
		"package.json":  true,
		"composer.json": true,
		"go.mod":        true,
		"dockerfile":    true,
		"makefile":      true,
		"wails.json":    true,
		"tsconfig.json": true,
	}

	if configPatterns[lowerFile] {
		return 0.005
	}

	return 0.0
}

// calculateJaccardSimilarity computes Jaccard similarity between two sets of file paths
func calculateJaccardSimilarity(set1, set2 []string) float64 {
	if len(set1) == 0 && len(set2) == 0 {
		return 1.0 // Both empty sets are identical
	}

	// Convert to sets
	map1 := make(map[string]bool)
	map2 := make(map[string]bool)

	for _, item := range set1 {
		map1[item] = true
	}

	for _, item := range set2 {
		map2[item] = true
	}

	// Calculate intersection
	intersection := 0
	for item := range map1 {
		if map2[item] {
			intersection++
		}
	}

	// Calculate union
	union := len(map1) + len(map2) - intersection

	if union == 0 {
		return 1.0
	}

	return float64(intersection) / float64(union)
}

// getTopNPaths extracts the top N file paths from ImportantFiles
func getTopNPaths(files []shared.ImportantFile, n int) []string {
	var paths []string
	for i, file := range files {
		if i >= n {
			break
		}
		paths = append(paths, file.Path)
	}
	return paths
}

// detectRankChurn compares current top-20 with previous top-20 and returns churn metric
func detectRankChurn(current, previous []shared.ImportantFile) float64 {
	const topN = 20

	currentTop := getTopNPaths(current, topN)
	previousTop := getTopNPaths(previous, topN)

	// Calculate Jaccard distance (1 - similarity)
	similarity := calculateJaccardSimilarity(currentTop, previousTop)
	return 1.0 - similarity
}

// DetectRankChurn is the exported version of detectRankChurn
func DetectRankChurn(current, previous []shared.ImportantFile) float64 {
	return detectRankChurn(current, previous)
}
