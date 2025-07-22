package indexer

import (
	"compress/gzip"
	"crypto/sha1"
	"encoding/gob"
	"fmt"
	"io"
	"loom/paths"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileMeta represents metadata for a single file
type FileMeta struct {
	RelativePath string    `json:"relative_path"`
	Size         int64     `json:"size"`
	ModTime      time.Time `json:"mod_time"`
	Hash         string    `json:"hash"`
	Language     string    `json:"language"`
}

// Index represents the workspace file index
type Index struct {
	WorkspacePath string               `json:"workspace_path"`
	Files         map[string]*FileMeta `json:"files"`
	LastUpdated   time.Time            `json:"last_updated"`
	projectPaths  *paths.ProjectPaths  `json:"-"`
	gitIgnore     *GitIgnore           `json:"-"`
	watcher       *fsnotify.Watcher    `json:"-"`
	watcherMutex  sync.RWMutex         `json:"-"`
	updateChan    chan string          `json:"-"`
	stopChan      chan bool            `json:"-"`
	maxFileSize   int64                `json:"-"`
}

// IndexStats represents statistics about the indexed files
type IndexStats struct {
	TotalFiles        int                `json:"total_files"`
	TotalSize         int64              `json:"total_size"`
	LanguageBreakdown map[string]int     `json:"language_breakdown"`
	LanguagePercent   map[string]float64 `json:"language_percent"`
}

// NewIndex creates a new Index instance
func NewIndex(workspacePath string, maxFileSize int64) *Index {
	// Get project paths
	projectPaths, err := paths.NewProjectPaths(workspacePath)
	if err != nil {
		// Fallback to nil if paths creation fails - cache will be disabled
		fmt.Printf("Warning: failed to create project paths: %v\n", err)
		projectPaths = nil
	} else {
		// Ensure project directories exist
		if err := projectPaths.EnsureProjectDir(); err != nil {
			fmt.Printf("Warning: failed to create project directories: %v\n", err)
		}
	}

	return &Index{
		WorkspacePath: workspacePath,
		Files:         make(map[string]*FileMeta),
		LastUpdated:   time.Now(),
		projectPaths:  projectPaths,
		maxFileSize:   maxFileSize,
		updateChan:    make(chan string, 100),
		stopChan:      make(chan bool, 1),
	}
}

// BuildIndex performs a full scan of the workspace
func (idx *Index) BuildIndex() error {
	// Load .gitignore patterns
	gitIgnore, err := LoadGitIgnore(idx.WorkspacePath)
	if err != nil {
		// Continue without .gitignore if it fails to load
		gitIgnore = &GitIgnore{}
	}
	idx.gitIgnore = gitIgnore

	// Clear existing files
	idx.Files = make(map[string]*FileMeta)

	// Create worker pool for parallel processing
	numWorkers := runtime.NumCPU()
	fileChan := make(chan string, 100)
	resultChan := make(chan *FileMeta, 100)
	var wg sync.WaitGroup
	var resultWg sync.WaitGroup

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go idx.indexWorker(fileChan, resultChan, &wg)
	}

	// Start result collector with proper synchronization
	resultWg.Add(1)
	go func() {
		defer resultWg.Done()
		for meta := range resultChan {
			if meta != nil {
				idx.Files[meta.RelativePath] = meta
			}
		}
	}()

	// Walk directory tree
	err = filepath.Walk(idx.WorkspacePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		// Skip directories
		if info.IsDir() {
			// Skip ignored directories
			relPath, _ := filepath.Rel(idx.WorkspacePath, path)
			if idx.shouldSkipDirectory(relPath) {
				return filepath.SkipDir
			}
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(idx.WorkspacePath, path)
		if err != nil {
			return nil
		}

		// Check if file should be indexed
		if idx.shouldSkipFile(relPath, info) {
			return nil
		}

		// Send to workers
		fileChan <- path
		return nil
	})

	// Close file channel and wait for workers
	close(fileChan)
	wg.Wait()

	// Close result channel and wait for result collector
	close(resultChan)
	resultWg.Wait()

	idx.LastUpdated = time.Now()
	return err
}

// indexWorker processes files in parallel
func (idx *Index) indexWorker(fileChan <-chan string, resultChan chan<- *FileMeta, wg *sync.WaitGroup) {
	defer wg.Done()

	for path := range fileChan {
		meta := idx.indexFile(path)
		resultChan <- meta
	}
}

