package lang_graph

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/loom/loom/internal/profiler/shared"
)

// ComposerAutoload represents composer autoload configuration
type ComposerAutoload struct {
	PSR4                  map[string]interface{} `json:"psr-4"`
	ExcludeFromClassmap   []string               `json:"exclude-from-classmap"`
	ClassmapAuthoritative bool                   `json:"classmap-authoritative"`
}

// PSR4Mapping represents a PSR-4 namespace to directory mapping
type PSR4Mapping struct {
	Namespace   string
	Directories []string
}

// PHPGraphBuilder builds import graphs for PHP files
type PHPGraphBuilder struct {
	root                  string
	graph                 *shared.Graph
	psr4Map               map[string]string // legacy simple mapping
	psr4Mappings          []PSR4Mapping     // improved multiple directory support
	usesCache             map[string][]string
	excludeFromClassmap   []string
	classmapAuthoritative bool
}

// NewPHPGraphBuilder creates a new PHP graph builder
func NewPHPGraphBuilder(root string, graph *shared.Graph) *PHPGraphBuilder {
	return &PHPGraphBuilder{
		root:      root,
		graph:     graph,
		psr4Map:   make(map[string]string),
		usesCache: make(map[string][]string),
	}
}

// Build processes PHP files and builds the import graph
func (p *PHPGraphBuilder) Build(files []string, composerPSR map[string]string) error {
	p.psr4Map = composerPSR
	p.parseComposerPSR4(composerPSR)

	// First, add all files as vertices to ensure they appear in the graph
	for _, file := range files {
		normalizedFile := shared.NormalizePath(file)
		p.graph.Vertices[normalizedFile] = true
	}

	for _, file := range files {
		if err := p.processFile(file); err != nil {
			// Log error but continue processing other files
			continue
		}
	}

	// Add Laravel-specific edges if this appears to be a Laravel project
	p.addLaravelConventionEdges(files)

	return nil
}

// parseComposerPSR4 parses composer PSR-4 configuration with support for multiple directories
func (p *PHPGraphBuilder) parseComposerPSR4(composerPSR map[string]string) {
	p.psr4Mappings = nil

	for namespace, pathValue := range composerPSR {
		// Normalize namespace: ensure it ends with backslash
		namespace = strings.TrimSuffix(namespace, "\\") + "\\"

		// Handle both single directory and multiple directories
		directories := []string{pathValue}

		mapping := PSR4Mapping{
			Namespace:   namespace,
			Directories: directories,
		}
		p.psr4Mappings = append(p.psr4Mappings, mapping)
	}
}

// BuildWithComposerConfig processes PHP files with full composer autoload config
func (p *PHPGraphBuilder) BuildWithComposerConfig(files []string, autoload ComposerAutoload) error {
	p.parseComposerAutoload(autoload)

	for _, file := range files {
		if err := p.processFile(file); err != nil {
			// Log error but continue processing other files
			continue
		}
	}

	// Add Laravel-specific edges if this appears to be a Laravel project
	p.addLaravelConventionEdges(files)

	return nil
}

// parseComposerAutoload parses full composer autoload configuration
func (p *PHPGraphBuilder) parseComposerAutoload(autoload ComposerAutoload) {
	p.psr4Mappings = nil
	p.excludeFromClassmap = autoload.ExcludeFromClassmap
	p.classmapAuthoritative = autoload.ClassmapAuthoritative

	for namespace, pathValue := range autoload.PSR4 {
		// Normalize namespace: ensure it ends with backslash
		namespace = strings.TrimSuffix(namespace, "\\") + "\\"

		var directories []string

		// Handle different path value types
		switch v := pathValue.(type) {
		case string:
			directories = []string{v}
		case []interface{}:
			for _, dir := range v {
				if dirStr, ok := dir.(string); ok {
					directories = append(directories, dirStr)
				}
			}
		case []string:
			directories = v
		}

		if len(directories) > 0 {
			mapping := PSR4Mapping{
				Namespace:   namespace,
				Directories: directories,
			}
			p.psr4Mappings = append(p.psr4Mappings, mapping)
		}
	}
}

