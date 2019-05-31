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
	lockchan := make(chan error) // To notify lock completion to main thread.
	done := make(chan struct{})  // To notify main thread completion to the goroutine.

	succeeded := false
	defer func() {
		if !succeeded {
			close(done) // Notify thread on error/timeout.
		}
	}()

	go func() {
		f, err := os.Create(checkNetworkLockPath)
		if err != nil {
			lockchan <- err
			return
		}
		defer f.Close()

		if err = unix.Flock(int(f.Fd()), unix.LOCK_SH); err != nil {
			lockchan <- err
			return
		}
		defer unix.Flock(int(f.Fd()), unix.LOCK_UN)
		lockchan <- nil
		<-done // Wait for unlock().
	}()

	lctx, cancel := context.WithTimeout(ctx, checkNetworkLockTimeout)
	defer cancel()
	select {
	case err := <-lockchan:
		if err != nil {
			return nil, errors.Wrapf(err, "failed to acquire lock %s", checkNetworkLockPath)
		}
	case <-lctx.Done():
		return nil, errors.Wrapf(lctx.Err(), "timed out acquiring lock %s", checkNetworkLockPath)
	}

	succeeded = true
	return func() {
		close(done)
	}, nil
}
