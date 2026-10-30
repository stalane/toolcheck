package lock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrAlreadyHeld = errors.New("alertme: another instance is already running")

type Lock struct {
	file     *os.File
	released bool
}

func Acquire(path string) (*Lock, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create lock dir %s: %w", dir, err)
		}
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", path, err)
	}

	if err := tryLock(file); err != nil {
		file.Close()
		return nil, err
	}

	if err := file.Truncate(0); err == nil {
		fmt.Fprintf(file, "%d\n", os.Getpid())
		file.Sync()
	}

	return &Lock{file: file}, nil
}

func (l *Lock) Release() error {
	if l == nil || l.released {
		return nil
	}
	l.released = true

	unlockErr := unlock(l.file)
	closeErr := l.file.Close()
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}
