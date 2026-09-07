//go:build linux

package main

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestDisableCoreDumps(t *testing.T) {
	disableCoreDumps()

	var lim unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_CORE, &lim); err != nil {
		t.Fatalf("Getrlimit: %v", err)
	}
	if lim.Cur != 0 {
		t.Fatalf("RLIMIT_CORE cur = %d, want 0", lim.Cur)
	}
}

func TestLockMemoryDoesNotPanic(t *testing.T) {
	// Best-effort: succeeds with CAP_IPC_LOCK, otherwise logs a warning. Either
	// way it must return without panicking.
	lockMemory()
}
