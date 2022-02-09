// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package removablemedia implements the testing sceanrio of arc.RemovableMedia
// test and its utilities.
package removablemedia

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/storage"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crosdisks"
	"chromiumos/tast/testing"
)

// CreateZeroFile creates a file filled with size bytes of 0.
func CreateZeroFile(size int64, name string) (string, error) {
	f, err := ioutil.TempFile("", name)
	if err != nil {
		return "", errors.Wrap(err, "failed to create an image file")
	}
	defer f.Close()

	if err := f.Truncate(size); err != nil {
		os.Remove(f.Name()) // Ignore error.
		return "", errors.Wrap(err, "failed to resize the image file")
	}

	return f.Name(), nil
}

// AttachLoopDevice attaches to loop device.
func AttachLoopDevice(ctx context.Context, path string) (string, error) {
	b, err := testexec.CommandContext(ctx, "losetup", "-f", path, "--show").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to attach to loop device")
	}
	return strings.TrimSpace(string(b)), nil
}

// DetachLoopDevice detaches from loop device.
func DetachLoopDevice(ctx context.Context, devLoop string) error {
	if err := testexec.CommandContext(ctx, "losetup", "-d", devLoop).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to detach from loop device at %s", devLoop)
	}

	return nil
}

// FormatVFAT formats the vfat file system.
func FormatVFAT(ctx context.Context, devLoop string) error {
	if err := testexec.CommandContext(ctx, "mkfs", "-t", "vfat", "-F", "32", "-n", "MyDisk", devLoop).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to format vfat file system")
	}
	return nil
}

// Mount adds device to allowlist and mounts it.
func Mount(ctx context.Context, cd *crosdisks.CrosDisks, devLoop, name string) (mountPath string, retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second)
	defer cancel()
	sysPath := path.Join("/sys/devices/virtual/block", path.Base(devLoop))
	if err := cd.AddDeviceToAllowlist(ctx, sysPath); err != nil {
		return "", errors.Wrapf(err, "failed to add device %s to allowlist", sysPath)
	}
	defer func() {
		if err := cd.RemoveDeviceFromAllowlist(cleanupCtx, sysPath); err != nil && retErr != nil {
			retErr = errors.Wrapf(err, "failed to remove device %s from allowlist", sysPath)
		}
	}()
	w, err := cd.WatchMountCompleted(ctx)
	if err != nil {
		return "", err
	}
	defer w.Close(ctx)

	if err := cd.Mount(ctx, devLoop, "", []string{"rw", "nodev", "noexec", "nosuid", "sync"}); err != nil {
		return "", errors.Wrap(err, "failed to mount")
	}

	path := filepath.Join("/media/removable", name)
	for {
		m, err := w.Wait(ctx)
		if err != nil {
			return "", errors.Wrap(err, "failed to mount")
		}
		if m.SourcePath == devLoop && m.MountPath == path {
			// Target mount point is found.
			if m.Status != crosdisks.MountErrorNone {
				return "", errors.Errorf("failed to mount with status %d", m.Status)
			}
			return path, nil
		}
	}
}

// Unmount unmounts the disk.
func Unmount(ctx context.Context, cd *crosdisks.CrosDisks, devLoop string) error {
	if status, err := cd.Unmount(ctx, devLoop, []string{"lazy"}); err != nil {
		return errors.Wrap(err, "failed to unmount")
	} else if status != crosdisks.MountErrorNone {
		return errors.Errorf("failed to unmount with status %d", status)
	}
	return nil
}

// RunTest executes the testing scenario of arc.RemovableMedia.
func RunTest(ctx context.Context, s *testing.State, a *arc.ARC, cr *chrome.Chrome, d *ui.Device, testFile string) {
	const (
		imageSize = 64 * 1024 * 1024
		diskName  = "MyDisk"
	)

	// Set up a filesystem image.
	image, err := CreateZeroFile(imageSize, "vfat.img")
	if err != nil {
		s.Fatal("Failed to create image: ", err)
	}
	defer os.Remove(image)

	devLoop, err := AttachLoopDevice(ctx, image)
	if err != nil {
		s.Fatal("Failed to attach loop device: ", err)
	}
	defer func() {
		if err := DetachLoopDevice(ctx, devLoop); err != nil {
			s.Error("Failed to detach from loop device: ", err)
		}
	}()
	if err := FormatVFAT(ctx, devLoop); err != nil {
		s.Fatal("Failed to format VFAT file system: ", err)
	}

	// Mount the image via CrosDisks.
	cd, err := crosdisks.New(ctx)
	if err != nil {
		s.Fatal("Failed to find crosdisks D-Bus service: ", err)
	}
	mountDir, err := Mount(ctx, cd, devLoop, diskName)
	if err != nil {
		s.Fatal("Failed to mount file system: ", err)
	}
	defer func() {
		if err := Unmount(ctx, cd, devLoop); err != nil {
			s.Error("Failed to unmount VFAT image: ", err)
		}
	}()

	if err := arc.WaitForARCRemovableMediaVolumeMount(ctx, a); err != nil {
		s.Fatal("Failed to wait for the volume to be mounted in ARC: ", err)
	}

	if err := storage.TestVolumeSharing(ctx, a, cr, d, mountDir, diskName, arc.RemovableMediaUUID, testFile, s.DataPath(testFile)); err != nil {
		s.Fatal("Failed to verify removable media volume sharing: ", err)
	}
}
