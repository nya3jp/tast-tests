// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type ecUsbPortTest int

const (
	testUSBOnLidClose ecUsbPortTest = iota
	testUSBOnShutdown
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECUSBPorts,
		Desc:         "Verify usb ports stop read/write after DUT shuts down",
		Contacts:     []string{"tij@google.com", "cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Fixture:      fixture.NormalMode,
		Params: []testing.Param{
			{
				Name:              "usb_pins_on_lid_close",
				Val:               testUSBOnLidClose,
				ExtraHardwareDeps: hwdep.D(hwdep.Lid()),
			},
			{
				Name: "usb_pins_on_shutdown",
				Val:  testUSBOnShutdown,
			},
		},
	})
}

const (
	// Output from ec console for gpioget or ioexget looks like:
	// "0* EN_USB_A0_5V" for gpio, or "1* O H EN_USB_A0_5V" for ioex.
	reECUSBPortGet string = `(?i)(0|1)[^\r\n]*%s`
)

func ECUSBPorts(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	enablePins, err := getUSBPorts(ctx, h)
	if err != nil {
		s.Fatal("Failed to probe usb ports: ", err)
	}

	if err := h.DUT.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep for 5 seconds: ", err)
	}

	s.Log("Check that ports are initially enabled")
	if err := checkUSBAPortEnabled(ctx, h, enablePins, 1); err != nil {
		s.Fatal("Expected USB Ports to be enabled: ", err)
	}

	switch s.Param().(ecUsbPortTest) {
	case testUSBOnShutdown:
		if err := testPortsAfterShutdown(ctx, h, enablePins); err != nil {
			s.Fatal("Some USB Ports enabled after shutdown: ", err)
		}
	case testUSBOnLidClose:
		if err := testPortsAfterLidClose(ctx, h, enablePins); err != nil {
			s.Fatal("Some USB Ports enabled after lidclose: ", err)
		}
	}
}

func testPortsAfterLidClose(ctx context.Context, h *firmware.Helper, enablePins []firmware.USBEnablePin) error {
	if err := h.Servo.CloseLid(ctx); err != nil {
		return errors.Wrap(err, "failed to close lid")
	}

	testing.ContextLog(ctx, "Check for G3 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3"); err != nil {
		return errors.Wrap(err, "failed to get G3 powerstate")
	}

	testing.ContextLog(ctx, "Check that ports have state 0")
	if err := checkUSBAPortEnabled(ctx, h, enablePins, 0); err != nil {
		return errors.Wrap(err, "failed to check usb ports")
	}

	if err := h.Servo.OpenLid(ctx); err != nil {
		return errors.Wrap(err, "failed to open lid")
	}

	testing.ContextLog(ctx, "Waiting for S0 powerstate")
	err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0")
	if err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	if err := h.WaitConnect(ctx); err != nil {
		return errors.Wrap(err, "failed to reconnect to DUT after restarting")
	}

	return nil
}

func testPortsAfterShutdown(ctx context.Context, h *firmware.Helper, enablePins []firmware.USBEnablePin) error {
	testing.ContextLog(ctx, "Shut down DUT")
	cmd := h.DUT.Conn().CommandContext(ctx, "/sbin/shutdown", "-P", "now")
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to shut down DUT")
	}

	testing.ContextLog(ctx, "Check for G3 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3"); err != nil {
		return errors.Wrap(err, "failed to get G3 powerstate")
	}

	testing.ContextLog(ctx, "Check that ports have state 0")
	if err := checkUSBAPortEnabled(ctx, h, enablePins, 0); err != nil {
		return errors.Wrap(err, "failed to check usb ports")
	}

	testing.ContextLog(ctx, "Power DUT back on with short press of the power button")
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
		return errors.Wrap(err, "failed to power on DUT with short press of the power button")
	}

	testing.ContextLog(ctx, "Waiting for S0 powerstate")
	err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0")
	if err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	if err := h.WaitConnect(ctx); err != nil {
		return errors.Wrap(err, "failed to reconnect to DUT after restarting")
	}

	return nil
}

