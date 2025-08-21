package profiler

import (
	"context"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/loom/loom/internal/profiler/shared"
)

// FSScan handles file system scanning with ignore patterns
type FSScan struct {
	root       string
	ignoreSet  map[string]bool
	extensions map[string][]string
	basenames  map[string][]string
}

// NewFSScan creates a new file system scanner
func NewFSScan(root string) *FSScan {
	fs := &FSScan{
		root:       root,
		ignoreSet:  make(map[string]bool),
		extensions: make(map[string][]string),
		basenames:  make(map[string][]string),
	}

	// Initialize ignore patterns
	fs.setupIgnorePatterns()

	return fs
}

// setupIgnorePatterns configures the default ignore patterns
func (fs *FSScan) setupIgnorePatterns() {
	// Common ignore directories
	ignoreDirs := []string{
		"node_modules",
		"vendor",
		"dist",
		"build",
		"target",
		"bin",
		".git",
		".svn",
		".hg",
		".bzr",
		"__pycache__",
		".venv",
		"venv",
		".env",
		"coverage",
		".nyc_output",
		".next",
		".nuxt",
		".output",
		"bower_components",
		"jspm_packages",
		"tmp",
		"temp",
		".tmp",
		".temp",
		"logs",
		"*.log",
		".DS_Store",
		"Thumbs.db",
		".idea",
		".vscode",
		".vs",
		"*.swp",
		"*.swo",
		"*~",
	}

	for _, pattern := range ignoreDirs {
		fs.ignoreSet[pattern] = true
	}
}

// shouldIgnore checks if a path should be ignored
func (fs *FSScan) shouldIgnore(path string, info os.FileInfo) bool {
	name := filepath.Base(path)

	// Check exact matches
	if fs.ignoreSet[name] {
		return true
	}

	// Check patterns
	if strings.HasPrefix(name, ".") && name != "." && name != ".." {
		// Ignore most dotfiles except important ones
		important := map[string]bool{
			".gitignore":       true,
			".dockerignore":    true,
			".editorconfig":    true,
			".env.example":     true,
			".eslintrc.js":     true,
			".eslintrc.cjs":    true,
			".eslintrc.json":   true,
			".prettierrc":      true,
			".prettierrc.js":   true,
			".prettierrc.json": true,
			".babelrc":         true,
			".babelrc.js":      true,
			".babelrc.json":    true,
		}
		if !important[name] {
			return true
		}
	}

	// Check file extensions to ignore
	if strings.HasSuffix(name, ".min.js") ||
		strings.HasSuffix(name, ".min.css") ||
		strings.HasSuffix(name, ".map") ||
		strings.HasSuffix(name, ".lock") ||
		strings.Contains(name, ".generated.") ||
		strings.HasSuffix(name, ".pb.go") ||
		strings.HasSuffix(name, ".g.dart") {
		return true
	}

	// Hard skip for very large files (>5MB)
	if info.Size() > 5*1024*1024 {
		// Allow configs regardless of size if they are known names
		if !isConfigFile(name, filepath.Ext(path)) {
			return true
		}
	}

	// Large files (>512KB) unless they're important
	if info.Size() > 512*1024 {
		important := isImportantLargeFile(name)
		if !important {
			return true
		}
	}

	// Check for binary files using entropy detection
	if fs.isBinaryFile(path, info) {
		return true
	}

	return false
}

// isImportantLargeFile checks if a large file should still be included
func isImportantLargeFile(name string) bool {
	important := []string{
		"package-lock.json",
		"yarn.lock",
		"composer.lock",
		"Cargo.lock",
		"go.sum",
		"poetry.lock",
	}

	for _, imp := range important {
		if strings.Contains(name, imp) {
			return true
		}
	}

	return false
}

