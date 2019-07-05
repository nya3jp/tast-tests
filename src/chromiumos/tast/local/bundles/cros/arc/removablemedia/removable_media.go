// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package removablemedia implements the testing sceanrio of arc.RemovableMedia
// test and its utilities.
package removablemedia

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/crosdisks"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// createZeroFile creates a file filled with size bytes of 0.
func createZeroFile(size int64, name string) (string, error) {
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

func attachLoopDevice(ctx context.Context, path string) (string, error) {
	b, err := testexec.CommandContext(ctx, "losetup", "-f", path, "--show").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to attach to loop device")
	}
	return strings.TrimSpace(string(b)), nil
}

func detachLoopDevice(ctx context.Context, devLoop string) error {
	if err := testexec.CommandContext(ctx, "losetup", "-d", devLoop).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to detach from loop device at %s", devLoop)
	}

	return nil
}

func formatVFAT(ctx context.Context, devLoop string) error {
	if err := testexec.CommandContext(ctx, "mkfs", "-t", "vfat", "-F", "32", "-n", "MyDisk", devLoop).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to format vfat file system")
	}
	return nil
}

func mount(ctx context.Context, cd *crosdisks.CrosDisks, devLoop, name string) (string, error) {
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

func unmount(ctx context.Context, cd *crosdisks.CrosDisks, devLoop string) error {
	if status, err := cd.Unmount(ctx, devLoop, []string{"lazy"}); err != nil {
		return errors.Wrap(err, "failed to unmount")
	} else if status != crosdisks.MountErrorNone {
		return errors.Errorf("failed to unmount with status %d", status)
	}
	return nil
}

// RunTest executes the testing scenario of arc.RemovableMedia.
func RunTest(ctx context.Context, s *testing.State, a *arc.ARC, testFile string) {
	const (
		imageSize = 64 * 1024 * 1024
		diskName  = "MyDisk"
	)
	expected, err := ioutil.ReadFile(s.DataPath(testFile))
	if err != nil {
		s.Fatalf("Failed to read %s: %v", testFile, err)
	}

	// Set up a filesystem image.
	image, err := createZeroFile(imageSize, "vfat.img")
	if err != nil {
		s.Fatal("Failed to create image: ", err)
	}
	defer os.Remove(image)

	devLoop, err := attachLoopDevice(ctx, image)
	if err != nil {
		s.Fatal("Failed to attach loop device: ", err)
	}
	defer func() {
		if err := detachLoopDevice(ctx, devLoop); err != nil {
			s.Error("Failed to detach from loop device: ", err)
		}
	}()
	if err := formatVFAT(ctx, devLoop); err != nil {
		s.Fatal("Failed to format VFAT file system: ", err)
	}

	// Mount the image via CrosDisks.
	cd, err := crosdisks.New(ctx)
	if err != nil {
		s.Fatal("Failed to find crosdisks D-Bus service: ", err)
	}
	mountDir, err := mount(ctx, cd, devLoop, diskName)
	if err != nil {
		s.Fatal("Failed to mount file system: ", err)
	}
	defer func() {
		if err := unmount(ctx, cd, devLoop); err != nil {
			s.Error("Failed to unmount VFAT image: ", err)
		}
	}()

	// Create a picture in the removable media.
	tpath := filepath.Join(mountDir, testFile)
	if err := ioutil.WriteFile(tpath, expected, 0644); err != nil {
		s.Fatal("Failed to write a data file: ", err)
	}

	// VolumeProvider should be able to read the file.
	uri := "content://org.chromium.arc.volumeprovider/" + path.Join("removable", diskName, testFile)
	out, err := a.Command(ctx, "content", "read", "--uri", uri).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to read the content: ", err)
	}

	if !bytes.Equal(out, expected) {
		if err := ioutil.WriteFile(filepath.Join(s.OutDir(), testFile), out, 0644); err != nil {
			s.Error("Failed to dump: ", err)
		}
		s.Fatalf("Failed to share the file (see %s for the read file content)", testFile)
	}
}
