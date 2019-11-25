// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package usbprinter provides an interface to configure and attach a virtual
// USB printer onto the system to be used for testing.
package usbprinter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// DevInfo contains information used to identify a USB device.
type DevInfo struct {
	// VID contains the device's vendor ID.
	VID string
	// PID contains the devices's product ID.
	PID string
}

// LoadPrinterIDs loads the JSON file located at path and attempts to extract
// the "vid" and "pid" from the USB device descriptor which should be defined
// in path.
func LoadPrinterIDs(path string) (devInfo DevInfo, err error) {
	f, err := os.Open(path)
	if err != nil {
		return devInfo, errors.Wrapf(err, "failed to open %s", path)
	}
	defer f.Close()

	var cfg struct {
		DevDesc struct {
			Vendor  int `json:"idVendor"`
			Product int `json:"idProduct"`
		} `json:"device_descriptor"`
	}

	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return devInfo, errors.Wrapf(err, "failed to decode JSON in %s", path)
	}

	return DevInfo{fmt.Sprintf("%04x", cfg.DevDesc.Vendor), fmt.Sprintf("%04x", cfg.DevDesc.Product)}, nil
}

// InstallModules installs the "usbip_core" and "vhci-hcd" kernel modules which
// are required by usbip in order to bind the virtual printer to the system.
func InstallModules(ctx context.Context) error {
	cmd := testexec.CommandContext(ctx, "modprobe", "-a", "usbip_core",
		"vhci-hcd")
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "failed to install usbip kernel modules")
	}
	return nil
}

// RemoveModules removes the "usbip_core" and "vhci-hcd" kernel modules that
// were installed during the test run.
func RemoveModules(ctx context.Context) error {
	cmd := testexec.CommandContext(ctx, "modprobe", "-r", "-a", "vhci-hcd",
		"usbip_core")
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "failed to remove usbip kernel modules")
	}
	return nil
}

// Start sets up and runs a new virtual printer and attaches it to the system
// using USBIP. The given descriptors and attributes provide the virtual printer
// with paths to the USB descriptors and IPP attributes files respectively. The
// path to the file to write received documents is specified by record. The
// returned command is already started and must be stopped (by calling its Kill
// and Wait methods) when testing is complete.
func Start(ctx context.Context, devInfo DevInfo, descriptors, attributes, record string) (cmd *testexec.Cmd, err error) {
	testing.ContextLog(ctx, "Starting virtual printer")
	launch := testexec.CommandContext(ctx, "stdbuf", "-o0", "virtual-usb-printer", "--descriptors_path="+descriptors, "--attributes_path="+attributes, "--record_doc_path="+record)

	p, err := launch.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := launch.Start(); err != nil {
		return nil, errors.Wrapf(err, "failed to start %v", launch.Args)
	}
	cmdToKill := launch
	defer func() {
		if cmdToKill != nil {
			launch.Kill()
			launch.Wait()
		}
	}()

	// Ensure that virtual-usb-printer has launched successfully.
	if err := waitLaunch(p); err != nil {
		return nil, errors.Wrap(err, "failed to launch virtual printer")
	}
	testing.ContextLog(ctx, "Started virtual printer")

	// Need to read from the pipe so that the virtual printer doesn't block on
	// writing to stdout
	go io.Copy(ioutil.Discard, p)

	// Begin waiting for udev event.
	udevCh := make(chan error, 1)
	go func() {
		udevCh <- waitEvent(ctx, "add", devInfo)
	}()

	// Attach the virtual printer to the system using the "usbip attach" command.
	testing.ContextLog(ctx, "Attaching virtual printer")
	attach := testexec.CommandContext(ctx, "usbip", "attach", "-r", "localhost",
		"-b", "1-1")
	if err := attach.Run(); err != nil {
		return nil, errors.Wrap(err, "failed to attach virtual usb printer")
	}

	// Wait for a signal from udevadm to see if the device was successfully
	// attached.
	testing.ContextLog(ctx, "Waiting for udev event")
	select {
	case err := <-udevCh:
		if err != nil {
			return nil, err
		}
		testing.ContextLog(ctx, "Found add event")
	case <-ctx.Done():
		return nil, errors.Wrap(ctx.Err(), "didn't get udev event")
	}

	// Run lsusb to sanity check that that the device is actually connected.
	id := fmt.Sprintf("%s:%s", devInfo.VID, devInfo.PID)
	checkAttached := testexec.CommandContext(ctx, "lsusb", "-d", id)
	if err := checkAttached.Run(); err != nil {
		checkAttached.DumpLog(ctx)
		return nil, errors.Wrap(err, "printer was not successfully attached")
	}

	cmdToKill = nil
	return launch, nil
}

// StartIPPUSB performs the same configuration as Start(), with the additional
// expectation that the given printer configuration defines an IPP-over-USB
// capable printer. For this reason, StartIPPUSB will wait for CUPS to
// automatically configure the printer and return the given name of the
// configured printer.
func StartIPPUSB(ctx context.Context, devInfo DevInfo, descriptors, attributes, record string) (cmd *testexec.Cmd, name string, err error) {
	printer, err := Start(ctx, devInfo, descriptors, attributes, record)
	if err != nil {
		return nil, "", err
	}
	// Since the given printer should describe use IPP-over-USB, we wait for it to
	// be automatically configured by Chrome in order to extract the name of the
	// device.
	testing.ContextLog(ctx, "Waiting for printer to be configured")
	name, err = waitPrinterConfigured(ctx, devInfo)
	if err != nil {
		return printer, "", err
	}
	testing.ContextLog(ctx, "Printer configured with name: ", name)
	return printer, name, nil
}
