// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crosdisks provides a series of tests to verify CrosDisks' D-Bus API
// behavior.
package crosdisks

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosdisks"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

// formatDevice is a convenience function to invoke provided command to create
// a filesystem on the device.
func formatDevice(ctx context.Context, formatCmd, device string) error {
	cmd := strings.Split(formatCmd, " ")
	args := append(cmd, device)[1:]
	command := cmd[0]
	if err := testexec.CommandContext(ctx, command, args...).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "could not format device %s with %q", device, command)
	}
	return nil
}

// WithLoopbackDeviceDo initializes a loopback device (optionally formatting it)
// and calls the provided function enclosed within the scope of validity of the
// loopback device.
func WithLoopbackDeviceDo(ctx context.Context, cd *crosdisks.CrosDisks, sizeBytes int64, formatCmd string, f func(ctx context.Context, ld *crosdisks.LoopbackDevice) error) (err error) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second*10)
	defer cancel()

	ld, err := crosdisks.CreateLoopbackDevice(ctx, sizeBytes)
	if err != nil {
		return errors.Wrap(err, "failed to create loopback device")
	}
	defer func() {
		testing.ContextLogf(ctxForCleanUp, "Detaching the loopback device %q", ld.DevicePath())
		if e := ld.Close(ctxForCleanUp); e != nil && err == nil {
			err = errors.Wrapf(e, "failed to destroy loopback device %q", ld.DevicePath())
		}
	}()
	testing.ContextLogf(ctx, "Created loopback device %q", ld.DevicePath())

	if err := cd.AddDeviceToAllowlist(ctx, ld.SysDevicePath()); err != nil {
		return errors.Wrapf(err, "failed to allowlist the loopback device %q", ld.SysDevicePath())
	}
	// We don't really care if this fails.
	defer cd.RemoveDeviceFromAllowlist(ctx, ld.SysDevicePath())

	if formatCmd != "" {
		testing.ContextLogf(ctx, "Formatting %q with %q", ld.DevicePath(), formatCmd)
		if err := formatDevice(ctx, formatCmd, ld.DevicePath()); err != nil {
			return errors.Wrapf(err, "failed to format the loopback device %q with %q", ld.DevicePath(), formatCmd)
		}
	}

	return f(ctx, ld)
}

// testMountFilesystem mounts the loopback device, attempts a write and returns
// an error if unsuccessful.
func testMountFilesystem(ctx context.Context, cd *crosdisks.CrosDisks, ld *crosdisks.LoopbackDevice, label string) error {
	expectedMountPath := filepath.Join("/media/removable", label)
	return WithMountDo(ctx, cd, ld.DevicePath(), "", []string{"rw"}, func(ctx context.Context, mountPath string, readOnly bool) error {
		if expectedMountPath != mountPath {
			return errors.Errorf("unexpected mount_path: got %q; want %q", mountPath, expectedMountPath)
		}

		if readOnly {
			return errors.Errorf("unexpected read-only flag for %q: got %v; want false", mountPath, readOnly)
		}

		// Test writes.
		dir := filepath.Join(mountPath, "mydir")
		if err := execAsUser(ctx, chronos, []string{"mkdir", dir}); err != nil {
			return errors.Wrapf(err, "failed to create a test directory %q as chronos", dir)
		}
		file := filepath.Join(dir, "test.txt")
		if err := execAsUser(ctx, chronos, []string{"touch", file}); err != nil {
			return errors.Wrapf(err, "failed to write a test file in %q as chronos", file)
		}
		// Check that file is actually there.
		st, err := os.Stat(file)
		if err != nil {
			return errors.Wrapf(err, "failed to stat a test file %q", file)
		}
		stat, _ := st.Sys().(*syscall.Stat_t)
		if stat.Uid != sysutil.ChronosUID {
			return errors.Errorf("wrong owner of the file: got %d; want %d", stat.Uid, sysutil.ChronosUID)
		}
		return nil
	})
}

const loopbackSizeBytes = 16 * 1024 * 1024

// RunFilesystemTests executes a set of tests which mount different filesystems
// using CrosDisks.
func RunFilesystemTests(ctx context.Context, s *testing.State) {
	cd, err := crosdisks.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect CrosDisks D-Bus service: ", err)
	}
	defer cd.Close()

	err = WithLoopbackDeviceDo(ctx, cd, loopbackSizeBytes, "", func(ctx context.Context, ld *crosdisks.LoopbackDevice) error {
		// Ideally we should run also some failure tests, e.g. unknown/no
		// filesystem, etc, but cros-disks is too fragile and remains in a
		// half-broken state after that, so we only check known good scenarios.
		s.Run(ctx, "vfat", func(ctx context.Context, state *testing.State) {
			if err := formatDevice(ctx, "mkfs.vfat -n EMPTY1", ld.DevicePath()); err != nil {
				state.Fatal("Could not format device: ", err)
			}
			if err := testMountFilesystem(ctx, cd, ld, "EMPTY1"); err != nil {
				state.Fatal("Test case failed: ", err)
			}
		})
		s.Run(ctx, "exfat", func(ctx context.Context, state *testing.State) {
			if err := formatDevice(ctx, "mkfs.exfat -n EMPTY2", ld.DevicePath()); err != nil {
				state.Fatal("Could not format device: ", err)
			}
			if err := testMountFilesystem(ctx, cd, ld, "EMPTY2"); err != nil {
				state.Fatal("Test case failed: ", err)
			}
		})
		s.Run(ctx, "ntfs", func(ctx context.Context, state *testing.State) {
			if err := formatDevice(ctx, "mkfs.ntfs -f -L EMPTY3", ld.DevicePath()); err != nil {
				state.Fatal("Could not format device: ", err)
			}
			if err := testMountFilesystem(ctx, cd, ld, "EMPTY3"); err != nil {
				state.Fatal("Test case failed: ", err)
			}
		})
		return nil
	})
	if err != nil {
		s.Fatal("Failed to initialize loopback device: ", err)
	}
}
