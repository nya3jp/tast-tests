// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package shill provides D-Bus wrappers and utilities for shill service.
package shill

import (
	"context"
	"fmt"
	"os"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
)

const (
	dbusService   = "org.chromium.flimflam"
	startLockPath = "/run/lock/shill-start.lock"
)

// acquireStartLock acquires the start lock of shill. Holding the lock prevents recover_duts from
// restarting shill (crbug.com/473976#c9).
func acquireStartLock() error {
	// We assume that there is no concurrent process trying to create/delete the lock file,
	// but check them just in case. This must not happen.
	if _, err := os.Stat(startLockPath); err == nil {
		p, _ := os.Readlink(startLockPath)
		return errors.Errorf("shill start lock is held by another process: %s", p)
	}

	// Remove an obsolete lock file if it exists.
	os.Remove(startLockPath)

	// Create the lock file. We set the link destination to our proc entry so that the lock is
	// automatically released even if the process crashes.
	if err := os.Symlink(fmt.Sprintf("/proc/%d", os.Getpid()), startLockPath); err != nil {
		return errors.Wrap(err, "failed creating a lock file")
	}
	return nil
}

// releaseStartLock releases the start lock of shill.
func releaseStartLock() error {
	if err := os.Remove(startLockPath); err != nil {
		return errors.Wrap(err, "failed deleting a lock file")
	}
	return nil
}

// SafeStop stops the shill service temporarily.
// This function does not only call upstart.StopJob, but also ensures shill is not started by
// recover_duts (crbug.com/473976#c9). Remember to call SafeStart once you are done.
func SafeStop(ctx context.Context) error {
	if err := acquireStartLock(); err != nil {
		return err
	}
	return upstart.StopJob(ctx, "shill")
}

// SafeStart starts the shill service.
func SafeStart(ctx context.Context) error {
	defer releaseStartLock()
	return upstart.RestartJob(ctx, "shill")
}

// call is a wrapper of dbus.BusObject.CallWithContext.
func call(ctx context.Context, obj dbus.BusObject, dbusInterface, method string, args ...interface{}) *dbus.Call {
	return obj.CallWithContext(ctx, dbusInterface+"."+method, 0, args...)
}