// categorizeFile determines the category and properties of a file
func (fs *FSScan) categorizeFile(path string, info os.FileInfo) *shared.FileInfo {
	name := filepath.Base(path)
	ext := strings.ToLower(filepath.Ext(path))

	fileInfo := &shared.FileInfo{
		Path:      shared.NormalizePath(path),
		Size:      info.Size(),
		Extension: ext,
		Basename:  name,
	}

	// Determine if it's a config file
	fileInfo.IsConfig = isConfigFile(name, ext)

	// Determine if it's documentation
	fileInfo.IsDoc = isDocFile(name, ext)

	// Determine if it's a script
	fileInfo.IsScript = isScriptFile(name, ext)

	// Determine if it's generated
	fileInfo.IsGenerated = isGeneratedFile(name, path)

	// Determine if it's vendored
	fileInfo.IsVendored = isVendoredFile(path)

	return fileInfo
}

// isConfigFile checks if a file is a configuration file
func isConfigFile(name, ext string) bool {
	configNames := map[string]bool{
		"package.json":        true,
		"composer.json":       true,
		"go.mod":              true,
		"go.sum":              true,
		"cargo.toml":          true,
		"cargo.lock":          true,
		"pyproject.toml":      true,
		"requirements.txt":    true,
		"dockerfile":          true,
		"docker-compose.yml":  true,
		"docker-compose.yaml": true,
		"makefile":            true,
		"justfile":            true,
		"taskfile.yml":        true,
		"procfile":            true,
		"wails.json":          true,
		"wails.toml":          true,
		"vite.config.ts":      true,
		"vite.config.js":      true,
		"webpack.config.js":   true,
		"rollup.config.js":    true,
		"tsconfig.json":       true,
		"jsconfig.json":       true,
		"babel.config.js":     true,
		"jest.config.js":      true,
		"vitest.config.ts":    true,
		"phpunit.xml":         true,
		"phpstan.neon":        true,
		"pint.json":           true,
		".editorconfig":       true,
		".gitignore":          true,
		".dockerignore":       true,
	}

	lowerName := strings.ToLower(name)
	if configNames[lowerName] {
		return true
	}

	// Check for config-like patterns
	if strings.Contains(lowerName, "config") ||
		strings.Contains(lowerName, ".rc") ||
		strings.HasPrefix(lowerName, ".eslint") ||
		strings.HasPrefix(lowerName, ".prettier") {
		return true
	}

	return false
}

// isDocFile checks if a file is documentation
func isDocFile(name, ext string) bool {
	lowerName := strings.ToLower(name)

	docNames := map[string]bool{
		"readme.md":       true,
		"readme.txt":      true,
		"readme":          true,
		"changelog.md":    true,
		"changelog":       true,
		"contributing.md": true,
		"contributing":    true,
		"architecture.md": true,
		"license":         true,
		"license.md":      true,
		"license.txt":     true,
		"authors":         true,
		"authors.md":      true,
		"todo.md":         true,
		"todo":            true,
	}

	if docNames[lowerName] {
		return true
	}

	// Check if in docs directory
	if strings.Contains(strings.ToLower(filepath.Dir(name)), "doc") {
		return true
	}

	// Check common doc extensions
	docExts := map[string]bool{
		".md":   true,
		".txt":  true,
		".rst":  true,
		".adoc": true,
	}

	return docExts[ext]
}

// isScriptFile checks if a file is a script
func isScriptFile(name, ext string) bool {
	lowerName := strings.ToLower(name)

	scriptNames := map[string]bool{
		"makefile":     true,
		"justfile":     true,
		"dockerfile":   true,
		"vagrantfile":  true,
		"rakefile":     true,
		"gemfile":      true,
		"procfile":     true,
		"taskfile.yml": true,
	}

	if scriptNames[lowerName] {
		return true
	}

	scriptExts := map[string]bool{
		".sh":   true,
		".bash": true,
		".zsh":  true,
		".fish": true,
		".bat":  true,
		".cmd":  true,
		".ps1":  true,
		".py":   true,
		".rb":   true,
		".pl":   true,
	}

	return scriptExts[ext]
}

// isGeneratedFile checks if a file is generated
func isGeneratedFile(name, path string) bool {
	lowerName := strings.ToLower(name)

	// Common generated file patterns
	patterns := []string{
		".generated.",
		".pb.go",
		".g.dart",
		"_pb2.py",
		".d.ts",
		".min.js",
		".min.css",
	}

	for _, pattern := range patterns {
		if strings.Contains(lowerName, pattern) {
			return true
		}
	}

	// Check for generated directories
	lowerPath := strings.ToLower(path)
	genDirs := []string{
		"/generated/",
		"/gen/",
		"/__generated__/",
		"/proto/",
		"/build/",
		"/dist/",
		"/target/",
	}

	for _, dir := range genDirs {
		if strings.Contains(lowerPath, dir) {
			return true
		}
	}

	return false
}

