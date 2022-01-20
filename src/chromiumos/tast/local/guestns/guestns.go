// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package guestns contains helpers to enter and leave the guest namespace.
package guestns

import (
	"context"
	"fmt"

	"github.com/shirou/gopsutil/process"
	"golang.org/x/sys/unix"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// getCryptohomeNamespaceMounterPID returns the PID of the
// 'cryptohome-namespace-mounter' process, if found.
func getCryptohomeNamespaceMounterPID() (int, error) {
	const exePath = "/usr/sbin/cryptohome-namespace-mounter"

	all, err := process.Pids()
	if err != nil {
		return -1, err
	}

	for _, pid := range all {
		proc, err := process.NewProcess(pid)
		if err != nil {
			// Assume that the process exited.
			continue
		}
		if exe, err := proc.Exe(); err == nil && exe == exePath {
			return int(pid), nil
		}
	}
	return -1, errors.New("mounter process not found")
}

// EnterGuestNS enters the mount namespace for guest users.
func EnterGuestNS(ctx context.Context) (chromeNsFd int, err error) {
	var nsPath = "/proc/1/ns/mnt"
	if mounterPid, err := getCryptohomeNamespaceMounterPID(); err == nil {
		nsPath = fmt.Sprintf("/proc/%d/ns/mnt", mounterPid)
	}

	// Guest sessions can be mounted in a non-root mount namespace
	// so the test needs to perform checks in that same namespace.
	if chromeNsFd, err = unix.Open(nsPath, unix.O_CLOEXEC, unix.O_RDWR); err != nil {
		return -1, errors.Wrapf(err, "failed to open Chrome mount namespace at %s", nsPath)
	}

	// Ensure we can successfully call setns(2) by first calling unshare(2)
	// which will make this thread's view of mounts distinct from the root's.
	if err = unix.Unshare(unix.CLONE_NEWNS); err != nil {
		return -1, errors.Wrap(err, "failed to unshare mount namespace")
	}

	if err = unix.Setns(chromeNsFd, unix.CLONE_NEWNS); err != nil {
		return -1, errors.Wrapf(err, "failed to enter Chrome mount namespace at %s", nsPath)
	}

	return
}

// ExitGuestNS exits the mount namespace for guest users.
func ExitGuestNS(ctx context.Context, chromeNsFd int) {
	// Switch back to root mount namespace.
	rootNsFd, err := unix.Open("/proc/1/ns/mnt", unix.O_CLOEXEC, unix.O_RDONLY)
	if err != nil {
		testing.ContextLog(ctx, "Opening root mount namespace failed: ", err)
	}

	testing.ContextLog(ctx, "about to set and close rootNsFd: ", rootNsFd)
	if err := unix.Setns(rootNsFd, unix.CLONE_NEWNS); err != nil {
		testing.ContextLog(ctx, "Setting root mount namespace failed: ", err)
	}

	if err := unix.Close(rootNsFd); err != nil {
		testing.ContextLog(ctx, "Closing root mount namespace failed: ", err)
	}

	if err := unix.Close(chromeNsFd); err != nil {
		// return errors.Wrap(err, "failed to ")
		testing.ContextLog(ctx, "Closing chrome mount namespace failed: ", err)
	}
}
