// Copyright 2019 The ChromiumOS Authors
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

	"golang.org/x/sys/unix"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/printing/lp"
	"chromiumos/tast/local/upstart"
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

// WithIPPUSBDescriptors passes the most commonly used USB descriptors.
func WithIPPUSBDescriptors() Option {
	return WithDescriptors("ippusb_printer.json")
}

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

// WithGenericIPPAttributes passes the most commonly used IPP attributes.
func WithGenericIPPAttributes() Option {
	return WithAttributes("ipp_attributes.json")
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
			return errors.Errorf("output log directory (%q) is not an absolute path", directory)
		}
		o.args = append(o.args, "--output_log_dir="+directory)
		return nil
	}
}

// WithHTTPLogDirectory sets the HTTP log directory.
func WithHTTPLogDirectory(directory string) Option {
	return func(o *config) error {
		if !path.IsAbs(directory) {
			return errors.Errorf("HTTP log directory (%q) is not an absolute path",
				directory)
		}
		o.args = append(o.args, "--http_header_output_dir="+directory)
		return nil
	}
}

// WithRecordPath sets the document output path.
func WithRecordPath(record string) Option {
	return func(o *config) error {
		if !path.IsAbs(record) {
			return errors.Errorf("record path (%q) is not an absolute path", record)
		}
		o.args = append(o.args, "--record_doc_path="+record)
		return nil
	}
}

// WithMockPrinterScriptPath sets the mock printer script path.
func WithMockPrinterScriptPath(script string) Option {
	return func(o *config) error {
		if !path.IsAbs(script) {
			return errors.Errorf("mock printer script path (%q) is not an absolute path", script)
		}
		o.args = append(o.args, "--mock_printer_script="+script)
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

	// The human-readable printer name as it would be displayed
	// in the UI.  This is parsed from its USB descriptors, e.g.
	// "DavieV Virtual USB Printer (USB)".
	VisibleName string
}

func ippUSBPrinterURI(devInfo DevInfo) string {
	return fmt.Sprintf("ippusb://%s_%s/ipp/print", devInfo.VID, devInfo.PID)
}

// loadPrinterIDs loads the JSON file located at path and attempts to extract
// the "vid" and "pid" from the USB device descriptor which should be defined
// in path.
func loadPrinterIDs(path string) (devInfo DevInfo, deviceName string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return devInfo, "", errors.Wrapf(err, "failed to open %s", path)
	}
	defer f.Close()

	var cfg struct {
		DevDesc struct {
			Vendor  int `json:"idVendor"`
			Product int `json:"idProduct"`
		} `json:"device_descriptor"`
		VendorModel []string `json:"string_descriptors"`
	}

	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return devInfo, "", errors.Wrapf(err, "failed to decode JSON in %s", path)
	}

	deviceName = fmt.Sprintf("%s %s (USB)", cfg.VendorModel[0], cfg.VendorModel[1])
	return DevInfo{fmt.Sprintf("%04x", cfg.DevDesc.Vendor), fmt.Sprintf("%04x", cfg.DevDesc.Product)}, deviceName, nil
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

func terminatePrinterProcess(ctx context.Context, cmd *testexec.Cmd) error {
	testing.ContextLogf(ctx, "Terminating virtual-usb-printer with PID %d", cmd.Cmd.Process.Pid)
	if err := cmd.Signal(unix.SIGTERM); err != nil {
		return errors.Wrap(err, "failed to send SIGTERM to virtual-usb-printer")
	}
	if err := cmd.Wait(); err != nil {
		// We're expecting the exit status to be non-zero if the process was killed by SIGTERM.
		// Anything else indicates a problem.
		if ws, ok := testexec.GetWaitStatus(err); !ok || !ws.Signaled() || ws.Signal() != unix.SIGTERM {
			return errors.Wrap(err, "failed to wait for virtual-usb-printer termination")
		}
	}
	return nil
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
	defer func(ctx context.Context) {
		if err == nil {
			return
		}
		if cleanupErr := terminatePrinterProcess(ctx, launch); cleanupErr != nil {
			testing.ContextLogf(ctx, "Virtual printer termination failed (%q)", cleanupErr)
		}
	}(ctx)

	if err := waitLaunch(p); err != nil {
		return nil, errors.Wrap(err, "failed to launch virtual printer")
	}

	// We pull everything out from the pipe so that
	// virtual-usb-printer doesn't block on writing to stdout.
	go io.Copy(ioutil.Discard, p)

	return launch, nil
}