// isVendoredFile checks if a file is in a vendor directory
func isVendoredFile(path string) bool {
	lowerPath := strings.ToLower(path)

	vendorDirs := []string{
		"/vendor/",
		"/node_modules/",
		"/bower_components/",
		"/jspm_packages/",
		"/third_party/",
		"/external/",
		"/.venv/",
		"/venv/",
	}

	for _, dir := range vendorDirs {
		if strings.Contains(lowerPath, dir) {
			return true
		}
	}

	return false
}

// Scan performs the file system scan and returns categorized files
func (fs *FSScan) Scan(ctx context.Context) ([]*shared.FileInfo, map[string][]*shared.FileInfo, map[string][]*shared.FileInfo) {
	var files []*shared.FileInfo
	extensions := make(map[string][]*shared.FileInfo)
	basenames := make(map[string][]*shared.FileInfo)

	err := filepath.WalkDir(fs.root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors, continue scanning
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Get relative path and normalize it
		relPath, err := filepath.Rel(fs.root, path)
		if err != nil {
			relPath = path
		}
		relPath = shared.NormalizePath(relPath)

		info, err := d.Info()
		if err != nil {
			return nil // Skip files we can't stat
		}

		// Skip if should ignore
		if fs.shouldIgnore(relPath, info) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process files
		if !info.IsDir() {
			fileInfo := fs.categorizeFile(relPath, info)
			files = append(files, fileInfo)

			// Index by extension
			if fileInfo.Extension != "" {
				extensions[fileInfo.Extension] = append(extensions[fileInfo.Extension], fileInfo)
			}

			// Index by basename
			basenames[fileInfo.Basename] = append(basenames[fileInfo.Basename], fileInfo)
		}

		return nil
	})

	if err != nil && err != context.Canceled {
		_ = err // Log error but don't fail completely
	}

	return files, extensions, basenames
}

// isBinaryFile checks if a file is binary using entropy detection
func (fs *FSScan) isBinaryFile(path string, info os.FileInfo) bool {
	// Skip entropy check for very small files
	if info.Size() < 1024 {
		return false
	}

	// Skip known text file extensions
	ext := strings.ToLower(filepath.Ext(path))
	textExtensions := map[string]bool{
		".txt": true, ".md": true, ".json": true, ".yaml": true,
		".yml": true, ".xml": true, ".html": true, ".css": true,
		".js": true, ".ts": true, ".jsx": true, ".tsx": true,
		".go": true, ".py": true, ".java": true, ".c": true,
		".cpp": true, ".h": true, ".hpp": true, ".cs": true,
		".php": true, ".rb": true, ".rs": true, ".sh": true,
		".sql": true, ".toml": true, ".ini": true, ".cfg": true,
		".conf": true, ".log": true, ".csv": true, ".env": true,
	}

	if textExtensions[ext] {
		return false
	}

	// Calculate entropy from first 32KB sample
	return fs.calculateEntropy(path) > 7.5
}

// calculateEntropy calculates the Shannon entropy of a file sample
func (fs *FSScan) calculateEntropy(path string) float64 {
	file, err := os.Open(filepath.Join(fs.root, path))
	if err != nil {
		return 0
	}
	defer func() { _ = file.Close() }()

	// Read up to 32KB sample
	sample := make([]byte, 32*1024)
	n, err := file.Read(sample)
	if err != nil && err != io.EOF {
		return 0
	}

	if n == 0 {
		return 0
	}

	sample = sample[:n]

	// Count byte frequencies
	freq := make(map[byte]int)
	for _, b := range sample {
		freq[b]++
	}

	// Calculate Shannon entropy
	entropy := 0.0
	length := float64(len(sample))

	for _, count := range freq {
		if count > 0 {
			p := float64(count) / length
			entropy -= p * math.Log2(p)
		}
	}

	return entropy
}
