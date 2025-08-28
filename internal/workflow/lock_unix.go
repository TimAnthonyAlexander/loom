//go:build unix

package workflow

import (
	"os"
	"syscall"
)

// flockFile applies an exclusive file lock using Unix flock syscall
func flockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

// unlockFile releases the file lock using Unix flock syscall
func unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
