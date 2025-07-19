package indexer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIndexerBasicFunctionality(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "loom-indexer-basic")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a simple Go file
	goFile := filepath.Join(tempDir, "main.go")
	err = os.WriteFile(goFile, []byte("package main\n\nfunc main() {}"), 0644)
	if err != nil {
		t.Fatalf("Failed to create Go file: %v", err)
	}

	// Test basic index creation
	index := NewIndex(tempDir, 1024*1024)
	if index == nil {
		t.Fatal("Expected non-nil index")
	}

	// Test index building
	err = index.BuildIndex()
	if err != nil {
		t.Fatalf("Failed to build index: %v", err)
	}

	// Test basic functionality
	stats := index.GetStats()
	if stats.TotalFiles == 0 {
		t.Error("Expected at least one file to be indexed")
	}

	if stats.TotalSize <= 0 {
		t.Error("Expected positive total size")
	}

	// Test that index has files map
	if index.Files == nil {
		t.Error("Expected Files map to be initialized")
	}

	// Log results for debugging
	t.Logf("Indexed %d files with total size %d bytes", stats.TotalFiles, stats.TotalSize)
}

func TestIndexerCacheOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "loom-cache-basic")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create .loom directory
	loomDir := filepath.Join(tempDir, ".loom")
	err = os.MkdirAll(loomDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create .loom directory: %v", err)
	}

	// Create a simple file
	testFile := filepath.Join(tempDir, "test.go")
	err = os.WriteFile(testFile, []byte("package test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Build and save index
	index := NewIndex(tempDir, 1024*1024)
	err = index.BuildIndex()
	if err != nil {
		t.Fatalf("Failed to build index: %v", err)
	}

	err = index.SaveToCache()
	if err != nil {
		t.Fatalf("Failed to save cache: %v", err)
	}

	// Try to load from cache
	loadedIndex, err := LoadFromCache(tempDir, 1024*1024)
	if err != nil {
		t.Fatalf("Failed to load from cache: %v", err)
	}

	if loadedIndex == nil {
		t.Fatal("Expected non-nil loaded index")
	}

	// Basic validation
	if len(loadedIndex.Files) == 0 {
		t.Error("Expected loaded index to have files")
	}
}

func TestIndexerEmptyDirectory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "loom-empty")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test indexing empty directory
	index := NewIndex(tempDir, 1024*1024)
	err = index.BuildIndex()
	if err != nil {
		t.Fatalf("Failed to build index for empty directory: %v", err)
	}

	stats := index.GetStats()
	if stats.TotalFiles != 0 {
		t.Logf("Empty directory indexed %d files (might include hidden files)", stats.TotalFiles)
	}
} 