package docsurface

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"

	"aladin/backend_v2/internal/service"
)

// Lock the stable page directory, not the file (which atomicWrite replaces).
// flock coordinates independent API/MCP processes on the local data volume.
// All store writers/deletes participate; external filesystem writes do not.
func (l *localStore) lockMutation(ctx context.Context, pageID string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dir, err := l.safePath(ctx, pageID, "")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	for {
		if err := ctx.Err(); err != nil {
			_ = f.Close()
			return nil, err
		}
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			_ = f.Close()
			return nil, err
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			_ = f.Close()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func (l *localStore) CompareAndSwapFile(ctx context.Context, pageID, rel string, previous, next []byte) error {
	// Validate before even creating the page directory/obtaining its lock.
	if _, err := l.safePath(ctx, pageID, rel); err != nil {
		return err
	}
	unlock, err := l.lockMutation(ctx, pageID)
	if err != nil {
		return err
	}
	defer unlock()
	current, err := l.ReadFile(ctx, pageID, rel)
	if errors.Is(err, service.ErrNotFound) || (err == nil && !bytes.Equal(current, previous)) {
		return fmt.Errorf("%w: %s changed since it was read; read_file again before editing", service.ErrConflict, rel)
	}
	if err != nil {
		return err
	}
	return l.writeFile(ctx, pageID, rel, next)
}
