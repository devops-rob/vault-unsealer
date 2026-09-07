//go:build linux

package main

import (
	logger "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// lockMemory locks all of the process's current and future pages into RAM so
// that unseal keys held in memory can never be written to swap.
//
// This is best-effort: mlockall(2) requires CAP_IPC_LOCK (or root) and a
// sufficient RLIMIT_MEMLOCK. If it fails we log a prominent warning rather than
// refusing to start, so the tool still works on hosts that mitigate swap
// exposure another way (disabled or encrypted swap).
func lockMemory() {
	if err := unix.Mlockall(unix.MCL_CURRENT | unix.MCL_FUTURE); err != nil {
		logger.Warnf("could not lock memory (mlockall: %v); unseal keys may be swapped to disk. "+
			"Grant CAP_IPC_LOCK (e.g. `docker run --cap-add IPC_LOCK --ulimit memlock=-1`) and raise "+
			"RLIMIT_MEMLOCK, disable/encrypt swap on the host, or set `disable_mlock = true` to silence "+
			"this warning.", err)
		return
	}
	logger.Debug("locked process memory into RAM (mlockall)")
}

// disableCoreDumps prevents the kernel from writing a core dump if the process
// crashes (a core file could contain unseal keys). PR_SET_DUMPABLE=0 also blocks
// ptrace-based memory reads and tightens /proc/<pid> ownership.
func disableCoreDumps() {
	if err := unix.Setrlimit(unix.RLIMIT_CORE, &unix.Rlimit{Cur: 0, Max: 0}); err != nil {
		logger.Warnf("could not disable core dumps (RLIMIT_CORE): %v", err)
	}
	if err := unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0); err != nil {
		logger.Warnf("could not set PR_SET_DUMPABLE=0: %v", err)
	}
}
