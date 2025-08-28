//go:build windows

package workflow

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32     = syscall.NewLazyDLL("kernel32.dll")
	lockFileEx   = kernel32.NewProc("LockFileEx")
	unlockFileEx = kernel32.NewProc("UnlockFileEx")
)

const (
	LOCKFILE_EXCLUSIVE_LOCK   = 0x00000002
	LOCKFILE_FAIL_IMMEDIATELY = 0x00000001
)

// flockFile applies an exclusive file lock using Windows LockFileEx
func flockFile(f *os.File) error {
	handle := syscall.Handle(f.Fd())
	overlapped := &syscall.Overlapped{}

	ret, _, err := lockFileEx.Call(
		uintptr(handle),
		uintptr(LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATELY),
		uintptr(0),
		uintptr(0xFFFFFFFF),
		uintptr(0xFFFFFFFF),
		uintptr(unsafe.Pointer(overlapped)))

	if ret == 0 {
		return err
	}
	return nil
}

// unlockFile releases the file lock using Windows UnlockFileEx
func unlockFile(f *os.File) error {
	handle := syscall.Handle(f.Fd())
	overlapped := &syscall.Overlapped{}

	ret, _, err := unlockFileEx.Call(
		uintptr(handle),
		uintptr(0),
		uintptr(0xFFFFFFFF),
		uintptr(0xFFFFFFFF),
		uintptr(unsafe.Pointer(overlapped)))

	if ret == 0 {
		return err
	}
	return nil
}
