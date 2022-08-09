// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mountns contains helpers to enter and leave mount namespaces.
package mountns

import (
	"context"
	"fmt"
	"runtime"

	"golang.org/x/sys/unix"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/procutil"
	"chromiumos/tast/testing"
)

// cryptohomeNamespaceMounterPID returns the PID of the
// 'cryptohome-namespace-mounter' process, if found.
func cryptohomeNamespaceMounterPID() (int, error) {
	const exePath = "/usr/sbin/cryptohome-namespace-mounter"
	p, err := procutil.FindUnique(procutil.ByExe(exePath))
	if err != nil {
		return -1, err
	}
	return int(p.Pid), nil
}

// EnterUserSessionMountNS enters the user session mount namespace.
// Mount namespace is per platform thread, which can be different from go-routine.
// So in order to use this API safely, the current running context needs to be bound
// to the platform thread, and it needs to be kept until it is unset by EnterInitMountNs.
func EnterUserSessionMountNS(ctx context.Context) (retErr error) {
	var nsPath = "/proc/1/ns/mnt"
	if mounterPid, err := cryptohomeNamespaceMounterPID(); err == nil {
		nsPath = fmt.Sprintf("/proc/%d/ns/mnt", mounterPid)
	}

	userSessionNsFd, err := unix.Open(nsPath, unix.O_CLOEXEC, unix.O_RDWR)
	if err != nil {
		return errors.Wrapf(err, "failed to open user session mount namespace at %s", nsPath)
	}
	defer func() {
		if err := unix.Close(userSessionNsFd); err != nil {
			if retErr != nil {
				testing.ContextLog(ctx, "Failed to close user session mount namespace: ", err)
			} else {
				retErr = errors.Wrap(err, "failed to close user session mount namespace")
			}
		}
	}()

	if err := unix.Unshare(unix.CLONE_NEWNS); err != nil {
		return errors.Wrap(err, "failed to unshare mount namespace")
	}

	if err := unix.Setns(userSessionNsFd, unix.CLONE_NEWNS); err != nil {
		return errors.Wrap(err, "failed to enter user session mount namespace")
	}

	return nil
}

// EnterInitMountNS enters the init mount namespace.
// Please see also the thread-related comment for EnterUserSessionMountNS, which
// applies here, too.
func EnterInitMountNS(ctx context.Context) {
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

// WithUserSessionMountNS runs the given test function in the user session mount namespace.
// This function must be called in the init mount namespace.
func WithUserSessionMountNS(ctx context.Context, f func(ctx context.Context) error) error {
	// Bind to the platform thread as mount namespace is effective per thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := EnterUserSessionMountNS(ctx); err != nil {
		return err
	}
	defer EnterInitMountNS(ctx)

	return f(ctx)
}
