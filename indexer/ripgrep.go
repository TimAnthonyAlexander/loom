package indexer

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func rgPath() string {
	// Try to find the module root by looking for go.mod
	moduleRoot, err := findModuleRoot()
	if err != nil {
		// Fallback to relative path if we can't find module root
		moduleRoot = "."
	}

	base := filepath.Join(moduleRoot, "bin")
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(base, "rg-windows.exe")
	case "darwin":
		return filepath.Join(base, "rg-macos")
	case "linux":
		return filepath.Join(base, "rg-linux")
	}
	panic("unsupported OS")
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
	return exec.Command(rgPath(), pattern, path).CombinedOutput()
}

// RunRipgrepWithArgs runs ripgrep with custom arguments for advanced search features
func RunRipgrepWithArgs(args ...string) ([]byte, error) {
	cmd := exec.Command(rgPath(), args...)
	return cmd.CombinedOutput()
}
