//go:build !linux

package main

import logger "github.com/sirupsen/logrus"

// lockMemory is a no-op on non-Linux platforms, which do not support the same
// mlockall semantics. Unseal keys may be swapped to disk; rely on disabled or
// encrypted swap instead.
func lockMemory() {
	logger.Warn("memory locking (mlock) is not supported on this platform; unseal keys may be swapped to disk")
}

// disableCoreDumps is a no-op on non-Linux platforms.
func disableCoreDumps() {
	logger.Debug("disabling core dumps is not supported on this platform")
}
