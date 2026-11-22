package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const directoryLockFileName = ".syj.lock"

type directoryLock struct {
	file   *os.File
	key    string
	unlock func() error
}

var directoryLockRegistry = struct {
	sync.Mutex
	held map[string]struct{}
}{held: make(map[string]struct{})}

func reserveDirectoryLock(dir string) (string, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve data directory for lock: %w", err)
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve data directory for lock: %w", err)
	}
	key := filepath.Clean(canonical)

	directoryLockRegistry.Lock()
	defer directoryLockRegistry.Unlock()
	if _, exists := directoryLockRegistry.held[key]; exists {
		return "", fmt.Errorf("data directory is already locked: %s", dir)
	}
	directoryLockRegistry.held[key] = struct{}{}
	return key, nil
}

func releaseDirectoryLockReservation(key string) {
	directoryLockRegistry.Lock()
	delete(directoryLockRegistry.held, key)
	directoryLockRegistry.Unlock()
}

func releaseDirectoryLock(lock *directoryLock) error {
	if lock == nil {
		return nil
	}
	var unlockErr, closeErr error
	if lock.unlock != nil {
		unlockErr = lock.unlock()
	}
	if lock.file != nil {
		closeErr = lock.file.Close()
	}
	releaseDirectoryLockReservation(lock.key)
	return errors.Join(unlockErr, closeErr)
}
