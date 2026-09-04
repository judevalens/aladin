package docsurface

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"aladin/backend_v2/internal/service"
)

func TestStoreConcurrentCAS(t *testing.T) {
	store := searchStore(t, map[string]string{"a.ts": "before"})
	start := make(chan struct{})
	results := make(chan error, 16)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			// Independent store objects must still share the same lock.
			other := NewStore(store.root).(service.DocSurfaceFileCAS)
			results <- other.CompareAndSwapFile(testCtx(), "p1", "a.ts", []byte("before"), []byte(fmt.Sprintf("after-%d", i)))
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	wins, conflicts := 0, 0
	for err := range results {
		if err == nil {
			wins++
		} else if errors.Is(err, service.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if wins != 1 || conflicts != 15 {
		t.Fatalf("wins=%d conflicts=%d", wins, conflicts)
	}
}

func TestStoreMutationLockCancellation(t *testing.T) {
	store := searchStore(t, map[string]string{"a.ts": "before"})
	unlock, err := store.lockMutation(testCtx(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	for _, operation := range []struct {
		name string
		run  func(context.Context) error
	}{
		{"write", func(ctx context.Context) error { return store.WriteFile(ctx, "p1", "a.ts", []byte("after")) }},
		{"delete", func(ctx context.Context) error { return store.DeleteFile(ctx, "p1", "a.ts") }},
		{"CAS", func(ctx context.Context) error {
			return store.CompareAndSwapFile(ctx, "p1", "a.ts", []byte("before"), []byte("after"))
		}},
	} {
		t.Run(operation.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(testCtx(), 30*time.Millisecond)
			defer cancel()
			if err := operation.run(ctx); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("%v", err)
			}
			data, err := store.ReadFile(testCtx(), "p1", "a.ts")
			if err != nil || string(data) != "before" {
				t.Fatalf("%q %v", data, err)
			}
		})
	}
}

func TestStoreCASMissingAndTraversal(t *testing.T) {
	store := searchStore(t, map[string]string{"a.ts": "before"})
	if err := store.CompareAndSwapFile(testCtx(), "p1", "missing.ts", nil, []byte("after")); !errors.Is(err, service.ErrConflict) {
		t.Fatal(err)
	}
	for _, name := range []string{"../p2/a.ts", "/tmp/escape"} {
		if err := store.CompareAndSwapFile(testCtx(), "p1", name, nil, []byte("after")); err == nil {
			t.Fatal("traversal accepted")
		}
	}
	if err := store.CompareAndSwapFile(context.Background(), "p1", "a.ts", []byte("before"), []byte("after")); err == nil {
		t.Fatal("principal not enforced")
	}
}

// The helper runs in an independent OS process, not just another goroutine.
func TestStoreCASProcessHelper(t *testing.T) {
	root := os.Getenv("ALADIN_CAS_TEST_ROOT")
	if root == "" {
		return
	}
	store := NewStore(root).(service.DocSurfaceFileCAS)
	fmt.Println("ready")
	err := store.CompareAndSwapFile(testCtx(), "p1", "a.ts", []byte("before"), []byte(os.Getenv("ALADIN_CAS_TEST_VALUE")))
	if errors.Is(err, service.ErrConflict) {
		os.Exit(10)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(11)
	}
	os.Exit(0)
}

func TestStoreCASAcrossProcesses(t *testing.T) {
	store := searchStore(t, map[string]string{"a.ts": "before"})
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	unlock, err := store.lockMutation(testCtx(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if unlock != nil {
			unlock()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var commands []*exec.Cmd
	for i := 0; i < 4; i++ {
		cmd := exec.CommandContext(ctx, executable, "-test.run=^TestStoreCASProcessHelper$")
		cmd.Env = append(os.Environ(), "ALADIN_CAS_TEST_ROOT="+store.root, fmt.Sprintf("ALADIN_CAS_TEST_VALUE=child-%d", i))
		cmd.Stderr = os.Stderr
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		commands = append(commands, cmd)
		line, err := bufio.NewReader(stdout).ReadString('\n')
		if err != nil || line != "ready\n" {
			t.Fatalf("child readiness %q %v", line, err)
		}
	}
	unlock()
	unlock = nil
	wins, conflicts := 0, 0
	for _, cmd := range commands {
		err := cmd.Wait()
		if err == nil {
			wins++
			continue
		}
		var status *exec.ExitError
		if errors.As(err, &status) && status.ExitCode() == 10 {
			conflicts++
			continue
		}
		t.Fatal(err)
	}
	if wins != 1 || conflicts != 3 {
		t.Fatalf("wins=%d conflicts=%d", wins, conflicts)
	}
}
