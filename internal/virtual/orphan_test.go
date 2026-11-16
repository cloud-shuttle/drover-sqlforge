package virtual

import (
	"io"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatchParentExitsWhenReparented(t *testing.T) {
	var ppid atomic.Int64
	ppid.Store(100)
	exited := make(chan struct{})
	go watchParent(100, func() int { return int(ppid.Load()) }, func() { close(exited) }, time.Millisecond, nil)

	select {
	case <-exited:
		t.Fatal("exited while parent alive")
	case <-time.After(20 * time.Millisecond):
	}
	ppid.Store(1)
	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Fatal("did not exit after parent changed")
	}
}

func TestWatchParentStop(t *testing.T) {
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		watchParent(1, func() int { return 1 }, func() { t.Error("unexpected exit") }, time.Millisecond, stop)
		close(done)
	}()
	close(stop)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watchParent did not stop")
	}
}

type closingRunner struct {
	Runner
	closed int
}

func (c *closingRunner) Close() error { c.closed++; return nil }

var _ io.Closer = (*closingRunner)(nil)

func TestManagerCloseStopsClosableRunner(t *testing.T) {
	r := &closingRunner{}
	if err := NewManager(r, nil).Close(); err != nil {
		t.Fatal(err)
	}
	if r.closed != 1 {
		t.Fatalf("closed = %d, want 1", r.closed)
	}
	// Runners without Close and nil managers are fine.
	if err := NewManager(&ClickHouseRunner{}, nil).Close(); err != nil {
		t.Fatal(err)
	}
	var nilMgr *Manager
	if err := nilMgr.Close(); err != nil {
		t.Fatal(err)
	}
}
