//go:build windows

package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

const (
	lockFileFailImmediately = 0x00000001
	lockFileExclusive       = 0x00000002
	errorLockViolation      = syscall.Errno(33)
)

var (
	lockFileExProc   = syscall.NewLazyDLL("kernel32.dll").NewProc("LockFileEx")
	unlockFileExProc = syscall.NewLazyDLL("kernel32.dll").NewProc("UnlockFileEx")
)

func acquireDirectoryLock(dir string) (*directoryLock, error) {
	key, err := reserveDirectoryLock(dir)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, directoryLockFileName), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		releaseDirectoryLockReservation(key)
		return nil, fmt.Errorf("open data directory lock: %w", err)
	}
	overlapped := new(syscall.Overlapped)
	result, _, callErr := lockFileExProc.Call(
		f.Fd(),
		uintptr(lockFileExclusive|lockFileFailImmediately),
		0,
		1,
		0,
		uintptr(unsafe.Pointer(overlapped)),
	)
	runtime.KeepAlive(overlapped)
	runtime.KeepAlive(f)
	if result == 0 {
		if callErr == syscall.Errno(0) {
			callErr = syscall.EINVAL
		}
		closeErr := f.Close()
		releaseDirectoryLockReservation(key)
		if errors.Is(callErr, errorLockViolation) {
			return nil, errors.Join(fmt.Errorf("data directory is already locked: %s", dir), closeErr)
		}
		return nil, errors.Join(fmt.Errorf("acquire data directory lock: %w", callErr), closeErr)
	}
	return &directoryLock{
		file: f,
		key:  key,
		unlock: func() error {
			result, _, callErr := unlockFileExProc.Call(
				f.Fd(),
				0,
				1,
				0,
				uintptr(unsafe.Pointer(overlapped)),
			)
			runtime.KeepAlive(overlapped)
			runtime.KeepAlive(f)
			if result == 0 {
				if callErr == syscall.Errno(0) {
					return syscall.EINVAL
				}
				return callErr
			}
			return nil
		},
	}, nil
}
