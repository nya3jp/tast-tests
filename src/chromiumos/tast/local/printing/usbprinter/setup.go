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

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/printing/lp"
	"chromiumos/tast/testing"
)

// DevInfo contains information used to identify a USB device.
type DevInfo struct {
	// VID contains the device's vendor ID.
	VID string
	// PID contains the devices's product ID.
	PID string
}

func ippUSBPrinterURI(ctx context.Context, devInfo DevInfo) string {
	return fmt.Sprintf("ippusb://%s_%s/ipp/print", devInfo.VID, devInfo.PID)
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

// runVirtualUsbPrinter starts an instance of virtual-usb-printer with the
// given arguments.  Waits until the printer has been launched successfully,
// and then returns the command.
// The returned command must be stopped using Kill()/Wait() once testing is
// complete.
func runVirtualUsbPrinter(ctx context.Context, descriptors, attributes, record, esclCaps, scanPath, logDir string) (cmd *testexec.Cmd, err error) {
	testing.ContextLog(ctx, "Starting virtual printer")
	launch := testexec.CommandContext(ctx, "stdbuf", "-o0", "virtual-usb-printer", "--descriptors_path="+descriptors, "--attributes_path="+attributes, "--record_doc_path="+record, "--scanner_capabilities_path="+esclCaps, "--scanner_doc_path="+scanPath, "--output_log_dir="+logDir)

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

	cmdToKill = nil
	return launch, nil
}

// attachUSBIPDevice attaches the UsbIp device specified by devInfo to the
// system. Returns nil if the device was attached successfully.
func attachUSBIPDevice(ctx context.Context, devInfo DevInfo) error {
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
		return errors.Wrap(err, "failed to attach virtual usb printer")
	}

	// Wait for a signal from udevadm to see if the device was successfully
	// attached.
	testing.ContextLog(ctx, "Waiting for udev event")
	select {
	case err := <-udevCh:
		if err != nil {
			return err
		}
		testing.ContextLog(ctx, "Found add event")
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "didn't get udev event")
	}

	// Run lsusb to validity check that that the device is actually connected.
	id := fmt.Sprintf("%s:%s", devInfo.VID, devInfo.PID)
	checkAttached := testexec.CommandContext(ctx, "lsusb", "-d", id)
	if err := checkAttached.Run(); err != nil {
		checkAttached.DumpLog(ctx)
		return errors.Wrap(err, "printer was not successfully attached")
	}
	return nil
}

// StartScanner sets up and runs a new virtual printer with scanner support and
// attaches it to the system using USBIP. The given descriptors and attributes
// provide the virtual printer with paths to the USB descriptors and IPP
// attributes files, which are necessary to setup the eSCL over IPP connection.
//
// scanPath is the path to use as the source for scanned documents, while
// esclCaps is a path to a JSON config file which specifies the supported
// behavior of the scanner.
//
// The returned command is already started and must be stopped (by calling its
// Kill and Wait methods) when testing is complete.
func StartScanner(ctx context.Context, devInfo DevInfo, descriptors, attributes, esclCaps, scanPath, logDir string) (cmd *testexec.Cmd, err error) {
	virtualUsbPrinter, err := runVirtualUsbPrinter(ctx, descriptors, attributes, "", esclCaps, scanPath, logDir)
	if err != nil {
		return nil, errors.Wrap(err, "runVirtualUsbPrinter failed")
	}
	cmdToKill := virtualUsbPrinter
	defer func() {
		if cmdToKill != nil {
			virtualUsbPrinter.Kill()
			virtualUsbPrinter.Wait()
		}
	}()

	err = attachUSBIPDevice(ctx, devInfo)
	if err != nil {
		return nil, errors.Wrap(err, "attaching usbip device failed")
	}
	cmdToKill = nil
	return virtualUsbPrinter, nil
}

// Start sets up and runs a new virtual printer and attaches it to the system
// using USBIP. The given descriptors and attributes provide the virtual printer
// with paths to the USB descriptors and IPP attributes files respectively. The
// path to the file to write received documents is specified by record. The
// returned command is already started and must be stopped (by calling its Kill
// and Wait methods) when testing is complete.
func Start(ctx context.Context, devInfo DevInfo, descriptors, attributes, record string) (cmd *testexec.Cmd, err error) {
	virtualUsbPrinter, err := runVirtualUsbPrinter(ctx, descriptors, attributes, record, "", "", "")
	if err != nil {
		return nil, errors.Wrap(err, "runVirtualUsbPrinter failed")
	}
	cmdToKill := virtualUsbPrinter
	defer func() {
		if cmdToKill != nil {
			virtualUsbPrinter.Kill()
			virtualUsbPrinter.Wait()
		}
	}()

	err = attachUSBIPDevice(ctx, devInfo)
	if err != nil {
		return nil, errors.Wrap(err, "attaching usbip device failed")
	}
	cmdToKill = nil
	return virtualUsbPrinter, nil
}

// StartIPPUSB performs the same configuration as Start(), with the additional
// expectation that the given printer configuration defines an IPP-over-USB
// capable printer. For this reason, StartIPPUSB will wait for CUPS to
// automatically configure the printer and return the given name of the
// configured printer.
func StartIPPUSB(ctx context.Context, devInfo DevInfo, descriptors, attributes, record string) (cmd *testexec.Cmd, name string, err error) {
	printer, err := Start(ctx, devInfo, descriptors, attributes, record)
	if err != nil {
		return nil, "", errors.Wrap(err, "failed to attach virtual printer")
	}
	// Since the given printer should describe use IPP-over-USB, we wait for it to
	// be automatically configured by Chrome in order to extract the name of the
	// device.
	testing.ContextLog(ctx, "Waiting for printer to be configured")
	name, err = WaitPrinterConfigured(ctx, devInfo)
	if err != nil {
		printer.Kill()
		printer.Wait()
		return nil, "", errors.Wrap(err, "failed to find configured printer name")
	}
	testing.ContextLog(ctx, "Printer configured with name: ", name)
	return printer, name, nil
}

// WaitPrinterConfigured waits for a printer which has the same VID/PID as
// devInfo to be configured on the system. If a match is found then the name of
// the configured device will be returned.
func WaitPrinterConfigured(ctx context.Context, devInfo DevInfo) (string, error) {
	var foundName string
	uri := ippUSBPrinterURI(ctx, devInfo)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		name, err := lp.PrinterNameByURI(ctx, uri)
		if err != nil {
			return err
		}
		foundName = name
		return nil
	}, nil); err != nil {
		return "", err
	}
	return foundName, nil
}

// StopPrinter terminates the virtual-usb-printer process, then waits for a
// udev event indicating that its associated USB device has been removed.
func StopPrinter(ctx context.Context, cmd *testexec.Cmd, devInfo DevInfo) error {
	// Begin waiting for udev event.
	udevCh := make(chan error, 1)
	go func() {
		udevCh <- waitEvent(ctx, "remove", devInfo)
	}()

	cmd.Kill()
	cmd.Wait()

	// Wait for a signal from udevadm to say the device was successfully
	// detached.
	testing.ContextLog(ctx, "Waiting for udev event")
	select {
	case err := <-udevCh:
		if err != nil {
			return err
		}
		testing.ContextLog(ctx, "Found remove event")
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "didn't get udev event")
	}

	return nil
}
