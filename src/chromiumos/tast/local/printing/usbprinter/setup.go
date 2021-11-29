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
	"path"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/printing/lp"
	"chromiumos/tast/testing"
)

const defaultConfigInstallDirectory = "/usr/local/etc/virtual-usb-printer/"

// DevInfo contains information used to identify a USB device.
type DevInfo struct {
	// VID contains the device's vendor ID.
	VID string
	// PID contains the devices's product ID.
	PID string
}

// PrinterInfo contains all information needed to run the
// `virtual-usb-printer` process.
//
// Config data path fields obey these rules:
// 1.	Absolute paths are passed verbatim to the invocation of
//	`virtual-usb-printer`.
// 2.	Relative paths (and basenames) are joined with the default
//	install location of `virtual-usb-printer`'s config files.
// 3.	Empty fields are not passed to `virtual-usb-printer`.
type PrinterInfo struct {
	// Required: path to USB device descriptors.
	Descriptors string

	// Optional: path to device attributes (e.g. IPP attributes).
	Attributes string

	// Optional: path to eSCL capabilities.
	ESCLCapabilities string

	// Optional: value for `--output_log_directory`.
	// Not a config data path; i.e., must be an absolute path.
	OutputLogDirectory string

	// Optional: specifies the path where the print job should be
	// recorded. Not a config data path; i.e., must be an absolute
	// path.
	RecordPath string

	// Optional: specifies whether or not `Start()` should block on
	// waiting for the printer to be configured.
	WaitUntilConfigured bool
}

// PrinterInfoField provides the type for functional options used
// to build a `PrinterInstance` via `Start()`.
type PrinterInfoField func(*PrinterInfo) error

// WithDescriptors sets `PrinterInfo.Descriptors`.
func WithDescriptors(path string) PrinterInfoField {
	return func(i *PrinterInfo) error {
		i.Descriptors = path
		return nil
	}
}

// WithAttributes sets `PrinterInfo.Attributes`.
func WithAttributes(path string) PrinterInfoField {
	return func(i *PrinterInfo) error {
		i.Attributes = path
		return nil
	}
}

// WithESCLCapabilities sets `PrinterInfo.ESCLCapabilities`.
func WithESCLCapabilities(path string) PrinterInfoField {
	return func(i *PrinterInfo) error {
		i.ESCLCapabilities = path
		return nil
	}
}

// WithOutputLogDirectory sets `PrinterInfo.OutputLogDirectory`.
func WithOutputLogDirectory(directory string) PrinterInfoField {
	return func(i *PrinterInfo) error {
		if !path.IsAbs(directory) {
			return errors.Errorf("not an absolute path: %s", directory)
		}
		i.OutputLogDirectory = directory
		return nil
	}
}

// WithRecordPath sets `PrinterInfo.RecordPath`.
func WithRecordPath(record string) PrinterInfoField {
	return func(i *PrinterInfo) error {
		if !path.IsAbs(record) {
			return errors.Errorf("not an absolute path: %s", record)
		}
		i.RecordPath = record
		return nil
	}
}

// WaitUntilConfigured sets `PrinterInfo.WaitUntilConfigured`.
func WaitUntilConfigured() PrinterInfoField {
	return func(i *PrinterInfo) error {
		i.WaitUntilConfigured = true
		return nil
	}
}

// PrinterInstance provides an interface to interact with the running
// `virtual-usb-printer` instance.
type PrinterInstance struct {
	// The printer name as detected by autoconfiguration.
	// Empty if `Start()` was called with `info.WaitUntilConfigured`
	// set false.
	ConfiguredName string

	// The printer's device information parsed from its USB
	// descriptors config.
	DevInfo DevInfo

	// The running `virtual-usb-printer` instance.
	cmd *testexec.Cmd
}

func ippUSBPrinterURI(devInfo DevInfo) string {
	return fmt.Sprintf("ippusb://%s_%s/ipp/print", devInfo.VID, devInfo.PID)
}

