// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crosdisks

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"syscall"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/testutil"
)

type filesystemTest struct {
	description       string
	blockSize         int64
	blockCount        int64
	filesystemType    string
	mkfsOptions       []string
	mountOptions      []string
	expectedMountPath string
	isWriteTest       bool
	testTimezone      string
}

var testFsContents = map[string]string{
	"file1":           "0123456789",
	"dir1/file1":      "",
	"dir1/file2":      "abcdefg",
	"dir1/dir2/file3": "abcdefg",
	"dir1/dir2/file4": strings.Repeat("a", 65536),
}

var vfatTests = []filesystemTest{
	{
		description:       "VFAT filesystem with label (read)",
		blockSize:         1024,
		blockCount:        65536,
		filesystemType:    "vfat",
		mkfsOptions:       []string{"-F", "32", "-n", "MYDISK"},
		mountOptions:      []string{"ro", "nodev", "noexec", "nosuid", "sync"},
		expectedMountPath: "/media/removable/MYDISK",
	},
	{
		description:       "VFAT filesystem with UUID (read)",
		blockSize:         1024,
		blockCount:        65536,
		filesystemType:    "vfat",
		mkfsOptions:       []string{"-F", "32", "-i", "deadbeef"},
		mountOptions:      []string{"ro", "nodev", "noexec", "nosuid", "sync"},
		expectedMountPath: "/media/removable/External Drive",
	},
	{
		description:       "VFAT filesystem with timezone > UTC+12",
		blockSize:         1024,
		blockCount:        65536,
		filesystemType:    "vfat",
		mkfsOptions:       []string{"-F", "32", "-n", "MYDISK"},
		mountOptions:      []string{"ro", "nodev", "noexec", "nosuid", "sync"},
		expectedMountPath: "/media/removable/MYDISK",
		testTimezone:      "Pacific/Kiritimati",
	},
	{
		description:       "VFAT filesystem (write)",
		blockSize:         1024,
		blockCount:        65536,
		filesystemType:    "vfat",
		mkfsOptions:       []string{"-F", "32", "-n", "MYDISK"},
		mountOptions:      []string{"rw", "nodev", "noexec", "nosuid", "sync"},
		expectedMountPath: "/media/removable/MYDISK",
		isWriteTest:       true,
	},
}

func restartCrosDisks(ctx context.Context, s *testing.State, timezone string) {
	args := []string{}
	if timezone != "" {
		args = append(args, fmt.Sprintf("TZ=:%s", timezone))
	}
	err := upstart.RestartJob(ctx, "cros-disks", args...)
	if err != nil {
		s.Error("Failed to restart cros-disks: ", err)
	}
}

func mountInfoWithSource(mountSource string) (sysutil.MountInfo, error) {
	infos, err := sysutil.MountInfoForPID(sysutil.SelfPID)
	if err != nil {
		return sysutil.MountInfo{}, err
	}

	for _, mi := range infos {
		if mi.MountSource == mountSource {
			return mi, nil
		}
	}
	return sysutil.MountInfo{}, errors.Errorf("No mount point found with source %s", mountSource)
}

func switchUser(s *testing.State) func() {
	oldEuid := syscall.Geteuid()
	err := syscall.Setresuid(int(sysutil.ChronosUID), int(sysutil.ChronosUID), oldEuid)
	if err != nil {
		s.Fatal("Error switching to chronos user: ", err)
	}
	return func() {
		err := syscall.Setresuid(oldEuid, oldEuid, oldEuid)
		if err != nil {
			s.Fatal("Error restoring original user: ", err)
		}
	}
}

func verifyFileAccessWithUser(ctx context.Context, s *testing.State, t *filesystemTest, mountPath string) {
	restoreFunc := switchUser(s)
	defer restoreFunc()

	if t.isWriteTest {
		if err := testutil.WriteFiles(mountPath, testFsContents); err != nil {
			s.Error("Failed to write cros-disks mounted filesystem contents: ", err)
			return
		}
	}
	readFiles, err := testutil.ReadFiles(mountPath)
	if err != nil {
		s.Error("Failed to read cros-disks mounted filesystem contents: ", err)
		return
	}
	if !reflect.DeepEqual(testFsContents, readFiles) {
		s.Error("Unexpected FS contents: got: %v, want: %v", readFiles, testFsContents)
	}
}

func testFilesystem(ctx context.Context, s *testing.State, cd *crosDisks, t *filesystemTest) {
	i := filesystemImage{
		BlockSize:  t.blockSize,
		BlockCount: t.blockCount,
		Type:       t.filesystemType,
	}
	defer i.Cleanup(ctx)

	if err := i.CreateAndFormat(ctx, t.mkfsOptions); err != nil {
		s.Error("Failed to create filesystem image: ", err)
		return
	}
	mountPath, err := i.Mount(ctx)
	if err != nil {
		s.Error("Failed to mount filesystem image for setup: ", err)
		return
	}

	if !t.isWriteTest {
		if err := testutil.WriteFiles(mountPath, testFsContents); err != nil {
			s.Error("Failed to write filesystem image contents: ", err)
			return
		}
	}

	if err := i.Unmount(ctx); err != nil {
		s.Error("Failed to unmount filesystem image for setup: ", err)
		return
	}

	loop, err := i.SetupLoopDevice(ctx)
	if err != nil {
		s.Error("Failed to bind filesystem image to loopback device: ", err)
		return
	}

	w, err := cd.watchMountCompleted(ctx)
	if err != nil {
		s.Error("Failed to start watching MountCompleted: ", err)
		return
	}
	defer w.Close(ctx)

	err = cd.mount(ctx, loop, t.filesystemType, t.mountOptions)
	if err != nil {
		s.Error("Failed to mount filesystem using cros-disks: ", err)
		return
	}

	m, err := w.wait(ctx)
	if err != nil {
		s.Error("Failed to see MountCompleted D-Bus signal: ", err)
		return
	}
	if m.mountPath != t.expectedMountPath {
		s.Errorf("Unexpected mount path: got: %s; want: %s", m.mountPath, t.expectedMountPath)
	}

	mi, err := mountInfoWithSource(loop)
	if err != nil {
		s.Error("Failed to obtain mount info: ", err)
	} else if mi.MountPath != t.expectedMountPath {
		s.Errorf("Unexpected mount path: got: %s; want: %s", mi.MountPath, t.expectedMountPath)
	}

	verifyFileAccessWithUser(ctx, s, t, m.mountPath)

	status, err := cd.unmount(ctx, m.mountPath, nil /* options */)
	if err != nil {
		s.Error("Failed to unmount filesystem using cros-disks: ", err)
		return
	}
	if status != 0 {
		s.Errorf("Unexpected unmount status: got: %d, want: 0", status)
	}

	if err := i.DetachLoopDevice(ctx); err != nil {
		s.Error("Failed to detach filesystem image from loopback device: ", err)
	}
}

func runSingleFilesystemTest(ctx context.Context, s *testing.State, t *filesystemTest) {
	s.Log("Running cros-disk filesystem test: ", t.description)

	if t.testTimezone != "" {
		restartCrosDisks(ctx, s, t.testTimezone)
		defer restartCrosDisks(ctx, s, "")
	}

	cd, err := newCrosDisks(ctx)
	if err != nil {
		s.Error("Failed to connect CrosDisks D-Bus service: ", err)
		return
	}

	testFilesystem(ctx, s, cd, t)
}

// RunFilesystemTests runs a series of filesystem mounting tests.
func RunFilesystemTests(ctx context.Context, s *testing.State) {
	for _, t := range vfatTests {
		runSingleFilesystemTest(ctx, s, &t)
	}
}
