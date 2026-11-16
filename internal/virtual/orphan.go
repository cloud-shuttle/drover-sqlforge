package virtual

import (
	"os"
	"sync"
	"time"
)

var orphanWatch sync.Once

// orphanPollInterval is how often a plugin checks that its host is alive.
var orphanPollInterval = time.Second

// exitWhenOrphaned makes a plugin process exit once its host process is gone.
// The host normally stops plugins (Managed clients + Runtime.Close), but a
// host that calls os.Exit, crashes or is killed never gets the chance, and
// the orphaned plugin would keep warehouse connections and DuckDB file locks.
// When the parent dies the plugin is re-parented, so its parent PID changes.
func exitWhenOrphaned() {
	orphanWatch.Do(func() {
		parent := os.Getppid()
		go watchParent(parent, os.Getppid, func() { os.Exit(0) }, orphanPollInterval, nil)
	})
}

// watchParent polls getppid and calls exit when it no longer returns parent.
// stop ends the loop early (tests); nil means run forever.
func watchParent(parent int, getppid func() int, exit func(), every time.Duration, stop <-chan struct{}) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			if getppid() != parent {
				exit()
				return
			}
		}
	}
}