// indexFile creates FileMeta for a single file
func (idx *Index) indexFile(path string) *FileMeta {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}

	relPath, err := filepath.Rel(idx.WorkspacePath, path)
	if err != nil {
		return nil
	}

	// Normalize path separators to forward slashes for cross-platform consistency
	relPath = filepath.ToSlash(relPath)

	// Calculate hash
	hash, err := idx.calculateFileHash(path)
	if err != nil {
		return nil
	}

	// Determine language
	language := getLanguageFromExtension(filepath.Ext(path))

	return &FileMeta{
		RelativePath: relPath,
		Size:         info.Size(),
		ModTime:      info.ModTime(),
		Hash:         hash,
		Language:     language,
	}
}

// calculateFileHash computes SHA-1 hash of file content
func (idx *Index) calculateFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha1.New()
	_, err = io.Copy(hasher, file)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

// shouldSkipDirectory checks if a directory should be skipped
func (idx *Index) shouldSkipDirectory(relPath string) bool {
	// Skip common ignored directories - removed .loom since it's no longer in workspace
	skipDirs := []string{".git", "node_modules", "vendor", ".vscode", ".idea", "target", "dist", "__pycache__"}

	dirName := filepath.Base(relPath)
	for _, skip := range skipDirs {
		if dirName == skip {
			return true
		}
	}

	// Check .gitignore
	if idx.gitIgnore != nil && idx.gitIgnore.MatchesPath(relPath) {
		return true
	}

	return false
}

// shouldSkipFile checks if a file should be skipped
func (idx *Index) shouldSkipFile(relPath string, info os.FileInfo) bool {
	// Skip large files
	if info.Size() > idx.maxFileSize {
		return true
	}

	// Skip .gitignore files
	if filepath.Base(relPath) == ".gitignore" {
		return true
	}

	// Skip binary files (basic check)
	if isBinaryFile(relPath) {
		return true
	}

	// Check .gitignore
	if idx.gitIgnore != nil && idx.gitIgnore.MatchesPath(relPath) {
		return true
	}

	return false
}

// GetStats returns statistics about the indexed files
func (idx *Index) GetStats() IndexStats {
	langBreakdown := make(map[string]int)
	var totalSize int64

	for _, meta := range idx.Files {
		langBreakdown[meta.Language]++
		totalSize += meta.Size
	}

	// Calculate percentages
	langPercent := make(map[string]float64)
	totalFiles := len(idx.Files)
	if totalFiles > 0 {
		for lang, count := range langBreakdown {
			langPercent[lang] = float64(count) / float64(totalFiles) * 100
		}
	}

	return IndexStats{
		TotalFiles:        totalFiles,
		TotalSize:         totalSize,
		LanguageBreakdown: langBreakdown,
		LanguagePercent:   langPercent,
	}
}

// SaveToCache saves the index to user loom directory
func (idx *Index) SaveToCache() error {
	if idx.projectPaths == nil {
		// No project paths available, skip caching
		return nil
	}

	cachePath := idx.projectPaths.IndexCachePath()

	file, err := os.Create(cachePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Use gzip compression
	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	encoder := gob.NewEncoder(gzWriter)
	return encoder.Encode(idx)
}

// LoadFromCache loads the index from user loom directory
func LoadFromCache(workspacePath string, maxFileSize int64) (*Index, error) {
	// Get project paths
	projectPaths, err := paths.NewProjectPaths(workspacePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create project paths: %w", err)
	}

	cachePath := projectPaths.IndexCachePath()

	file, err := os.Open(cachePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gzReader.Close()

	var idx Index
	decoder := gob.NewDecoder(gzReader)
	err = decoder.Decode(&idx)
	if err != nil {
		return nil, err
	}

	// Restore runtime fields
	idx.projectPaths = projectPaths
	idx.updateChan = make(chan string, 100)
	idx.stopChan = make(chan bool, 1)
	idx.maxFileSize = maxFileSize

	return &idx, nil
}

// StartWatching starts file system watching for incremental updates
func (idx *Index) StartWatching() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	idx.watcher = watcher

	// Add workspace to watcher
	err = filepath.Walk(idx.WorkspacePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			relPath, _ := filepath.Rel(idx.WorkspacePath, path)
			if !idx.shouldSkipDirectory(relPath) {
				return watcher.Add(path)
			}
			return filepath.SkipDir
		}
		return nil
	})

	if err != nil {
		return err
	}

	// Start watching goroutine
	go idx.watchLoop()

	return nil
}

// StopWatching stops file system watching
func (idx *Index) StopWatching() {
	if idx.watcher != nil {
		idx.stopChan <- true
		idx.watcher.Close()
	}
}

