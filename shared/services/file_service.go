package services

import (
	"fmt"
	"loom/indexer"
	"loom/shared/events"
	"loom/shared/models"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FileService handles file operations and workspace management
type FileService struct {
	index         *indexer.Index
	workspacePath string
	eventBus      *events.EventBus
	mutex         sync.RWMutex
}

// NewFileService creates a new file service
func NewFileService(workspacePath string, index *indexer.Index, eventBus *events.EventBus) *FileService {
	return &FileService{
		index:         index,
		workspacePath: workspacePath,
		eventBus:      eventBus,
	}
}

// GetFileTree returns the current file tree structure
func (fs *FileService) GetFileTree() ([]models.FileInfo, error) {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	var files []models.FileInfo

	// Walk through the workspace directory
	err := filepath.Walk(fs.workspacePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden files and directories
		if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Get relative path from workspace
		relPath, err := filepath.Rel(fs.workspacePath, path)
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		// Determine language from file extension
		language := ""
		if !info.IsDir() {
			language = fs.getLanguageFromPath(relPath)
		}

		fileInfo := models.FileInfo{
			Path:         relPath,
			Name:         info.Name(),
			Size:         info.Size(),
			IsDirectory:  info.IsDir(),
			Language:     language,
			ModifiedTime: info.ModTime(),
		}

		files = append(files, fileInfo)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk file tree: %w", err)
	}

	// Emit file tree updated event
	fs.eventBus.EmitFileTreeUpdate(files)

	return files, nil
}

// ReadFile reads the content of a file
func (fs *FileService) ReadFile(relativePath string) (string, error) {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	fullPath := filepath.Join(fs.workspacePath, relativePath)

	// Security check - ensure the path is within workspace
	if !strings.HasPrefix(fullPath, fs.workspacePath) {
		return "", fmt.Errorf("access denied: path outside workspace")
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", relativePath, err)
	}

	// Emit file opened event
	fs.eventBus.Emit(events.FileOpened, map[string]string{
		"path": relativePath,
	})

	return string(content), nil
}

// ReadFileLines reads specific lines from a file
func (fs *FileService) ReadFileLines(relativePath string, startLine, endLine int) (string, error) {
	content, err := fs.ReadFile(relativePath)
	if err != nil {
		return "", err
	}

	lines := strings.Split(content, "\n")

	// Validate line numbers (1-based)
	if startLine < 1 || endLine < 1 || startLine > len(lines) || endLine > len(lines) || startLine > endLine {
		return "", fmt.Errorf("invalid line range: %d-%d (file has %d lines)", startLine, endLine, len(lines))
	}

	// Extract requested lines (convert to 0-based indexing)
	selectedLines := lines[startLine-1 : endLine]
	return strings.Join(selectedLines, "\n"), nil
}

// SearchFiles searches for files matching a pattern
func (fs *FileService) SearchFiles(pattern string) ([]models.FileInfo, error) {
	allFiles, err := fs.GetFileTree()
	if err != nil {
		return nil, err
	}

	var matchingFiles []models.FileInfo
	pattern = strings.ToLower(pattern)

	for _, file := range allFiles {
		if strings.Contains(strings.ToLower(file.Name), pattern) ||
			strings.Contains(strings.ToLower(file.Path), pattern) {
			matchingFiles = append(matchingFiles, file)
		}
	}

	return matchingFiles, nil
}

// GetProjectSummary returns project summary with statistics
func (fs *FileService) GetProjectSummary() (models.ProjectSummary, error) {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	stats := fs.index.GetStats()

	// Convert language stats to map[string]float64
	languages := make(map[string]float64)
	totalFiles := float64(stats.TotalFiles)
	if totalFiles > 0 {
		for lang, count := range stats.LanguageBreakdown {
			languages[lang] = (float64(count) / totalFiles) * 100.0
		}
	}

	summary := models.ProjectSummary{
		Summary:     "Project indexed and ready", // This could be AI-generated
		Languages:   languages,
		FileCount:   stats.TotalFiles,
		TotalLines:  0, // Note: IndexStats doesn't have TotalLines field
		GeneratedAt: time.Now(),
	}

	return summary, nil
}

// GetFileAutocompleteOptions returns file suggestions for autocomplete
func (fs *FileService) GetFileAutocompleteOptions(query string) ([]string, error) {
	files, err := fs.GetFileTree()
	if err != nil {
		return nil, err
	}

	var options []string
	query = strings.ToLower(query)

	for _, file := range files {
		if !file.IsDirectory && strings.Contains(strings.ToLower(file.Path), query) {
			options = append(options, file.Path)
		}
	}

	// Limit to reasonable number of suggestions
	if len(options) > 10 {
		options = options[:10]
	}

	return options, nil
}

// WatchFiles starts watching for file changes (if supported by index)
func (fs *FileService) WatchFiles() error {
	// This would integrate with the existing file watching in indexer
	// For now, we'll emit periodic updates
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// Check if index has been updated
			if fs.index != nil {
				// Emit file tree update periodically
				files, err := fs.GetFileTree()
				if err == nil {
					fs.eventBus.EmitFileTreeUpdate(files)
				}
			}
		}
	}()

	return nil
}

// Helper methods

func (fs *FileService) getLanguageFromPath(path string) string {
	ext := filepath.Ext(path)
	switch ext {
	case ".go":
		return "Go"
	case ".js", ".jsx":
		return "JavaScript"
	case ".ts", ".tsx":
		return "TypeScript"
	case ".py":
		return "Python"
	case ".java":
		return "Java"
	case ".cpp", ".cc", ".cxx":
		return "C++"
	case ".c":
		return "C"
	case ".rs":
		return "Rust"
	case ".php":
		return "PHP"
	case ".rb":
		return "Ruby"
	case ".swift":
		return "Swift"
	case ".kt":
		return "Kotlin"
	case ".scala":
		return "Scala"
	case ".sh", ".bash":
		return "Shell"
	case ".html", ".htm":
		return "HTML"
	case ".css":
		return "CSS"
	case ".scss", ".sass":
		return "SCSS"
	case ".json":
		return "JSON"
	case ".xml":
		return "XML"
	case ".yml", ".yaml":
		return "YAML"
	case ".toml":
		return "TOML"
	case ".md", ".markdown":
		return "Markdown"
	case ".txt":
		return "Text"
	default:
		return ""
	}
}
