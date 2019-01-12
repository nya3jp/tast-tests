package setup

import (
	"context"
	"fmt"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/usbprinter/monitor"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Config represents the configuration of a printer.
type Config struct {
	action          string // action to look for in udev events.
	vid             string // vendor ID of printer.
	pid             string // product ID of printer.
	descriptorsPath string // path to USB descriptors JSON file.
}

// NewConfig creates a new printer Config object with the given values.
func NewConfig(action, vid, pid, descriptorPath string) Config {
	return Config{action, vid, pid, descriptorPath}
}

// Printer sets up a new virtual printer with the given Config |conf| and
// attaches it to the system using USBIP.
func Printer(ctx context.Context, conf Config) (*testexec.Cmd, error) {
	testing.ContextLog(ctx, "Starting virtual printer")
	descriptorArg := fmt.Sprintf("--descriptors_path=%s",
		conf.descriptorsPath)

	launch := testexec.CommandContext(ctx, "stdbuf", "-o0", "virtual-usb-printer",
		descriptorArg)

	p, err := launch.StdoutPipe()
	if err != nil {
		return launch, err
	}

	if err := launch.Start(); err != nil {
		return launch, err
	}

	// Ensure that virtual-usb-printer has launched successfully.
	if err := monitor.PrinterLaunch(ctx, p); err != nil {
		return launch, errors.Errorf("Failed to launch virtual printer: %s", err)
	}

	testing.ContextLog(ctx, "Started virtual printer")

	// Begin waiting for udev event.
	udevCh := make(chan error, 1)
	go monitor.PrinterEvent(ctx, udevCh, conf.action, conf.vid, conf.pid)

	// Attach the virtual printer to the system using the "usbip attach" command.
	testing.ContextLog(ctx, "Attaching virtual printer")
	attach := testexec.CommandContext(ctx, "usbip", "attach", "-r", "localhost",
		"-b", "1-1")
	if err := attach.Run(); err != nil {
		return launch, errors.Errorf("Failed to attach virtual usb printer: %s",
			err)
	}

	// Wait for a signal from udevadm to see if the device was successfully
	// attached.
	select {
	case err := <-udevCh:
		if err != nil {
			return launch, err
		}
		testing.ContextLog(ctx, "Found event ", conf.action)
	}

	// Run lsusb to sanity check that that the device is actually connected.
	id := fmt.Sprintf("%s:%s", conf.vid, conf.pid)
	checkAttached := testexec.CommandContext(ctx, "lsusb", "-d", id)
	if err := checkAttached.Run(); err != nil {
		checkAttached.Wait()
		checkAttached.DumpLog(ctx)
		return launch, errors.Errorf("Printer was not successfully attached: %s",
			err)
	}

	return launch, nil
}