// processFile processes a single PHP file
func (p *PHPGraphBuilder) processFile(filePath string) error {
	content, err := os.ReadFile(filepath.Join(p.root, filePath))
	if err != nil {
		return err
	}

	contentStr := string(content)

	// Extract use statements
	uses := p.extractUseStatements(contentStr)
	p.usesCache[filePath] = uses

	// Extract class instantiations and static calls
	classRefs := p.extractClassReferences(contentStr)

	// Combine uses and class references
	allRefs := append(uses, classRefs...)

	for _, ref := range allRefs {
		resolved := p.resolveClassToFile(ref)
		if resolved != "" {
			// Add edge from current file to referenced file
			p.graph.AddEdge(filePath, resolved, 1.0)
		}
	}

	return nil
}

// extractUseStatements extracts use statements from PHP content
func (p *PHPGraphBuilder) extractUseStatements(content string) []string {
	var uses []string

	// Remove PHP comments to avoid false positives
	content = p.removePHPComments(content)

	// Patterns for use statements
	patterns := []*regexp.Regexp{
		// use Namespace\Class;
		regexp.MustCompile(`use\s+([A-Za-z\\][A-Za-z0-9\\]*);`),
		// use Namespace\Class as Alias;
		regexp.MustCompile(`use\s+([A-Za-z\\][A-Za-z0-9\\]*)\s+as\s+[A-Za-z][A-Za-z0-9]*;`),
		// use function Namespace\function;
		regexp.MustCompile(`use\s+function\s+([A-Za-z\\][A-Za-z0-9\\]*);`),
		// use const Namespace\CONSTANT;
		regexp.MustCompile(`use\s+const\s+([A-Za-z\\][A-Za-z0-9\\]*);`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				className := strings.TrimSpace(match[1])
				if p.isLocalClass(className) {
					uses = append(uses, className)
				}
			}
		}
	}

	return p.uniqueStrings(uses)
}

// extractClassReferences extracts class references from new and static calls
func (p *PHPGraphBuilder) extractClassReferences(content string) []string {
	var refs []string

	// Remove PHP comments to avoid false positives
	content = p.removePHPComments(content)

	// Patterns for class references
	patterns := []*regexp.Regexp{
		// new ClassName()
		regexp.MustCompile(`new\s+([A-Za-z\\][A-Za-z0-9\\]*)\s*\(`),
		// ClassName::method()
		regexp.MustCompile(`([A-Za-z\\][A-Za-z0-9\\]*)::[A-Za-z][A-Za-z0-9_]*\s*\(`),
		// ClassName::$property
		regexp.MustCompile(`([A-Za-z\\][A-Za-z0-9\\]*)::\$[A-Za-z][A-Za-z0-9_]*`),
		// ClassName::CONSTANT
		regexp.MustCompile(`([A-Za-z\\][A-Za-z0-9\\]*)::[A-Z][A-Z0-9_]*`),
		// instanceof ClassName
		regexp.MustCompile(`instanceof\s+([A-Za-z\\][A-Za-z0-9\\]*)`),
		// Type hints in function parameters
		regexp.MustCompile(`function\s+[A-Za-z][A-Za-z0-9_]*\s*\([^)]*?([A-Za-z\\][A-Za-z0-9\\]*)\s+\$`),
		// Return type hints
		regexp.MustCompile(`:\s*([A-Za-z\\][A-Za-z0-9\\]*)\s*{`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				className := strings.TrimSpace(match[1])
				if p.isLocalClass(className) {
					refs = append(refs, className)
				}
			}
		}
	}

	return p.uniqueStrings(refs)
}

// removePHPComments removes PHP comments to avoid false matches
func (p *PHPGraphBuilder) removePHPComments(content string) string {
	// Remove single-line comments
	singleLineCommentRe := regexp.MustCompile(`//.*$`)
	content = singleLineCommentRe.ReplaceAllString(content, "")

	// Remove # comments
	hashCommentRe := regexp.MustCompile(`#.*$`)
	content = hashCommentRe.ReplaceAllString(content, "")

	// Remove multi-line comments
	multiLineCommentRe := regexp.MustCompile(`/\*[\s\S]*?\*/`)
	content = multiLineCommentRe.ReplaceAllString(content, "")

	return content
}

