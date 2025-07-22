package indexer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func rgPath() string {
	// First try to find ripgrep in the system PATH
	if systemRg, err := exec.LookPath("rg"); err == nil {
		return systemRg
	}

	// Try to find the module root and look for bundled binary
	moduleRoot, err := findModuleRoot()
	if err != nil {
		// Fallback to relative path if we can't find module root
		moduleRoot = "."
	}

	base := filepath.Join(moduleRoot, "bin")
	var binaryName string
	switch runtime.GOOS {
	case "windows":
		binaryName = "rg-windows.exe"
	case "darwin":
		binaryName = "rg-macos"
	case "linux":
		binaryName = "rg-linux"
	default:
		panic(fmt.Sprintf("unsupported OS: %s", runtime.GOOS))
	}

	bundledPath := filepath.Join(base, binaryName)

	// Check if bundled binary exists
	if _, err := os.Stat(bundledPath); err == nil {
		return bundledPath
	}

	// If nothing found, return the bundled path anyway (will error later with helpful message)
	return bundledPath
}

// findModuleRoot finds the Go module root by looking for go.mod
func findModuleRoot() (string, error) {
	// Start from current working directory
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up the directory tree looking for go.mod
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the root directory
			break
		}
		dir = parent
	}

	return "", os.ErrNotExist
}

func RunRipgrep(pattern, path string) ([]byte, error) {
	rgBinary := rgPath()

	// Check if the binary exists and provide helpful error
	if _, err := os.Stat(rgBinary); os.IsNotExist(err) {
		return nil, fmt.Errorf("ripgrep binary not found at %s. Please install ripgrep system-wide using 'brew install ripgrep' or ensure the bin/ directory contains the ripgrep binary", rgBinary)
	}

	return exec.Command(rgBinary, pattern, path).CombinedOutput()
}

// RunRipgrepWithArgs runs ripgrep with custom arguments for advanced search features
func RunRipgrepWithArgs(args ...string) ([]byte, error) {
	rgBinary := rgPath()

	// Check if the binary exists and provide helpful error
	if _, err := os.Stat(rgBinary); os.IsNotExist(err) {
		return nil, fmt.Errorf("ripgrep binary not found at %s. Please install ripgrep system-wide using 'brew install ripgrep' or ensure the bin/ directory contains the ripgrep binary", rgBinary)
	}

	cmd := exec.Command(rgBinary, args...)
	return cmd.CombinedOutput()
}
