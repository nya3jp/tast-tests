// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package adb enables controlling android devices through the local adb server.
package adb

import (
	"context"
	"os"
	"path/filepath"

	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/sys/unix"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	adbHome = "/tmp/adb_home"
)

// Command creates an ADB command with appropriate environment variables.
func Command(ctx context.Context, args ...string) *testexec.Cmd {
	cmd := testexec.CommandContext(ctx, "adb", args...)
	cmd.Env = append(
		os.Environ(),
		"ADB_VENDOR_KEYS="+vendorKeyPath(),
		// adb expects $HOME to be writable.
		"HOME="+adbHome)
	return cmd
}

// KillADBLocalServer kills the existing ADB local server if it is running.
//
// We do not use adb kill-server since it is unreliable (crbug.com/855325).
// We do not use killall since it can wait for orphan adb processes indefinitely (b/137797801).
func KillADBLocalServer(ctx context.Context) error {
	ps, err := process.Processes()
	if err != nil {
		return err
	}

	for _, p := range ps {
		if name, err := p.Name(); err != nil || name != "adb" {
			continue
		}
		if ppid, err := p.Ppid(); err != nil || ppid != 1 {
			continue
		}

		if err := unix.Kill(int(p.Pid), unix.SIGKILL); err != nil {
			// In a very rare race condition, the server process might be already gone.
			// Just log the error rather than reporting it to the caller.
			testing.ContextLog(ctx, "Failed to kill ADB local server process: ", err)
			continue
		}

		// Wait for the process to exit for sure.
		// This may take as long as 10 seconds due to busy init process.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			// We need a fresh process.Process since it caches attributes.
			if _, err := process.NewProcess(p.Pid); err == nil {
				return errors.Errorf("pid %d is still running", p.Pid)
			}
			return nil
		}, nil); err != nil {
			return errors.Wrap(err, "failed on waiting for ADB local server process to exit")
		}
	}
	return nil
}

const apkPathPrefix = "/usr/local/libexec/tast/apks/local/cros"

// APKPath returns the absolute path to a helper APK.
func APKPath(value string) string {
	return filepath.Join(apkPathPrefix, value)
}
