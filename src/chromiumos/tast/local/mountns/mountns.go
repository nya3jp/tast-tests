// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mountns contains helpers to enter and leave mount namespaces.
package mountns

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

// EnterUserSessionMountNs enters the user session mount namespace.
func EnterUserSessionMountNs(ctx context.Context) error {
	var nsPath = "/proc/1/ns/mnt"
	if mounterPid, err := getCryptohomeNamespaceMounterPID(); err == nil {
		nsPath = fmt.Sprintf("/proc/%d/ns/mnt", mounterPid)
	}

	userSessionNsFd, err := unix.Open(nsPath, unix.O_CLOEXEC, unix.O_RDWR)
	if err != nil {
		return errors.Wrapf(err, "failed to open user session mount namespace at %s", nsPath)
	}

	if err := unix.Unshare(unix.CLONE_NEWNS); err != nil {
		return errors.Wrap(err, "failed to unshare mount namespace")
	}

	if err := unix.Setns(userSessionNsFd, unix.CLONE_NEWNS); err != nil {
		return errors.Wrap(err, "failed to enter user session mount namespace")
	}

	if err := unix.Close(userSessionNsFd); err != nil {
		return errors.Wrap(err, "failed to close user session mount namespace")
	}

	return nil
}

// EnterInitMountNs enters the init mount namespace.
func EnterInitMountNs(ctx context.Context) {
	initNsFd, err := unix.Open("/proc/1/ns/mnt", unix.O_CLOEXEC, unix.O_RDONLY)
	if err != nil {
		testing.ContextLog(ctx, "Opening init mount namespace failed: ", err)
	}

	if err := unix.Setns(initNsFd, unix.CLONE_NEWNS); err != nil {
		testing.ContextLog(ctx, "Setting init mount namespace failed: ", err)
	}

	if err := unix.Close(initNsFd); err != nil {
		testing.ContextLog(ctx, "Closing init mount namespace failed: ", err)
	}
}
