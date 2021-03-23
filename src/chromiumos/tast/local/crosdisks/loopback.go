// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crosdisks provides an interface to talk to cros_disks service
// via D-Bus and utilities.
package crosdisks

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

// LoopbackDevice holds info about a created loopback device.
type LoopbackDevice struct {
	file   *os.File
	device string
}

// CreateLoopbackDevice creates a loopback device backed by a file of the specified size.
func CreateLoopbackDevice(ctx context.Context, sizeBytes int64) (*LoopbackDevice, error) {
	file, err := ioutil.TempFile("", "cros-disks-loop-*")
	if err != nil {
		return nil, errors.Wrap(err, "could not create temporary loopback file")
	}
	filename := file.Name()
	needsClose := true
	needsRemove := true
	defer func() {
		if needsClose {
			file.Close()
		}
		if needsRemove {
			os.Remove(filename)
		}
	}()
	if err = file.Truncate(sizeBytes); err != nil {
		return nil, errors.Wrapf(err, "could not set size of the loopback image file %q", filename)
	}
	needsClose = false
	if err = file.Close(); err != nil {
		return nil, errors.Wrapf(err, "could not save the loopback file %q", filename)
	}
	data, err := testexec.CommandContext(ctx, "losetup", "-f", filename, "--show").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrapf(err, "could not attach file %q as a loopback device", filename)
	}
	loopDevice := strings.TrimSpace(string(data))
	needsRemove = false
	return &LoopbackDevice{file: file, device: loopDevice}, nil
}

// Close detaches the loopback device and removes the file backing it.
func (l *LoopbackDevice) Close(ctx context.Context) (err error) {
	// Ignore unmount errors as not necessarily mounted.
	_ = testexec.CommandContext(ctx, "umount", "-f", l.device).Run()
	err = testexec.CommandContext(ctx, "losetup", "-d", l.device).Run(testexec.DumpLogOnError)
	if err != nil {
		err = errors.Wrapf(err, "could not detach loopback device %q", l.device)
		// Still try to remove the backing file.
	}
	if e := os.Remove(l.file.Name()); e != nil && err == nil {
		err = errors.Wrapf(err, "could not remove temporary loopback file %q", l.file.Name())
	}
	return
}

// DevicePath returns the device path in /dev.
func (l *LoopbackDevice) DevicePath() string {
	return l.device
}

// SysDevicePath returns the device path in /sys.
func (l *LoopbackDevice) SysDevicePath() string {
	return path.Join("/sys/devices/virtual/block", path.Base(l.device))
}
