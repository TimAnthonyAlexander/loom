package lang_graph

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/loom/loom/internal/profiler/shared"
)

// PathMapping represents a TypeScript path mapping with preserved order
type PathMapping struct {
	Pattern  string
	Mappings []string
}

// TSGraphBuilder builds import graphs for TypeScript/JavaScript files
type TSGraphBuilder struct {
	root         string
	graph        *shared.Graph
	tsConfig     map[string]interface{}
	baseURL      string
	paths        map[string][]string
	pathMappings []PathMapping // Preserve order for better resolution
}

// NewTSGraphBuilder creates a new TypeScript graph builder
func NewTSGraphBuilder(root string, graph *shared.Graph) *TSGraphBuilder {
	return &TSGraphBuilder{
		root:  root,
		graph: graph,
		paths: make(map[string][]string),
	}
}

// Build processes TypeScript files and builds the import graph
func (ts *TSGraphBuilder) Build(files []string, tsConfig map[string]interface{}) error {
	ts.tsConfig = tsConfig
	ts.parseTypeScriptConfig()

	for _, file := range files {
		if err := ts.processFile(file); err != nil {
			// Log error but continue processing other files
			continue
		}
	}

	return nil
}

// parseTypeScriptConfig extracts path mapping from TypeScript config
func (ts *TSGraphBuilder) parseTypeScriptConfig() {
	if ts.tsConfig == nil {
		return
	}

	compilerOptions, ok := ts.tsConfig["compilerOptions"].(map[string]interface{})
	if !ok {
		return
	}

	// Extract baseUrl
	if baseURL, ok := compilerOptions["baseUrl"].(string); ok {
		ts.baseURL = baseURL
	}

	// Extract path mappings and preserve order where possible
	if pathsInterface, ok := compilerOptions["paths"].(map[string]interface{}); ok {
		// Clear existing mappings
		ts.pathMappings = nil

		for key, valueInterface := range pathsInterface {
			if valueArray, ok := valueInterface.([]interface{}); ok {
				var paths []string
				for _, v := range valueArray {
					if pathStr, ok := v.(string); ok {
						paths = append(paths, pathStr)
					}
				}
				ts.paths[key] = paths

				// Also store in ordered slice
				ts.pathMappings = append(ts.pathMappings, PathMapping{
					Pattern:  key,
					Mappings: paths,
				})
			}
		}
	}
}

// processFile processes a single TypeScript/JavaScript file
func (ts *TSGraphBuilder) processFile(filePath string) error {
	content, err := os.ReadFile(filepath.Join(ts.root, filePath))
	if err != nil {
		return err
	}

	imports := ts.extractImports(string(content))

	for _, imp := range imports {
		resolved := ts.resolveImport(imp, filePath)
		if resolved != "" {
			// Add edge from current file to imported file
			ts.graph.AddEdge(filePath, resolved, 1.0)
		}
	}

	return nil
}

// extractImports extracts import statements from TypeScript/JavaScript content
func (ts *TSGraphBuilder) extractImports(content string) []string {
	var imports []string

	// Remove comments to avoid false positives
	content = ts.removeComments(content)

	// Patterns for different import/require styles
	patterns := []*regexp.Regexp{
		// ES6 imports: import ... from 'module'
		regexp.MustCompile(`import\s+(?:[^'"]*\s+from\s+)?['"]([^'"]+)['"]`),
		// Dynamic imports: import('module')
		regexp.MustCompile(`import\s*\(\s*['"]([^'"]+)['"]\s*\)`),
		// CommonJS requires: require('module')
		regexp.MustCompile(`require\s*\(\s*['"]([^'"]+)['"]\s*\)`),
		// Re-exports: export ... from 'module'
		regexp.MustCompile(`export\s+(?:[^'"]*\s+from\s+)?['"]([^'"]+)['"]`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				importPath := match[1]
				// Skip non-relative imports that are external packages
				if ts.isLocalImport(importPath) {
					imports = append(imports, importPath)
				}
			}
		}
	}

	return ts.uniqueStrings(imports)
}

// removeComments removes JavaScript/TypeScript comments to avoid false import matches
func (ts *TSGraphBuilder) removeComments(content string) string {
	// Remove single-line comments
	singleLineCommentRe := regexp.MustCompile(`//.*$`)
	content = singleLineCommentRe.ReplaceAllString(content, "")

	// Remove multi-line comments (simple approach)
	multiLineCommentRe := regexp.MustCompile(`/\*[\s\S]*?\*/`)
	content = multiLineCommentRe.ReplaceAllString(content, "")

	return content
}