func getUSBPorts(ctx context.Context, h *firmware.Helper) ([]firmware.USBEnablePin, error) {
	enablePins := make([]firmware.USBEnablePin, 0)
	ec := firmware.NewECTool(h.DUT, firmware.ECToolNameMain)

	for _, pin := range h.Config.USBEnablePins {
		gpioName := firmware.GpioName(pin.Name)
		if !pin.Ioex {
			// Probe pin to verify it actually exists.
			_, err := ec.FindBaseGpio(ctx, []firmware.GpioName{gpioName})
			if err != nil {
				return enablePins, errors.Wrapf(err, "GPIO pin %q defined in fw testing configs but not found in gpio, update configs to reflect this", pin.Name)
			}
		} else {
			matchList := []string{fmt.Sprintf(reECUSBPortGet, pin.Name)}
			_, err := h.Servo.RunECCommandGetOutput(ctx, fmt.Sprintf("ioexget %s", pin.Name), matchList)
			if err != nil {
				return enablePins, errors.Wrapf(err, "IOEX pin %q defined in fw testing configs but not found in ioex, update configs to reflect this", pin.Name)
			}
		}
		enablePins = append(enablePins, pin)
	}

	portsToCheck := h.Config.USBAPortCount
	if portsToCheck <= 0 {
		// If the value is -1 (unknown) or 0 (default), we want to check a bunch of numbers manually.
		portsToCheck = 11
	}
	for i := 1; i <= portsToCheck; i++ {
		name := fmt.Sprintf("USB%d_ENABLE", i)
		testing.ContextLogf(ctx, "Probing port %q with gpioget", name)
		_, err := ec.FindBaseGpio(ctx, []firmware.GpioName{firmware.GpioName(name)})
		if err != nil && h.Config.USBAPortCount >= i {
			// If port i doesn't exist (regex fails) but it is expected to exist (0 < i <= h.Config.USBAPortCount), raise an error.
			return enablePins, errors.Errorf("explicit port count is %d; expected port %d to exist but it does not", h.Config.USBAPortCount, i)
		} else if err == nil {
			testing.ContextLogf(ctx, "Found usb port: %q with gpioget", name)
			enablePins = append(enablePins, firmware.USBEnablePin{Name: name, Ioex: false})
		} else {
			testing.ContextLogf(ctx, "Did not find port %q with gpioget, got err: %v", name, err)
		}
	}
	return enablePins, nil

}

func checkUSBAPortEnabled(ctx context.Context, h *firmware.Helper, enablePins []firmware.USBEnablePin, expectedStatusInt int) error {
	// Collect errors for all usb ports instead of failing at first.
	var unexpectedStatus = map[string]string{}
	expectedStatus := strconv.Itoa(expectedStatusInt)

	for _, pin := range enablePins {
		gpioOrIoex := "gpio"
		if pin.Ioex {
			gpioOrIoex = "ioex"
		}
		testing.ContextLogf(ctx, "Checking status of %q pin name: %q", gpioOrIoex, pin.Name)
		cmd := fmt.Sprintf("%sget %s", gpioOrIoex, pin.Name)
		matchList := []string{fmt.Sprintf(reECUSBPortGet, pin.Name)}
		out, err := h.Servo.RunECCommandGetOutput(ctx, cmd, matchList)
		if err != nil {
			return errors.Wrapf(err, "failed to run cmd %q, got error", cmd)
		}
		if out[0][1] != expectedStatus {
			unexpectedStatus[pin.Name] = out[0][1]
		}
	}

	if len(unexpectedStatus) != 0 {
		failStr := fmt.Sprintf("The following USB Ports didn't have state %q", expectedStatus)
		for name, state := range unexpectedStatus {
			failStr += fmt.Sprintf(", %q had status %q", name, state)
		}
		return errors.New(failStr)
	}
	return nil
}
