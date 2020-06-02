// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"fmt"

	"github.com/shirou/gopsutil/process"
	"golang.org/x/sys/unix"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/security/userfiles"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: UserFilesGuest,
		Desc: "Checks ownership and permissions of files for guest users",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"chromeos-security@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
	})
}

// getCryptohomeNamespaceMounterPID returns the PID of the 'cryptohome-namespace-mounter' process,
// if found.
func getCryptohomeNamespaceMounterPID() (int, error) {
	const exePath = "/usr/sbin/cryptohome-namespace-mounter"

	all, err := process.Pids()
	if err != nil {
		return -1, err
	}

	for _, pid := range all {
		if proc, err := process.NewProcess(pid); err != nil {
			// Assume that the process exited.
			continue
		} else if exe, err := proc.Exe(); err == nil && exe == exePath {
			return int(pid), nil
		}
	}
	return -1, errors.New("mounter process not found")
}

func UserFilesGuest(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.GuestLogin())
	if err != nil {
		s.Fatal("Login failed: ", err)
	}
	// chrome.Chrome.Close() will not log the user out.
	defer upstart.RestartJob(ctx, "ui")

	nsPath := "/proc/1/ns/mnt"
	if mounterPid, err := getCryptohomeNamespaceMounterPID(); err == nil {
		nsPath = fmt.Sprintf("/proc/%d/ns/mnt", mounterPid)
	}

	// Guest sessions can be mounted in a non-root mount namespace
	// so the test needs to perform checks in that same namespace.
	s.Log("Attempting to open Chrome mount namespace at ", nsPath)
	chromeNsFd, err := unix.Open(nsPath, unix.O_CLOEXEC, unix.O_RDWR)
	if err != nil {
		s.Fatal("Opening Chrome mount namespace failed: ", err)
	}
	defer unix.Close(chromeNsFd)

	// Open root mount namespace to be able to switch back to it.
	rootNsFd, err := unix.Open("/proc/1/ns/mnt", unix.O_CLOEXEC, unix.O_RDONLY)
	if err != nil {
		s.Fatal("Opening root mount namespace failed: ", err)
	}
	defer unix.Close(rootNsFd)

	// Ensure we can successfully call setns(2) by first calling unshare(2)
	// which will make this thread's view of mounts distinct from the root's.
	if err := unix.Unshare(unix.CLONE_NEWNS); err != nil {
		s.Fatal("Unsharing mount namespace failed: ", err)
	}
	// As soon as we've successfully called unshare(2) ensure we switch back
	// to the original namespace.
	defer unix.Setns(rootNsFd, unix.CLONE_NEWNS)

	if err := unix.Setns(chromeNsFd, unix.CLONE_NEWNS); err == nil {
		userfiles.Check(ctx, s, cr.User())
	} else {
		s.Logf("Entering Chrome mount namespace at %s failed: %v", nsPath, err)
	}
}