// isLocalImport checks if an import is local (relative or configured path mapping)
func (ts *TSGraphBuilder) isLocalImport(importPath string) bool {
	// Relative imports
	if strings.HasPrefix(importPath, "./") || strings.HasPrefix(importPath, "../") || strings.HasPrefix(importPath, "/") {
		return true
	}

	// Check if it matches any configured path mapping
	for pattern := range ts.paths {
		// Simple wildcard matching for path patterns like "@/*"
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(importPath, prefix) {
				return true
			}
		} else if pattern == importPath {
			return true
		}
	}

	// If we have a baseUrl, non-relative imports might be local
	if ts.baseURL != "" {
		// Check if the import resolves to a local file
		resolved := ts.resolveImport(importPath, "")
		return resolved != ""
	}

	return false
}

// resolveImport resolves an import path to an actual file path
func (ts *TSGraphBuilder) resolveImport(importPath, fromFile string) string {
	// Handle relative imports
	if strings.HasPrefix(importPath, "./") || strings.HasPrefix(importPath, "../") {
		fromDir := filepath.Dir(fromFile)
		resolved := filepath.Join(fromDir, importPath)
		return ts.findActualFile(resolved)
	}

	// Handle absolute imports
	if strings.HasPrefix(importPath, "/") {
		return ts.findActualFile(importPath[1:]) // Remove leading slash
	}

	// Handle path mappings
	resolved := ts.resolvePathMapping(importPath)
	if resolved != "" {
		return ts.findActualFile(resolved)
	}

	// Handle baseUrl resolution
	if ts.baseURL != "" {
		resolved := filepath.Join(ts.baseURL, importPath)
		return ts.findActualFile(resolved)
	}

	return ""
}

// resolvePathMapping resolves import using TypeScript path mappings with proper priority
func (ts *TSGraphBuilder) resolvePathMapping(importPath string) string {
	// First pass: check for exact matches in declaration order
	for _, pathMapping := range ts.pathMappings {
		if pathMapping.Pattern == importPath {
			// Exact match - try each mapping in order
			for _, mapping := range pathMapping.Mappings {
				actualFile := ts.findActualFile(mapping)
				if actualFile != "" {
					return actualFile
				}
			}
		}
	}

	// Second pass: check wildcard patterns in declaration order
	for _, pathMapping := range ts.pathMappings {
		pattern := pathMapping.Pattern
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(importPath, prefix) {
				suffix := strings.TrimPrefix(importPath, prefix)

				// Try each mapping in order until we find one that exists
				for _, mapping := range pathMapping.Mappings {
					if strings.HasSuffix(mapping, "*") {
						mappingPrefix := strings.TrimSuffix(mapping, "*")
						resolved := mappingPrefix + suffix
						actualFile := ts.findActualFile(resolved)
						if actualFile != "" {
							return actualFile
						}
					}
				}
			}
		}
	}

	return ""
}

// findActualFile finds the actual file given a path (handles extensions)
func (ts *TSGraphBuilder) findActualFile(basePath string) string {
	// Normalize path
	basePath = filepath.Clean(basePath)

	// If the path already has an extension and exists, return it
	if filepath.Ext(basePath) != "" {
		if ts.fileExists(basePath) {
			return basePath
		}
		return ""
	}

	// Try common TypeScript/JavaScript extensions
	extensions := []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"}

	for _, ext := range extensions {
		candidate := basePath + ext
		if ts.fileExists(candidate) {
			return candidate
		}
	}

	// Try index files
	indexExtensions := []string{"/index.ts", "/index.tsx", "/index.js", "/index.jsx"}
	for _, indexExt := range indexExtensions {
		candidate := basePath + indexExt
		if ts.fileExists(candidate) {
			return candidate
		}
	}

	return ""
}

// fileExists checks if a file exists relative to the project root
func (ts *TSGraphBuilder) fileExists(path string) bool {
	fullPath := filepath.Join(ts.root, path)
	_, err := os.Stat(fullPath)
	return err == nil
}

// uniqueStrings removes duplicate strings from a slice
func (ts *TSGraphBuilder) uniqueStrings(strs []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, str := range strs {
		if !seen[str] {
			seen[str] = true
			result = append(result, str)
		}
	}

	return result
}