// isLocalClass checks if a class name refers to a local class
func (p *PHPGraphBuilder) isLocalClass(className string) bool {
	// Skip built-in PHP classes
	builtInClasses := map[string]bool{
		"stdClass":         true,
		"Exception":        true,
		"DateTime":         true,
		"DateTimeZone":     true,
		"PDO":              true,
		"PDOStatement":     true,
		"ReflectionClass":  true,
		"ReflectionMethod": true,
		"Iterator":         true,
		"ArrayIterator":    true,
		"SplFileInfo":      true,
		"DOMDocument":      true,
		"SimpleXMLElement": true,
	}

	// Remove leading backslash
	className = strings.TrimPrefix(className, "\\")

	if builtInClasses[className] {
		return false
	}

	// Check if it maps to any configured PSR-4 namespace
	for namespace := range p.psr4Map {
		namespaceClean := strings.TrimSuffix(namespace, "\\")
		if strings.HasPrefix(className, namespaceClean) {
			return true
		}
	}

	return false
}

// resolveClassToFile resolves a class name to a file path using PSR-4 autoloading
func (p *PHPGraphBuilder) resolveClassToFile(className string) string {
	// Normalize class name: ensure it starts with backslash, then remove it
	if !strings.HasPrefix(className, "\\") {
		className = "\\" + className
	}
	className = strings.TrimPrefix(className, "\\")

	// Try improved PSR-4 mappings first
	if result := p.resolveWithPSR4Mappings(className); result != "" {
		return result
	}

	// Fallback to legacy mapping for backward compatibility
	for namespace, path := range p.psr4Map {
		namespaceClean := strings.TrimSuffix(namespace, "\\")

		if strings.HasPrefix(className, namespaceClean) {
			// Remove the namespace prefix
			relativePath := strings.TrimPrefix(className, namespaceClean)
			relativePath = strings.TrimPrefix(relativePath, "\\")

			// Convert namespace separators to directory separators
			relativePath = strings.ReplaceAll(relativePath, "\\", "/")

			// Construct the full path
			fullPath := filepath.Join(path, relativePath+".php")

			// Check if the file exists and is not excluded
			if p.fileExists(fullPath) && !p.isExcludedFromClassmap(fullPath) {
				return fullPath
			}
		}
	}

	return ""
}

// resolveWithPSR4Mappings resolves using improved PSR-4 mappings
func (p *PHPGraphBuilder) resolveWithPSR4Mappings(className string) string {
	// Sort by namespace length (longest first) for more specific matches
	for _, mapping := range p.psr4Mappings {
		namespace := strings.TrimSuffix(mapping.Namespace, "\\")

		if strings.HasPrefix(className, namespace) {
			// Remove the namespace prefix
			relativePath := strings.TrimPrefix(className, namespace)
			relativePath = strings.TrimPrefix(relativePath, "\\")

			// Convert namespace separators to directory separators
			relativePath = strings.ReplaceAll(relativePath, "\\", "/")

			// Try each directory in order
			for _, dir := range mapping.Directories {
				// Construct the full path
				fullPath := filepath.Join(dir, relativePath+".php")

				// Check if the file exists and is not excluded
				if p.fileExists(fullPath) && !p.isExcludedFromClassmap(fullPath) {
					return fullPath
				}
			}
		}
	}

	return ""
}

// isExcludedFromClassmap checks if a file should be excluded from classmap
func (p *PHPGraphBuilder) isExcludedFromClassmap(filePath string) bool {
	for _, excludePattern := range p.excludeFromClassmap {
		// Simple glob-style matching
		if matched, _ := filepath.Match(excludePattern, filePath); matched {
			return true
		}
		// Also check if the pattern is a substring
		if strings.Contains(filePath, excludePattern) {
			return true
		}
	}
	return false
}

