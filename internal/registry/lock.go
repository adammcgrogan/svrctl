package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// staleLockAge is how old a lock file must be before a waiting process
// assumes its owner crashed without cleaning up and steals it.
const staleLockAge = 10 * time.Second

// lockPath returns the sibling lock file for a registry path.
func lockPath(regPath string) string {
	return regPath + ".lock"
}

// acquireLock takes an exclusive, cross-process lock on the registry file,
// retrying until timeout. The lock is a directory entry created with O_EXCL,
// which is atomic on every OS svrctl supports (including Windows, unlike a
// plain file create).
func acquireLock(regPath string) (func(), error) {
	path := lockPath(regPath)
	deadline := time.Now().Add(5 * time.Second)

	for {
		err := os.Mkdir(path, 0o755)
		if err == nil {
			os.WriteFile(filepath.Join(path, "pid"), []byte(strconv.Itoa(os.Getpid())), 0o644)
			return func() { os.RemoveAll(path) }, nil
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("locking registry: %w", err)
		}

		if info, statErr := os.Stat(path); statErr == nil && time.Since(info.ModTime()) > staleLockAge {
			os.RemoveAll(path) // owner crashed without releasing; steal it
			continue
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("registry is locked by another svrctl process (stale lock? remove %s)", path)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// WithLock loads the registry, hands it to fn for mutation, and — if fn
// returns nil — saves the result, all while holding an exclusive lock so a
// concurrent svrctl process can't interleave its own read-modify-write and
// silently drop this change (or the other one).
func WithLock(regPath string, fn func(*Registry) error) error {
	release, err := acquireLock(regPath)
	if err != nil {
		return err
	}
	defer release()

	reg, err := Load(regPath)
	if err != nil {
		return err
	}
	if err := fn(reg); err != nil {
		return err
	}
	return reg.Save(regPath)
}