// watchLoop handles file system events
func (idx *Index) watchLoop() {
	updateTimer := time.NewTimer(500 * time.Millisecond)
	updateTimer.Stop()
	pendingUpdates := make(map[string]bool)

	for {
		select {
		case event, ok := <-idx.watcher.Events:
			if !ok {
				return
			}

			relPath, err := filepath.Rel(idx.WorkspacePath, event.Name)
			if err != nil {
				continue
			}

			// Add to pending updates
			pendingUpdates[relPath] = true

			// Reset timer for batching
			updateTimer.Stop()
			updateTimer.Reset(500 * time.Millisecond)

		case <-updateTimer.C:
			// Process batched updates
			for relPath := range pendingUpdates {
				idx.updateFile(relPath)
			}
			pendingUpdates = make(map[string]bool)

		case err, ok := <-idx.watcher.Errors:
			if !ok {
				return
			}
			fmt.Printf("Watcher error: %v\n", err)

		case <-idx.stopChan:
			return
		}
	}
}

// updateFile updates a single file in the index
func (idx *Index) updateFile(relPath string) {
	idx.watcherMutex.Lock()
	defer idx.watcherMutex.Unlock()

	fullPath := filepath.Join(idx.WorkspacePath, relPath)
	info, err := os.Stat(fullPath)

	if os.IsNotExist(err) {
		// File deleted
		delete(idx.Files, relPath)
		return
	}

	if err != nil || info.IsDir() {
		return
	}

	if idx.shouldSkipFile(relPath, info) {
		delete(idx.Files, relPath)
		return
	}

	// Update file metadata
	meta := idx.indexFile(fullPath)
	if meta != nil {
		idx.Files[relPath] = meta
	}
}

// GetFileList returns a sorted list of file paths
func (idx *Index) GetFileList() []string {
	files := make([]string, 0, len(idx.Files))
	for path := range idx.Files {
		files = append(files, path)
	}
	return files
}

// isBinaryFile checks if a file is likely binary based on extension
func isBinaryFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	binaryExts := []string{
		".exe", ".dll", ".so", ".dylib", ".a", ".o", ".obj",
		".bin", ".dat", ".db", ".sqlite", ".sqlite3",
		".jpg", ".jpeg", ".png", ".gif", ".bmp", ".ico", ".tiff",
		".mp3", ".mp4", ".avi", ".mov", ".wmv", ".flv",
		".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
		".zip", ".tar", ".gz", ".bz2", ".7z", ".rar",
		".woff", ".woff2", ".ttf", ".otf", ".eot",
	}

	for _, binExt := range binaryExts {
		if ext == binExt {
			return true
		}
	}
	return false
}

// getLanguageFromExtension maps file extensions to programming languages
func getLanguageFromExtension(ext string) string {
	ext = strings.ToLower(ext)
	langMap := map[string]string{
		".go":         "Go",
		".js":         "JavaScript",
		".ts":         "TypeScript",
		".jsx":        "React",
		".tsx":        "React",
		".py":         "Python",
		".java":       "Java",
		".c":          "C",
		".cpp":        "C++",
		".cc":         "C++",
		".cxx":        "C++",
		".h":          "C/C++",
		".hpp":        "C++",
		".cs":         "C#",
		".php":        "PHP",
		".rb":         "Ruby",
		".rs":         "Rust",
		".swift":      "Swift",
		".kt":         "Kotlin",
		".scala":      "Scala",
		".clj":        "Clojure",
		".elm":        "Elm",
		".hs":         "Haskell",
		".ml":         "OCaml",
		".fs":         "F#",
		".dart":       "Dart",
		".lua":        "Lua",
		".perl":       "Perl",
		".pl":         "Perl",
		".r":          "R",
		".matlab":     "MATLAB",
		".sh":         "Shell",
		".bash":       "Bash",
		".zsh":        "Zsh",
		".fish":       "Fish",
		".ps1":        "PowerShell",
		".html":       "HTML",
		".htm":        "HTML",
		".css":        "CSS",
		".scss":       "SCSS",
		".sass":       "Sass",
		".less":       "Less",
		".xml":        "XML",
		".json":       "JSON",
		".yaml":       "YAML",
		".yml":        "YAML",
		".toml":       "TOML",
		".ini":        "INI",
		".cfg":        "Config",
		".conf":       "Config",
		".md":         "Markdown",
		".txt":        "Text",
		".sql":        "SQL",
		".graphql":    "GraphQL",
		".dockerfile": "Docker",
		".makefile":   "Makefile",
		".cmake":      "CMake",
		".gradle":     "Gradle",
		".maven":      "Maven",
		".ant":        "Ant",
	}

	if lang, exists := langMap[ext]; exists {
		return lang
	}

	if ext == "" {
		return "No Extension"
	}

	return "Other"
}
