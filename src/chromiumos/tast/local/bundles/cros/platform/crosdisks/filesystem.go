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

func formatDevice(ctx context.Context, formatCmd, device string) error {
	cmd := strings.Split(formatCmd, " ")
	args := append(cmd, device)[1:]
	command := cmd[0]
	if err := testexec.CommandContext(ctx, command, args...).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "could not format device %s with %q", device, command)
	}
	return nil
}

func withLoopbackDeviceDo(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks, formatCmd string, f func(ctx context.Context, ld *crosdisks.LoopbackDevice)) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second*10)
	defer cancel()

	ld, err := crosdisks.CreateLoopbackDevice(ctx, loopbackSizeBytes)
	if err != nil {
		s.Error("Failed to create loopback device: ", err)
		return
	}
	defer func() {
		s.Logf("Detaching the loopback device %q", ld.GetDevicePath())
		if err := ld.Close(ctxForCleanUp); err != nil {
			s.Errorf("Failed to destroy loopback device %q: %v", ld.GetDevicePath(), err)
		}
	}()
	s.Logf("Created loopback device %q", ld.GetDevicePath())

	err = cd.AddDeviceToAllowlist(ctx, ld.GetSysDevicePath())
	if err != nil {
		s.Errorf("Failed to allowlist the loopback device %q: %v", ld.GetSysDevicePath(), err)
		return
	}
	defer cd.RemoveDeviceFromAllowlist(ctx, ld.GetSysDevicePath())

	if formatCmd != "" {
		s.Logf("Formatting %q with %q", ld.GetDevicePath(), formatCmd)
		if err := formatDevice(ctx, formatCmd, ld.GetDevicePath()); err != nil {
			s.Errorf("Failed to format the loopback device %q with %q: %v", ld.GetDevicePath(), formatCmd, err)
			return
		}
	}

	f(ctx, ld)
}

func doMount(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks, source, fsType, options string) (m crosdisks.MountCompleted, err error) {
	s.Logf("Mounting %q as %q with options %q", source, fsType, options)
	m, err = cd.MountAndWaitForCompletion(ctx, source, fsType, strings.Split(options, ","))
	if err != nil {
		err = errors.Wrap(err, "failed to invoke mount")
		return
	}
	s.Logf("Mount completed with status %d", m.Status)
	if m.SourcePath != source {
		err = errors.Errorf("unexpected source_path: got %q; want %q", m.SourcePath, source)
	}
	return
}

func testMountFilesystem(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks, ld *crosdisks.LoopbackDevice, label string) {
	ctxForUnmount := ctx
	ctx, unmount := ctxutil.Shorten(ctx, time.Second*5)
	defer unmount()

	m, err := doMount(ctx, s, cd, ld.GetDevicePath(), "", "rw")
	if err != nil {
		s.Error("Failed to mount: ", err)
		return
	}

	if m.Status != 0 {
		s.Errorf("Unexpected mount status: got %d; want %d", m.Status, 0)
		return
	}
	defer func() {
		status, err := cd.Unmount(ctxForUnmount, m.MountPath, []string{})
		if err != nil {
			s.Errorf("Could not invoke unmount %q: %v", m.MountPath, err)
			return
		}
		if status != 0 {
			s.Errorf("Failed to unmount %q: status %d", m.MountPath, status)
		}
	}()

	mountPath := path.Join("/media/removable", label)
	if m.MountPath != mountPath {
		s.Errorf("Unexpected mount_path: got %q; want %q", m.MountPath, mountPath)
		return
	}

	// Test writes.
	dir := path.Join(m.MountPath, "mydir")
	if err := os.Mkdir(dir, 0777); err != nil {
		s.Errorf("Failed to create a test directory %q: %v", dir, err)
		return
	}
	file := path.Join(dir, "test.txt")
	if err := ioutil.WriteFile(file, []byte("some text\n"), 0666); err != nil {
		s.Errorf("Failed to write a test file in %q: %v", file, err)
		return
	}
}

// RunFilesystemTests runs filesystem tests.
func RunFilesystemTests(ctx context.Context, s *testing.State) {
	ctxForDisconnect := ctx
	ctx, disconnect := ctxutil.Shorten(ctx, time.Second)
	defer disconnect()
	cd, err := crosdisks.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect CrosDisks D-Bus service: ", err)
	}
	defer func() {
		if err := cd.Close(ctxForDisconnect); err != nil {
			s.Error("Failed to disconnect from CrosDisks D-Bus service: ", err)
		}
	}()

	withLoopbackDeviceDo(ctx, s, cd, "", func(ctx context.Context, ld *crosdisks.LoopbackDevice) {
		// Ideally we should run also some failure tests, e.g. unknown/no filesystem, etc, but cros-disks
		// is too fragile and remains in a half-broken state after that, so we only check known good scenarios.
		s.Run(ctx, "vfat", func(ctx context.Context, state *testing.State) {
			if err := formatDevice(ctx, "mkfs.vfat -n EMPTY1", ld.GetDevicePath()); err != nil {
				s.Error("Could not format device: ", err)
				return
			}
			testMountFilesystem(ctx, s, cd, ld, "EMPTY1")
		})
		s.Run(ctx, "exfat", func(ctx context.Context, state *testing.State) {
			if err := formatDevice(ctx, "mkfs.exfat -n EMPTY2", ld.GetDevicePath()); err != nil {
				s.Error("Could not format device: ", err)
				return
			}
			testMountFilesystem(ctx, state, cd, ld, "EMPTY2")
		})
		s.Run(ctx, "ntfs", func(ctx context.Context, state *testing.State) {
			if err := formatDevice(ctx, "mkfs.ntfs -f -L EMPTY3", ld.GetDevicePath()); err != nil {
				s.Error("Could not format device: ", err)
				return
			}
			testMountFilesystem(ctx, state, cd, ld, "EMPTY3")
		})
	})
}
