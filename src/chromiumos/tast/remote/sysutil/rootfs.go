// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package sysutil provides utilities for getting system-related information.
package sysutil

import (
	"context"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/rpc"
	"chromiumos/tast/testing"
)

func removeRootfsVerification(ctx context.Context, d *dut.DUT) error {
	output, err := d.Conn().Command("/usr/share/vboot/bin/make_dev_ssd.sh", "--remove_rootfs_verification", "--force").CombinedOutput(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to run make_dev_ssd.sh: %q", string(output))
	}
	return nil
}

// MakeRootfsWritable makes the rootfs writable.
func MakeRootfsWritable(ctx context.Context, d *dut.DUT, rpcHint *testing.RPCHint) error {
	cl, err := rpc.Dial(ctx, d, rpcHint, "cros")
	if err != nil {
		return errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer cl.Close(ctx)

	writable, err := IsRootfsWritable(ctx, cl)
	if err != nil {
		return err
	}

	if writable {
		return nil
	}

	err = makeRootfsWritable(ctx, d)
	if err != nil {
		return errors.Wrap(err, "failed to make rootfs writable")
	}

	// TODO(https://crbug.com/1195936): Need to reconnect to RPC service since we rebooted.
	cl, err = rpc.Dial(ctx, d, rpcHint, "cros")
	if err != nil {
		return errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer cl.Close(ctx)

	writable, err = IsRootfsWritable(ctx, cl)
	if err != nil {
		return errors.Wrap(err, "rootfs is not writable after enabling")
	}

	if !writable {
		return errors.New("rootfs is not writable after enabling")
	}

	return nil
}

func makeRootfsWritable(ctx context.Context, d *dut.DUT) error {
	err := removeRootfsVerification(ctx, d)
	if err != nil {
		return err
	}

	err = d.Reboot(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to reboot")
	}

	output, err := d.Conn().Command("mount", "-o", "remount,rw", "/").CombinedOutput(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to remount in read-write mode: %q", string(output))
	}

	return nil
}

func isRootfsWritable(procMounts string) (bool, error) {
	// Find line that starts with /dev/root
	rootLine := ""
	for _, line := range strings.Split(procMounts, "\n") {
		if strings.HasPrefix(line, "/dev/root") {
			rootLine = line
			break
		}
	}

	if rootLine == "" {
		return false, errors.New("unable to find /dev/root in /proc/mounts")
	}

	fields := strings.Fields(rootLine)
	if len(fields) < 4 {
		return false, errors.New("Unable to find attributes in mount line")
	}

	attributes := strings.Split(fields[3], ",")

	return attributes[0] == "rw", nil
}

// IsRootfsWritable returns true if the rootfs is writable.
func IsRootfsWritable(ctx context.Context, cl *rpc.Client) (bool, error) {
	// Use service to read /proc/mounts on device
	fs := dutfs.NewClient(cl.Conn)

	file, err := fs.ReadFile(ctx, "/proc/mounts")
	if err != nil {
		return false, errors.Wrap(err, "failed to read /proc/mounts")
	}

	return isRootfsWritable(string(file))
}
