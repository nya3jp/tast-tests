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
	"syscall"

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

// config contains all information needed to run the
// virtual-usb-printer process.
type config struct {
	// The actual arguments fed to (and not including) stdbuf.
	//
	// Config data path fields obey these rules:
	// 1. Absolute paths are passed verbatim to the invocation of
	//    virtual-usb-printer.
	// 2. Relative paths (and basenames) are joined with the default
	//    install location of virtual-usb-printer's config files.
	args []string

	// Populated with path from WithDescriptors().
	descriptors string

	// Whether or not Start() blocks on printer autoconfiguration.
	waitUntilConfigured bool

	// Whether or not Printer.Stop() should propagate an error if
	// no udev event is observed on stoppage.
	expectUdevEventOnStop bool
}

// Option provides the type for functional options used
// to build a Printer via Start().
type Option func(*config) error

// WithDescriptors sets the required descriptors.
func WithDescriptors(path string) Option {
	return func(o *config) error {
		if len(path) == 0 {
			return errors.New("empty descriptors path")
		}
		o.args = append(o.args, "--descriptors_path="+absoluteConfigPath(path))
		o.descriptors = absoluteConfigPath(path)
		return nil
	}
}

// WithAttributes sets attributes.
func WithAttributes(path string) Option {
	return func(o *config) error {
		if len(path) == 0 {
			return errors.New("empty attributes path")
		}
		o.args = append(o.args, "--attributes_path="+absoluteConfigPath(path))
		return nil
	}
}

// WithESCLCapabilities sets eSCL capabilities.
func WithESCLCapabilities(path string) Option {
	return func(o *config) error {
		if len(path) == 0 {
			return errors.New("empty eSCL capabilities path")
		}
		o.args = append(o.args, "--scanner_capabilities_path="+absoluteConfigPath(path))
		return nil
	}
}

// WithOutputLogDirectory sets the output log directory.
func WithOutputLogDirectory(directory string) Option {
	return func(o *config) error {
		if !path.IsAbs(directory) {
			return errors.Errorf("output log directory is (%q) not an absolute path", directory)
		}
		o.args = append(o.args, "--output_log_dir="+directory)
		return nil
	}
}

// WithRecordPath sets the document output path.
func WithRecordPath(record string) Option {
	return func(o *config) error {
		if !path.IsAbs(record) {
			return errors.Errorf("not an absolute path: %s", record)
		}
		o.args = append(o.args, "--record_doc_path="+record)
		return nil
	}
}

// WaitUntilConfigured controls whether or not Start() blocks on printer
// autoconfiguration.
func WaitUntilConfigured() Option {
	return func(o *config) error {
		o.waitUntilConfigured = true
		return nil
	}
}

// ExpectUdevEventOnStop causes Printer.Stop() to propagate errors if
// a udev event is not seen.
func ExpectUdevEventOnStop() Option {
	return func(o *config) error {
		o.expectUdevEventOnStop = true
		return nil
	}
}

// Printer provides an interface to interact with the running
// virtual-usb-printer instance.
type Printer struct {
	// The printer name as detected by autoconfiguration.
	// Empty if Start() was called with info.WaitUntilConfigured
	// set false.
	ConfiguredName string

	// The printer's device information parsed from its USB
	// descriptors config.
	DevInfo DevInfo

	// The running virtual-usb-printer instance.
	cmd *testexec.Cmd

	// Whether or not Stop() should propagate an error if
	// no udev event is observed on stoppage.
	expectUdevEventOnStop bool
}

