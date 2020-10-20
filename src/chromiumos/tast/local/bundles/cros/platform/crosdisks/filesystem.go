// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crosdisks provides a series of tests to verify CrosDisks'
// D-Bus API behavior.
package crosdisks

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosdisks"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const loopbackSizeBytes = 16 * 1024 * 1024

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

// withLoopbackDeviceDo initializes a loopback device (optionally formatting it) and calls
// the provided function enclosed within the scope of validity of the loopback device.
func withLoopbackDeviceDo(ctx context.Context, cd *crosdisks.CrosDisks, formatCmd string, f func(ctx context.Context, ld *crosdisks.LoopbackDevice) error) (err error) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second*10)
	defer cancel()

	ld, err := crosdisks.CreateLoopbackDevice(ctx, loopbackSizeBytes)
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
			return errors.Wrapf(err, "failed to format the loopback device %q with %q: %v", ld.DevicePath(), formatCmd, err)
		}
	}

	return f(ctx, ld)
}

func doMount(ctx context.Context, cd *crosdisks.CrosDisks, source, fsType, options string) (m crosdisks.MountCompleted, err error) {
	testing.ContextLogf(ctx, "Mounting %q as %q with options %q", source, fsType, options)
	m, err = cd.MountAndWaitForCompletion(ctx, source, fsType, strings.Split(options, ","))
	if err != nil {
		err = errors.Wrap(err, "failed to invoke mount")
		return
	}
	testing.ContextLogf(ctx, "Mount completed with status %d", m.Status)
	if m.SourcePath != source {
		err = errors.Errorf("unexpected source_path: got %q; want %q", m.SourcePath, source)
	}
	return
}

func testMountFilesystem(ctx context.Context, cd *crosdisks.CrosDisks, ld *crosdisks.LoopbackDevice, label string) (err error) {
	ctxForUnmount := ctx
	ctx, unmount := ctxutil.Shorten(ctx, time.Second*5)
	defer unmount()

	m, err := doMount(ctx, cd, ld.DevicePath(), "", "rw")
	if err != nil {
		return err
	}

	if m.Status != 0 {
		return errors.Errorf("unexpected mount status: got %d; want %d", m.Status, 0)
	}
	defer func() {
		status, e := cd.Unmount(ctxForUnmount, m.MountPath, []string{})
		if e != nil {
			testing.ContextLogf(ctxForUnmount, "Could not invoke unmount %q: %v", m.MountPath, e)
			if err == nil {
				err = errors.Wrapf(e, "could not invoke unmount %q", m.MountPath)
			}
			return
		}
		if status != 0 {
			testing.ContextLogf(ctxForUnmount, "Failed to unmount %q: status %d", m.MountPath, status)
			if err == nil {
				err = errors.Wrapf(e, "failed to unmount %q: status %d", m.MountPath, status)
			}
		}
	}()

	mountPath := path.Join("/media/removable", label)
	if m.MountPath != mountPath {
		return errors.Errorf("unexpected mount_path: got %q; want %q", m.MountPath, mountPath)
	}

	// Test writes.
	dir := path.Join(m.MountPath, "mydir")
	if err := os.Mkdir(dir, 0777); err != nil {
		return errors.Wrapf(err, "failed to create a test directory %q", dir)
	}
	file := path.Join(dir, "test.txt")
	if err := ioutil.WriteFile(file, []byte("some text\n"), 0666); err != nil {
		return errors.Wrapf(err, "failed to write a test file in %q", file)
	}
	return
}

// RunFilesystemTests executes a set of tests which mount different filesystems using CrosDisks.
func RunFilesystemTests(ctx context.Context, s *testing.State) {
	cd, err := crosdisks.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect CrosDisks D-Bus service: ", err)
	}

	err = withLoopbackDeviceDo(ctx, cd, "", func(ctx context.Context, ld *crosdisks.LoopbackDevice) error {
		// Ideally we should run also some failure tests, e.g. unknown/no filesystem, etc, but cros-disks
		// is too fragile and remains in a half-broken state after that, so we only check known good scenarios.
		s.Run(ctx, "vfat", func(ctx context.Context, state *testing.State) {
			if err := formatDevice(ctx, "mkfs.vfat -n EMPTY1", ld.DevicePath()); err != nil {
				s.Error("Could not format device: ", err)
				return
			}
			if err := testMountFilesystem(ctx, cd, ld, "EMPTY1"); err != nil {
				s.Error("Test case failed: ", err)
			}
		})
		s.Run(ctx, "exfat", func(ctx context.Context, state *testing.State) {
			if err := formatDevice(ctx, "mkfs.exfat -n EMPTY2", ld.DevicePath()); err != nil {
				s.Error("Could not format device: ", err)
				return
			}
			if err := testMountFilesystem(ctx, cd, ld, "EMPTY2"); err != nil {
				s.Error("Test case failed: ", err)
			}
		})
		s.Run(ctx, "ntfs", func(ctx context.Context, state *testing.State) {
			if err := formatDevice(ctx, "mkfs.ntfs -f -L EMPTY3", ld.DevicePath()); err != nil {
				s.Error("Could not format device: ", err)
				return
			}
			if err := testMountFilesystem(ctx, cd, ld, "EMPTY3"); err != nil {
				s.Error("Test case failed: ", err)
			}
		})
		return nil
	})
	if err != nil {
		s.Fatal("Failed to initialize loopback device: ", err)
	}
}
