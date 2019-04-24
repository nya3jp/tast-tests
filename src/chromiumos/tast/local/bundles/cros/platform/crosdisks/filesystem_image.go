// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crosdisks

import (
	"context"
	"io/ioutil"
	"os"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

type filesystemImage struct {
	BlockSize  int64
	BlockCount int64
	Type       string
	MountType  string

	imagePath  string
	mountPath  string
	loopDevice string
}

func (i *filesystemImage) Cleanup(ctx context.Context) {
	if i.mountPath != "" {
		i.Unmount(ctx)
	}

	if i.loopDevice != "" {
		i.DetachLoopDevice(ctx)
	}

	if i.imagePath != "" {
		os.Remove(i.imagePath)
	}
}

func (i *filesystemImage) CreateAndFormat(ctx context.Context, mkfsOptions []string) error {
	if i.imagePath != "" {
		return errors.Errorf("fs image already exists at %s", i.imagePath)
	}

	f, err := ioutil.TempFile("", "fs_image")
	if err != nil {
		return errors.Wrap(err, "failed to created fs image file")
	}
	defer f.Close()

	if err := f.Truncate(i.BlockSize * i.BlockCount); err != nil {
		return errors.Wrap(err, "failed to resize fs image file")
	}

	i.imagePath = f.Name()
	f.Close()

	cmdLine := []string{"-t", i.Type}
	cmdLine = append(cmdLine, mkfsOptions...)
	cmdLine = append(cmdLine, i.imagePath)
	cmd := testexec.CommandContext(ctx, "mkfs", cmdLine...)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to run mkfs")
	}

	return nil
}

func (i *filesystemImage) Mount(ctx context.Context) (string, error) {
	mountDir, err := ioutil.TempDir("", "fs_image_mount")
	if err != nil {
		return "", errors.Wrap(err, "failed to mount fs image")
	}

	mountType := i.MountType
	if mountType == "" {
		mountType = i.Type
	}
	cmdLine := []string{"-t", mountType, i.imagePath, mountDir}
	cmd := testexec.CommandContext(ctx, "mount", cmdLine...)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return "", errors.Wrap(err, "failed to mount fs image")
	}

	i.mountPath = mountDir
	return mountDir, nil
}

func (i *filesystemImage) Unmount(ctx context.Context) error {
	cmd := testexec.CommandContext(ctx, "umount", i.mountPath)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed in unmount fs image")
	}
	i.mountPath = ""
	return nil
}

func (i *filesystemImage) SetupLoopDevice(ctx context.Context) (string, error) {
	cmd := testexec.CommandContext(ctx, "losetup", "-f", i.imagePath)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return "", errors.Wrap(err, "failed to bind fs image to loopback device")
	}

	cmd = testexec.CommandContext(ctx, "losetup", "-j", i.imagePath)
	output, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to find loopback device")
	}

	// output should look like: "/dev/loop0: [000d]:6329 (/tmp/test.img)"
	parts := strings.Split(string(output), ":")
	if len(parts) < 2 {
		return "", errors.Errorf("Invalid losetup output: %s", string(output))
	}

	i.loopDevice = parts[0]
	return i.loopDevice, nil
}

func (i *filesystemImage) DetachLoopDevice(ctx context.Context) error {
	cmd := testexec.CommandContext(ctx, "losetup", "-d", i.loopDevice)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to detach loopback device")
	}

	i.loopDevice = ""
	return nil
}
