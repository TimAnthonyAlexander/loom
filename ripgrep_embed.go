package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// Embed ripgrep binaries for different platforms
//
//go:embed bin/rg-linux
var rgLinux []byte

//go:embed bin/rg-macos
var rgMacos []byte

//go:embed bin/rg-windows.exe
var rgWindows []byte

var (
	embeddedRgPath string
	embeddedOnce   sync.Once
	embeddedErr    error
)

// GetEmbeddedRipgrepPath extracts and returns the path to the embedded ripgrep binary
func GetEmbeddedRipgrepPath() (string, error) {
	embeddedOnce.Do(func() {
		embeddedRgPath, embeddedErr = extractEmbeddedRipgrep()
	})
	return embeddedRgPath, embeddedErr
}

// extractEmbeddedRipgrep extracts the embedded ripgrep binary to a temporary location
func extractEmbeddedRipgrep() (string, error) {
	var binaryData []byte
	var fileName string

	switch runtime.GOOS {
	case "windows":
		binaryData = rgWindows
		fileName = "rg.exe"
	case "darwin":
		binaryData = rgMacos
		fileName = "rg"
	case "linux":
		binaryData = rgLinux
		fileName = "rg"
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	// Create a persistent temp directory for this process
	tempDir, err := os.MkdirTemp("", "loom-ripgrep-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Extract binary to temp location
	binaryPath := filepath.Join(tempDir, fileName)
	err = os.WriteFile(binaryPath, binaryData, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to extract ripgrep binary: %w", err)
	}

	return binaryPath, nil
}
