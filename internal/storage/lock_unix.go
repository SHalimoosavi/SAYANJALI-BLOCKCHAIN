//go:build linux || darwin

package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
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
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		closeErr := f.Close()
		releaseDirectoryLockReservation(key)
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, errors.Join(fmt.Errorf("data directory is already locked: %s", dir), closeErr)
		}
		return nil, errors.Join(fmt.Errorf("acquire data directory lock: %w", err), closeErr)
	}
	return &directoryLock{
		file: f,
		key:  key,
		unlock: func() error {
			return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		},
	}, nil
}