// Start creates a new Printer and starts the underlying
// virtual-usb-printer process.
func Start(ctx context.Context, opts ...Option) (pr *Printer, err error) {
	// Debugd needs to be running before the USB device shows up so Chrome can add the printer.
	if err := upstart.EnsureJobRunning(ctx, "debugd"); err != nil {
		testing.ContextLogf(ctx, "debugd not running: %q", err)
		return nil, err
	}

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

	devInfo, deviceName, err := loadPrinterIDs(op.descriptors)
	if err != nil {
		return nil, err
	}
	cmd, err := launchPrinter(ctx, op)
	if err != nil {
		return nil, err
	}
	defer func(ctx context.Context) {
		if err == nil {
			return
		}
		if cleanupErr := cmd.Signal(unix.SIGTERM); cleanupErr != nil {
			testing.ContextLogf(ctx, "Virtual printer termination failed (%q)", cleanupErr)
		}
		if cleanupErr := cmd.Wait(); cleanupErr != nil {
			// This error is noisy: sending SIGTERM always causes Wait()
			// to return an error.
			testing.ContextLogf(ctx, "Virtual printer termination wait failed (%q)", cleanupErr)
		}
	}(ctx)

	if err = attachUSBIPDevice(ctx, devInfo); err != nil {
		return nil, err
	}

	printerName := ""
	if op.waitUntilConfigured {
		printerName, err = waitPrinterConfigured(ctx, devInfo)
		if err != nil {
			return nil, err
		}
	}

	return &Printer{
		ConfiguredName:        printerName,
		DevInfo:               devInfo,
		cmd:                   cmd,
		expectUdevEventOnStop: op.expectUdevEventOnStop,
		VisibleName:           deviceName,
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

	var udevCh <-chan error
	if p.expectUdevEventOnStop {
		var err error
		udevCh, err = startUdevMonitor(ctx, "remove", p.DevInfo)
		if err != nil {
			return err
		}
	}

	if err := terminatePrinterProcess(ctx, p.cmd); err != nil {
		testing.ContextLogf(ctx, "Failed to terminate printer (%q)", err)
	}

	if p.expectUdevEventOnStop {
		// Wait for a signal from udevadm to say the device was successfully
		// detached.
		testing.ContextLog(ctx, "Waiting for udev remove event")
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

// attachUSBIPDevice attaches the UsbIp device specified by devInfo to the
// system. Returns nil if the device was attached successfully.
func attachUSBIPDevice(ctx context.Context, devInfo DevInfo) error {
	// Begin waiting for udev event.
	udevCh, err := startUdevMonitor(ctx, "add", devInfo)
	if err != nil {
		return err
	}

	// Attach the virtual printer to the system using the "usbip attach" command.
	testing.ContextLog(ctx, "Attaching virtual printer")
	attach := testexec.CommandContext(ctx, "usbip", "attach", "-r", "localhost",
		"-b", "1-1")
	if err := attach.Run(); err != nil {
		return errors.Wrap(err, "failed to attach virtual usb printer")
	}

	// Wait for a signal from udevadm to see if the device was successfully
	// attached.
	testing.ContextLog(ctx, "Waiting for udev add event")
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

// waitPrinterConfigured waits for a printer which has the same VID/PID as
// devInfo to be configured on the system. If a match is found then the name of
// the configured device will be returned.
func waitPrinterConfigured(ctx context.Context, devInfo DevInfo) (string, error) {
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
