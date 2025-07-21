package indexer

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

func rgPath() string {
	base := "bin"
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

func RunRipgrep(pattern, path string) ([]byte, error) {
	return exec.Command(rgPath(), pattern, path).CombinedOutput()
}
