//go:build !linux && !darwin && !windows

package storage

import (
	"fmt"
	"runtime"
)

func acquireDirectoryLock(dir string) (*directoryLock, error) {
	return nil, fmt.Errorf("exclusive data-directory locking is not supported on %s", runtime.GOOS)
}
