// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network provides general CrOS network goodies.
package network

import (
	"context"
	"os"
	"time"

	"golang.org/x/sys/unix"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	checkNetworkLockPath = "/run/autotest_pause_ethernet_hook"
	// An estimate of the longest time 'check_ethernet.hook' needs for its
	// connectivity checks.
	checkNetworkLockTimeout = 20 * time.Second
)

// LockCheckNetworkHook prevents the 'check_ethernet' recovery hook (runs on
// machines in the lab) from interrupting us (e.g., trying to restart shill or
// forcibly reset the Ethernet device). Use this if your test is going to
// perform operations that may interrupt the DUT's network connectivity (e.g.,
// restarting shill; configuring non-standard network profiles; suspending the
// system).
func LockCheckNetworkHook(ctx context.Context) (unlock func(), e error) {
	lockChan := make(chan error, 1) // To notify lock completion to main thread.
	done := make(chan struct{})     // To notify main thread completion to the goroutine.

	doUnlock := func() {
		close(done)
	}

	succeeded := false
	defer func() {
		if !succeeded {
			doUnlock()
		}
	}()

	go func() {
		f, err := os.Create(checkNetworkLockPath)
		if err != nil {
			lockChan <- err
			return
		}
		defer f.Close()

		// NOTE: if this lock is held for a "long time", the main
		// thread may time out but we'll still be stuck here beyond the
		// time of test completion. This is OK, but beware that (for
		// example) cleanup logging may not go anywhere useful.
		if err = unix.Flock(int(f.Fd()), unix.LOCK_SH); err != nil {
			lockChan <- err
			return
		}
		defer func() {
			// Update access and modification time, so
			// check_ethernet.hook knows when we last released the
			// lock.
			if err = unix.Futimes(int(f.Fd()), nil); err != nil {
				testing.ContextLogf(ctx, "Failed to update time %s: %v", checkNetworkLockPath, err)
			}
			if err = unix.Flock(int(f.Fd()), unix.LOCK_UN); err != nil {
				testing.ContextLogf(ctx, "Failed to unlock %s: %v", checkNetworkLockPath, err)
			}
		}()
		lockChan <- nil
		<-done // Wait for main thread.
	}()

	lctx, cancel := context.WithTimeout(ctx, checkNetworkLockTimeout)
	defer cancel()
	select {
	case err := <-lockChan:
		if err != nil {
			return nil, errors.Wrapf(err, "failed to acquire lock %s", checkNetworkLockPath)
		}
	case <-lctx.Done():
		return nil, errors.Wrapf(lctx.Err(), "timed out acquiring lock %s", checkNetworkLockPath)
	}

	succeeded = true
	return doUnlock, nil
}
