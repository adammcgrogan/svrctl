package registry

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestWithLockSerializesConcurrentWriters(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.yaml")

	const n = 20
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := filepath.Join("srv", string(rune('a'+i)))
			err := WithLock(path, func(reg *Registry) error {
				reg.Put(name, Server{Type: "vanilla", Version: "1.21.1"})
				return nil
			})
			if err != nil {
				t.Errorf("WithLock: %v", err)
			}
		}(i)
	}
	wg.Wait()

	reg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(reg.Servers) != n {
		t.Errorf("expected %d servers after concurrent writes, got %d", n, len(reg.Servers))
	}
}

func TestWithLockErrorLeavesRegistryUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.yaml")

	reg := &Registry{Servers: map[string]Server{}}
	reg.Put("survival", Server{Type: "paper"})
	if err := reg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	wantErr := WithLock(path, func(reg *Registry) error {
		reg.Remove("survival")
		return errAbort
	})
	if wantErr != errAbort {
		t.Fatalf("expected errAbort, got %v", wantErr)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := loaded.Get("survival"); !ok {
		t.Errorf("expected survival to remain after fn returned an error")
	}
}

func TestAcquireLockStealsStaleLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.yaml")

	release, err := acquireLock(path)
	if err != nil {
		t.Fatalf("acquireLock: %v", err)
	}
	_ = release // simulate a crash: never call it

	// Back-date the lock so it looks stale without a real sleep.
	old := time.Now().Add(-staleLockAge - time.Second)
	if err := os.Chtimes(lockPath(path), old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	release2, err := acquireLock(path)
	if err != nil {
		t.Fatalf("acquireLock did not steal stale lock: %v", err)
	}
	release2()
}

var errAbort = &abortError{"abort"}

type abortError struct{ msg string }

func (e *abortError) Error() string { return e.msg }