// addLaravelConventionEdges adds edges based on Laravel conventions
func (p *PHPGraphBuilder) addLaravelConventionEdges(files []string) {
	routeFiles := p.findLaravelRouteFiles(files)
	controllerFiles := p.findLaravelControllerFiles(files)
	modelFiles := p.findLaravelModelFiles(files)
	migrationFiles := p.findLaravelMigrationFiles(files)
	policyFiles := p.findLaravelPolicyFiles(files)
	routeServiceProvider := p.findRouteServiceProvider(files)

	// Add edges from routes to controllers
	for _, routeFile := range routeFiles {
		for _, controllerFile := range controllerFiles {
			p.graph.AddEdge(routeFile, controllerFile, 0.6)
		}
	}

	// Add edges from models to migrations
	for _, modelFile := range modelFiles {
		for _, migrationFile := range migrationFiles {
			p.graph.AddEdge(modelFile, migrationFile, 0.4)
			p.graph.AddEdge(migrationFile, modelFile, 0.4)
		}
	}

	// Laravel route model binding: routes → models
	for _, routeFile := range routeFiles {
		for _, modelFile := range modelFiles {
			p.graph.AddEdge(routeFile, modelFile, 0.3)
		}
	}

	// Laravel policy discovery: models → policies
	for _, modelFile := range modelFiles {
		for _, policyFile := range policyFiles {
			p.graph.AddEdge(modelFile, policyFile, 0.4)
		}
	}

	// Laravel route service provider weak edges
	if routeServiceProvider != "" {
		for _, modelFile := range modelFiles {
			p.graph.AddEdge(routeServiceProvider, modelFile, 0.2)
		}
		for _, policyFile := range policyFiles {
			p.graph.AddEdge(routeServiceProvider, policyFile, 0.2)
		}
	}
}

// findLaravelRouteFiles finds Laravel route files
func (p *PHPGraphBuilder) findLaravelRouteFiles(files []string) []string {
	var routeFiles []string

	for _, file := range files {
		lowerPath := strings.ToLower(file)
		if strings.Contains(lowerPath, "routes/") {
			routeFiles = append(routeFiles, file)
		}
	}

	return routeFiles
}

// findLaravelControllerFiles finds Laravel controller files
func (p *PHPGraphBuilder) findLaravelControllerFiles(files []string) []string {
	var controllerFiles []string

	for _, file := range files {
		lowerPath := strings.ToLower(file)
		if strings.Contains(lowerPath, "app/http/controllers/") ||
			(strings.Contains(lowerPath, "controller") && strings.HasSuffix(lowerPath, ".php")) {
			controllerFiles = append(controllerFiles, file)
		}
	}

	return controllerFiles
}

// findLaravelModelFiles finds Laravel model files
func (p *PHPGraphBuilder) findLaravelModelFiles(files []string) []string {
	var modelFiles []string

	for _, file := range files {
		lowerPath := strings.ToLower(file)
		if strings.Contains(lowerPath, "app/models/") ||
			(strings.Contains(lowerPath, "app/") && !strings.Contains(lowerPath, "controller") &&
				!strings.Contains(lowerPath, "middleware") && strings.HasSuffix(lowerPath, ".php")) {
			modelFiles = append(modelFiles, file)
		}
	}

	return modelFiles
}

// findLaravelMigrationFiles finds Laravel migration files
func (p *PHPGraphBuilder) findLaravelMigrationFiles(files []string) []string {
	var migrationFiles []string

	for _, file := range files {
		lowerPath := strings.ToLower(file)
		if strings.Contains(lowerPath, "database/migrations/") {
			migrationFiles = append(migrationFiles, file)
		}
	}

	return migrationFiles
}

// findLaravelPolicyFiles finds Laravel policy files
func (p *PHPGraphBuilder) findLaravelPolicyFiles(files []string) []string {
	var policyFiles []string

	for _, file := range files {
		lowerPath := strings.ToLower(file)
		if strings.Contains(lowerPath, "app/policies/") ||
			(strings.Contains(lowerPath, "policy") && strings.HasSuffix(lowerPath, ".php")) {
			policyFiles = append(policyFiles, file)
		}
	}

	return policyFiles
}

// findRouteServiceProvider finds the Laravel RouteServiceProvider
func (p *PHPGraphBuilder) findRouteServiceProvider(files []string) string {
	for _, file := range files {
		lowerPath := strings.ToLower(file)
		if strings.Contains(lowerPath, "app/providers/routeserviceprovider.php") ||
			strings.Contains(lowerPath, "providers/routeserviceprovider.php") {
			return file
		}
	}
	return ""
}

// fileExists checks if a file exists relative to the project root
func (p *PHPGraphBuilder) fileExists(path string) bool {
	fullPath := filepath.Join(p.root, path)
	_, err := os.Stat(fullPath)
	return err == nil
}

// uniqueStrings removes duplicate strings from a slice
func (p *PHPGraphBuilder) uniqueStrings(strs []string) []string {
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