// loadPrinterIDs loads the JSON file located at path and attempts to extract
// the "vid" and "pid" from the USB device descriptor which should be defined
// in path.
func loadPrinterIDs(path string) (devInfo DevInfo, err error) {
	f, err := os.Open(absoluteConfigPath(path))
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

// absoluteConfigPath returns
// *	`configPath` untouched if it is absolute or
// *	`configPath` prefixed with the default install directory of
//	`virtual-usb-printer`'s config files if it is relative.
func absoluteConfigPath(configPath string) string {
	if path.IsAbs(configPath) {
		return configPath
	}
	return path.Join(defaultConfigInstallDirectory, configPath)
}

func buildPrinterCommand(info PrinterInfo) []string {
	// The actual base command is `stdbuf`, which is fed as a
	// separate argument to `testexec.CommandContext()`, so we
	// don't include it here.
	command := []string{"-o0", "virtual-usb-printer",
		"--descriptors_path=" + absoluteConfigPath(info.Descriptors)}
	if len(info.Attributes) > 0 {
		command = append(command,
			"--attributes_path="+absoluteConfigPath(info.Attributes))
	}
	if len(info.ESCLCapabilities) > 0 {
		command = append(command,
			"--scanner_capabilities_path="+absoluteConfigPath(info.ESCLCapabilities))
	}
	if len(info.OutputLogDirectory) > 0 {
		command = append(command, "--output_log_dir="+info.OutputLogDirectory)
	}
	if len(info.RecordPath) > 0 {
		command = append(command, "--record_doc_path="+info.RecordPath)
	}
	return command
}

func launchPrinter(ctx context.Context, info PrinterInfo) (cmd *testexec.Cmd, err error) {
	args := buildPrinterCommand(info)
	testing.ContextLog(ctx, "Starting virtual printer: ", args)

	// Cleanup is centralized in `Start()`, so long as `cmd` is
	// returned as a non-nil.
	launch := testexec.CommandContext(ctx, "stdbuf", args...)

	p, err := launch.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := launch.Start(); err != nil {
		return nil, errors.Wrapf(err, "failed to start %v", launch.Args)
	}

	if err := waitLaunch(p); err != nil {
		return cmd, errors.Wrap(err, "failed to launch virtual printer")
	}

	testing.ContextLog(ctx, "Started virtual printer")

	// We pull everything out from the pipe so that
	// `virtual-usb-printer` doesn't block on writing to stdout.
	go io.Copy(ioutil.Discard, p)

	return launch, nil
}

// Start creates a new `PrinterInstance`, starting the underlying
// `virtual-usb-printer` process.
func Start(ctx context.Context, fields ...PrinterInfoField) (instance PrinterInstance, err error) {
	var info PrinterInfo
	for _, field := range fields {
		if err = field(&info); err != nil {
			return instance, err
		}
	}

	// The check goes here because a caller could forget to call
	// `Descriptors()` entirely, rather than just calling it with an
	// empty string.
	if len(info.Descriptors) == 0 {
		return instance, errors.New("`Descriptors` are required")
	}

	devInfo, err := loadPrinterIDs(info.Descriptors)
	if err != nil {
		return instance, err
	}
	cmd, err := launchPrinter(ctx, info)
	earlyCleanupCommand := cmd
	defer func() {
		if earlyCleanupCommand != nil {
			earlyCleanupCommand.Kill()
			earlyCleanupCommand.Wait()
		}
	}()
	if err != nil {
		return instance, err
	}

	err = attachUSBIPDevice(ctx, devInfo)
	if err != nil {
		return instance, err
	}

	printerName := ""
	if info.WaitUntilConfigured {
		printerName, err = WaitPrinterConfigured(ctx, devInfo)
		if err != nil {
			return instance, err
		}
	}

	instance = PrinterInstance{ConfiguredName: printerName, DevInfo: devInfo, cmd: cmd}
	earlyCleanupCommand = nil
	return instance, nil
}

// Stop terminates and waits for the `virtual-usb-printer`. Users must
// call this when finished with the `virtual-usb-printer`.
//
// Returns an error if
// *	we don't observe a udev signal that a USB device has
//	been removed _and_
// *	`expectUdevEvent` is true.
//
// This method is idempotent.
func (p *PrinterInstance) Stop(ctx context.Context, expectUdevEvent bool) error {
	if p.cmd == nil {
		return nil
	}
	defer func() {
		p.cmd = nil
	}()

	var udevCh chan error
	if expectUdevEvent {
		udevCh = make(chan error, 1)
		go func() {
			udevCh <- waitEvent(ctx, "remove", p.DevInfo)
		}()
	}

	p.cmd.Kill()
	p.cmd.Wait()

	if expectUdevEvent {
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
	}

	return nil
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

// WaitPrinterConfigured waits for a printer which has the same VID/PID as
// devInfo to be configured on the system. If a match is found then the name of
// the configured device will be returned.
func WaitPrinterConfigured(ctx context.Context, devInfo DevInfo) (string, error) {
	var foundName string
	uri := ippUSBPrinterURI(devInfo)
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