func ippUSBPrinterURI(devInfo DevInfo) string {
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

// absoluteConfigPath returns configPath untouched if it is absolute.
// Otherwise, it returns configPath prefixed with the default install
// directory of virtual-usb-printer.
func absoluteConfigPath(configPath string) string {
	if path.IsAbs(configPath) {
		return configPath
	}
	return path.Join("/usr/local/etc/virtual-usb-printer/", configPath)
}

func launchPrinter(ctx context.Context, op config) (cmd *testexec.Cmd, err error) {
	testing.ContextLog(ctx, "Starting virtual printer: ", op.args)

	launch := testexec.CommandContext(ctx, "stdbuf", op.args...)

	p, err := launch.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := launch.Start(); err != nil {
		return nil, errors.Wrapf(err, "failed to start %v", launch.Args)
	}
	earlyCleanupCommand := launch
	defer func() {
		if earlyCleanupCommand == nil {
			return
		}
		if cleanupErr := earlyCleanupCommand.Signal(syscall.SIGTERM); cleanupErr != nil {
			if err == nil {
				err = cleanupErr
			} else {
				testing.ContextLogf(ctx, "termination failed (%q)", err)
			}
		}
		if cleanupErr := earlyCleanupCommand.Wait(); cleanupErr != nil {
			if err == nil {
				err = cleanupErr
			} else {
				// This error is noisy: sending SIGTERM always causes Wait()
				// to return an error.
				testing.ContextLogf(ctx, "wait failed (%q)", err)
			}
		}
	}()

	if err := waitLaunch(p); err != nil {
		return nil, errors.Wrap(err, "failed to launch virtual printer")
	}

	testing.ContextLog(ctx, "Started virtual printer")

	// We pull everything out from the pipe so that
	// virtual-usb-printer doesn't block on writing to stdout.
	go io.Copy(ioutil.Discard, p)

	earlyCleanupCommand = nil
	return launch, nil
}

// Start creates a new Printer and starts the underlying
// virtual-usb-printer process.
func Start(ctx context.Context, opts ...Option) (*Printer, error) {
	op := config{
		args: []string{"-o0", "virtual-usb-printer"},
	}
	for _, field := range opts {
		if err := field(&op); err != nil {
			return nil, err
		}
	}

	if len(op.descriptors) == 0 {
		return nil, errors.New("missing required WithDescriptors() option")
	}

	devInfo, err := LoadPrinterIDs(op.descriptors)
	if err != nil {
		return nil, err
	}
	cmd, err := launchPrinter(ctx, op)
	if err != nil {
		return nil, err
	}
	earlyCleanupCommand := cmd
	defer func() {
		if earlyCleanupCommand == nil {
			return
		}
		if cleanupErr := earlyCleanupCommand.Signal(syscall.SIGTERM); cleanupErr != nil {
			if err == nil {
				err = cleanupErr
			} else {
				testing.ContextLogf(ctx, "termination failed (%q)", err)
			}
		}
		if cleanupErr := earlyCleanupCommand.Wait(); cleanupErr != nil {
			if err == nil {
				err = cleanupErr
			} else {
				// This error is noisy: sending SIGTERM always causes Wait()
				// to return an error.
				testing.ContextLogf(ctx, "wait failed (%q)", err)
			}
		}
	}()

	if err = attachUSBIPDevice(ctx, devInfo); err != nil {
		return nil, err
	}

	printerName := ""
	if op.waitUntilConfigured {
		printerName, err = WaitPrinterConfigured(ctx, devInfo)
		if err != nil {
			return nil, err
		}
	}

	earlyCleanupCommand = nil
	return &Printer{
		ConfiguredName:        printerName,
		DevInfo:               devInfo,
		cmd:                   cmd,
		expectUdevEventOnStop: op.expectUdevEventOnStop,
	}, nil
}

// Stop terminates and waits for the virtual-usb-printer. Users must
// call this when finished with the virtual-usb-printer.
//
// Returns an error if we fail to terminate or wait for the
// virtual-usb-printer, or if we don't see an expected udev event
// upon stoppage.
//
// This method is idempotent.
func (p *Printer) Stop(ctx context.Context) error {
	if p.cmd == nil {
		return nil
	}
	defer func() {
		p.cmd = nil
	}()

	var udevCh chan error
	if p.expectUdevEventOnStop {
		udevCh = make(chan error, 1)
		go func() {
			udevCh <- waitEvent(ctx, "remove", p.DevInfo)
		}()
	}

	if err := p.cmd.Signal(syscall.SIGTERM); err != nil {
		testing.ContextLogf(ctx, "Failed to terminate printer (%q)", err)
	}
	if err := p.cmd.Wait(); err != nil {
		// This error is noisy: sending SIGTERM always causes Wait() to
		// return an error.
		testing.ContextLogf(ctx, "Failed to wait on printer (%q)", err)
	}

	if p.expectUdevEventOnStop {
		// Wait for a signal from udevadm to say the device was successfully
		// detached.
		testing.ContextLog(ctx, "Waiting for udev event")
		select {
		case err := <-udevCh:
			if err != nil {
				return err
			}
			testing.ContextLog(ctx, "received remove event")
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "didn't receive udev event")
		}
	}

	return nil
}

// runVirtualUsbPrinter starts an instance of virtual-usb-printer with the
// given arguments.  Waits until the printer has been launched successfully,
// and then returns the command.
// The returned command must be stopped using Kill()/Wait() once testing is
// complete.
func runVirtualUsbPrinter(ctx context.Context, descriptors, attributes, record, esclCaps, logDir string) (cmd *testexec.Cmd, err error) {
	testing.ContextLog(ctx, "Starting virtual printer")
	launch := testexec.CommandContext(ctx, "stdbuf", "-o0", "virtual-usb-printer", "--descriptors_path="+descriptors, "--attributes_path="+attributes, "--record_doc_path="+record, "--scanner_capabilities_path="+esclCaps, "--output_log_dir="+logDir)

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
// esclCaps is a path to a JSON config file which specifies the supported
// behavior of the scanner.
//
// The returned command is already started and must be stopped (by calling its
// Kill and Wait methods) when testing is complete.
func StartScanner(ctx context.Context, devInfo DevInfo, descriptors, attributes, esclCaps, logDir string) (cmd *testexec.Cmd, err error) {
	virtualUsbPrinter, err := runVirtualUsbPrinter(ctx, descriptors, attributes, "", esclCaps, logDir)
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

// startDeprecated sets up and runs a new virtual printer and attaches it to the system
// using USBIP. The given descriptors and attributes provide the virtual printer
// with paths to the USB descriptors and IPP attributes files respectively. The
// path to the file to write received documents is specified by record. The
// returned command is already started and must be stopped (by calling its Kill
// and Wait methods) when testing is complete.
func startDeprecated(ctx context.Context, devInfo DevInfo, descriptors, attributes, record string) (cmd *testexec.Cmd, err error) {
	virtualUsbPrinter, err := runVirtualUsbPrinter(ctx, descriptors, attributes, record, "", "")
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
	printer, err := startDeprecated(ctx, devInfo, descriptors, attributes, record)
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
