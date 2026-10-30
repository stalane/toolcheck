//go:build !windows

package lock

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func tryLock(file *os.File) error {
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return ErrAlreadyHeld
		}
		return fmt.Errorf("flock %s: %w", file.Name(), err)
	}
	return nil
}

func unlock(file *os.File) error {
	if err := unix.Flock(int(file.Fd()), unix.LOCK_UN); err != nil {
		return fmt.Errorf("unlock %s: %w", file.Name(), err)
	}
	return nil
}
