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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/upstart"
)

// getCryptohomeNamespaceMounterPID returns the PID of the 'cryptohome-namespace-mounter' process,
// if found.
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

var nsPath = "/proc/1/ns/mnt"
var rootNsFd int
var chromeNsFd int

// EnterGuestNS calls chrome.New and logs in as a guest user while also entering
// the guest namespace.
func EnterGuestNS(ctx context.Context, opts ...chrome.Option) (*chrome.Chrome, error) {
	opts = append(opts, chrome.GuestLogin())

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to log in")
	}

	if mounterPid, err := getCryptohomeNamespaceMounterPID(); err == nil {
		nsPath = fmt.Sprintf("/proc/%d/ns/mnt", mounterPid)
	}

	// Guest sessions can be mounted in a non-root mount namespace
	// so the test needs to perform checks in that same namespace.
	chromeNsFd, err = unix.Open(nsPath, unix.O_CLOEXEC, unix.O_RDWR)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open Chrome mount namespace at %s", nsPath)
	}

	// Open root mount namespace to be able to switch back to it.
	rootNsFd, err = unix.Open("/proc/1/ns/mnt", unix.O_CLOEXEC, unix.O_RDONLY)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open root mount namespace")
	}

	// Ensure we can successfully call setns(2) by first calling unshare(2)
	// which will make this thread's view of mounts distinct from the root's.
	if err = unix.Unshare(unix.CLONE_NEWNS); err != nil {
		return nil, errors.Wrap(err, "failed to unshare mount namespace")
	}

	if err = unix.Setns(chromeNsFd, unix.CLONE_NEWNS); err != nil {
		return nil, errors.Wrapf(err, "failed to enter Chrome mount namespace at %s", nsPath)
	}

	return cr, err
}

// ExitGuestNS exits both the logged in guest user and the guest namespace
// entered by EnterGuestNS
func ExitGuestNS(ctx context.Context) {
	// ensure we switch back to the original namespace.
	unix.Setns(rootNsFd, unix.CLONE_NEWNS)
	unix.Close(rootNsFd)
	unix.Close(chromeNsFd)

	// chrome.Chrome.Close() will not log the user out.
	upstart.RestartJob(ctx, "ui")
}
