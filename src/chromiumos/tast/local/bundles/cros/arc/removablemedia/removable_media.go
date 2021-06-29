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
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/crosdisks"
	"chromiumos/tast/testing"
)

const (
	// Prefix of content URIs used by ArcVolumeProvider.
	contentURIPrefix = "content://org.chromium.arc.volumeprovider/"
	// Fake UUID of removable device for testing.
	// Defined in chromium:components/arc/volume_mounter/arc_volume_mounter_bridge.cc.
	fakeUUID = "00000000000000000000000000000000DEADBEEF"
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

func mount(ctx context.Context, cd *crosdisks.CrosDisks, devLoop, name string) (mountPath string, retErr error) {
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

func unmount(ctx context.Context, cd *crosdisks.CrosDisks, devLoop string) error {
	if status, err := cd.Unmount(ctx, devLoop, []string{"lazy"}); err != nil {
		return errors.Wrap(err, "failed to unmount")
	} else if status != crosdisks.MountErrorNone {
		return errors.Errorf("failed to unmount with status %d", status)
	}
	return nil
}

// waitForARCVolumeMount waits for the volume to be mounted in ARC using the sm
// command. Just checking mountinfo is not sufficient here since it takes some
// time for the FUSE layer in Android R+ to be ready after /storage/<UUID> has
// become a mountpoint.
func waitForARCVolumeMount(ctx context.Context, a *arc.ARC) error {
	// Regular expression that matches the output line for the mounted
	// volume. Each output line of the sm command is of the form:
	// <volume id><space(s)><mount status><space(s)><volume UUID>.
	re := regexp.MustCompile(`^(stub:)?[0-9]+\s+mounted\s+` + fakeUUID + `$`)

	testing.ContextLog(ctx, "Waiting for the volume to be mounted in ARC")

	return testing.Poll(ctx, func(ctx context.Context) error {
		out, err := a.Command(ctx, "sm", "list-volumes").Output(testexec.DumpLogOnError)
		if err != nil {
			return errors.Wrap(err, "sm command failed")
		}
		lines := bytes.Split(out, []byte("\n"))
		for _, line := range lines {
			if re.Find(bytes.TrimSpace(line)) != nil {
				return nil
			}
		}
		return errors.New("the volume is not yet mounted")
	}, &testing.PollOptions{Timeout: 30 * time.Second})
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

	if err := waitForARCVolumeMount(ctx, a); err != nil {
		s.Fatal("Failed to wait for the volume to be mounted in ARC: ", err)
	}

	// Create a picture in the removable media.
	tpath := filepath.Join(mountDir, testFile)
	if err := ioutil.WriteFile(tpath, expected, 0644); err != nil {
		s.Fatal("Failed to write a data file: ", err)
	}

	// VolumeProvider should be able to read the file.
	verify := func(uri, dumpPath string) error {
		out, err := a.Command(ctx, "content", "read", "--uri", uri).Output(testexec.DumpLogOnError)
		if err != nil {
			return errors.Wrap(err, "failed to read the content")
		}

		if !bytes.Equal(out, expected) {
			if err := ioutil.WriteFile(dumpPath, out, 0644); err != nil {
				s.Logf("Failed to dump the read content to %s: %v", dumpPath, err)
				return errors.New("file content does not match with the original")
			}
			return errors.Errorf("file content does not match with the original (see %s for the read content)", dumpPath)
		}
		return nil
	}

	uri := contentURIPrefix + path.Join(fakeUUID, testFile)
	if err := verify(uri, filepath.Join(s.OutDir(), testFile)); err != nil {
		s.Fatalf("Failed to read the file via VolumeProvider using content URI %s: %v", uri, err)
	}
}
