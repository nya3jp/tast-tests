// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package usbprinter provides an interface to configure and attach a virtual
// USB printer onto the system to be used for testing.
package usbprinter

import (
	"context"
	"fmt"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// installModules installs the "usbip_core" and "vhci-hcd" kernel modules which
// are required by usbip in order to bind the virtual printer to the system.
func installModules(ctx context.Context) error {
	cmd1 := testexec.CommandContext(ctx, "modprobe", "usbip_core")
	if err := cmd1.Run(); err != nil {
		return errors.Wrap(err, "failed to install usbip_core module")
	}
	cmd2 := testexec.CommandContext(ctx, "modprobe", "vhci-hcd")
	if err := cmd2.Run(); err != nil {
		return errors.Wrap(err, "failed to install vhci-hcd module")
	}
	return nil
}

// Setup sets up a runs a new virtual printer and attaches it to the system
// using USBIP. Returns the Cmd used to run the virtual printer so that it can
// be properly shut down once the test has completed.
func Setup(ctx context.Context, action, vid, pid, descriptorsPath string,
) (cmd *testexec.Cmd, err error) {
	if err := installModules(ctx); err != nil {
		return nil, err
	}

	testing.ContextLog(ctx, "starting virtual printer")
	descriptorArg := "--descriptors_path=" + descriptorsPath
	launch := testexec.CommandContext(ctx, "stdbuf", "-o0", "virtual-usb-printer",
		descriptorArg)

	p, err := launch.StdoutPipe()
	defer func() {
		if err != nil && launch != nil {
			launch.Kill()
			launch.Wait()
		}
	}()

	if err != nil {
		return nil, err
	}

	if err := launch.Start(); err != nil {
		return nil, err
	}

	// Ensure that virtual-usb-printer has launched successfully.
	if err := waitLaunch(ctx, p); err != nil {
		return nil, errors.Wrap(err, "failed to launch virtual printer")
	}

	testing.ContextLog(ctx, "started virtual printer")

	// Begin waiting for udev event.
	udevCh := make(chan error, 1)
	go func() { udevCh <- waitEvent(ctx, action, vid, pid) }()

	// Attach the virtual printer to the system using the "usbip attach" command.
	testing.ContextLog(ctx, "attaching virtual printer")
	attach := testexec.CommandContext(ctx, "usbip", "attach", "-r", "localhost",
		"-b", "1-1")
	if err := attach.Run(); err != nil {
		return nil, errors.Wrap(err, "failed to attach virtual usb printer")
	}

	// Wait for a signal from udevadm to see if the device was successfully
	// attached.
	select {
	case err := <-udevCh:
		if err != nil {
			return nil, err
		}
		testing.ContextLog(ctx, "found event ", action)
	}

	// Run lsusb to sanity check that that the device is actually connected.
	id := fmt.Sprintf("%s:%s", vid, pid)
	checkAttached := testexec.CommandContext(ctx, "lsusb", "-d", id)
	if err := checkAttached.Run(); err != nil {
		checkAttached.DumpLog(ctx)
		return nil, errors.Wrap(err, "printer was not successfully attached")
	}

	return launch, nil
}
