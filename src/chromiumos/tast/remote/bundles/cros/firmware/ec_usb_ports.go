// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECUSBPorts,
		Desc:         "Verify usb ports stop read/write after DUT shuts down",
		Contacts:     []string{"tij@google.com", "cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Fixture:      fixture.NormalMode,
	})
}

func ECUSBPorts(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	s.Log("Check that ports are initially enabled")
	if err := checkUSBAPortEnabled(ctx, h, 1); err != nil {
		s.Fatal("Expected USB Ports to be enabled")
	}

	if err := testPortsAfterShutdown(ctx, h, 0); err != nil {
		s.Fatal("Some USB Ports enabled after shutdown: ", err)
	}

	if err := testPortsAfterLidClose(ctx, h, 0); err != nil {
		s.Fatal("Some USB Ports enabled after lidclose: ", err)
	}
}

func testPortsAfterLidClose(ctx context.Context, h *firmware.Helper, expectedState int) error {
	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		return errors.Wrap(err, "failed to create mode switcher")
	}

	if err := h.Servo.CloseLid(ctx); err != nil {
		return errors.Wrap(err, "failed to close lid")
	}

	testing.ContextLog(ctx, "Check for G3 or S5 powerstate")
	if err := ms.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3", "S5"); err != nil {
		return errors.Wrap(err, "failed to get G3 or S5 powerstate")
	}

	testing.ContextLog(ctx, "Check that ports have state ", expectedState)
	if err := checkUSBAPortEnabled(ctx, h, expectedState); err != nil {
		return errors.Wrap(err, "failed to check usb ports")
	}

	if err := h.Servo.OpenLid(ctx); err != nil {
		return errors.Wrap(err, "failed to open lid")
	}

	testing.ContextLog(ctx, "Waiting for S0 powerstate")
	err = ms.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0")
	if err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	if err := h.WaitConnect(ctx); err != nil {
		return errors.Wrap(err, "failed to reconnect to DUT after restarting")
	}

	return nil
}

func testPortsAfterShutdown(ctx context.Context, h *firmware.Helper, expectedState int) error {
	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		return errors.Wrap(err, "failed to create mode switcher")
	}

	testing.ContextLog(ctx, "Shut down DUT")
	cmd := h.DUT.Conn().CommandContext(ctx, "/sbin/shutdown", "-P", "now")
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to shut down DUT")
	}

	testing.ContextLog(ctx, "Check for G3 powerstate")
	if err := ms.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3"); err != nil {
		return errors.Wrap(err, "failed to get G3 powerstate")
	}

	testing.ContextLog(ctx, "Check that ports have state ", expectedState)
	if err := checkUSBAPortEnabled(ctx, h, expectedState); err != nil {
		return errors.Wrap(err, "failed to check usb ports")
	}

	testing.ContextLog(ctx, "Power DUT back on with short press of the power button")
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
		return errors.Wrap(err, "failed to power on DUT with short press of the power button")
	}

	testing.ContextLog(ctx, "Waiting for S0 powerstate")
	err = ms.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0")
	if err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	if err := h.WaitConnect(ctx); err != nil {
		return errors.Wrap(err, "failed to reconnect to DUT after restarting")
	}

	return nil
}

func checkUSBAPortEnabled(ctx context.Context, h *firmware.Helper, expectedStatus int) error {
	var unexpectedStatus = map[string]int{}

	// Check named pins from the model config.
	if len(h.Config.USBEnablePins) != 0 {
		for _, pin := range h.Config.USBEnablePins {
			gpioOrIoex := "gpio"
			if pin.Ioex {
				gpioOrIoex = "ioex"
			}
			cmd := fmt.Sprintf("%sget %s", gpioOrIoex, pin.Name)
			matchList := []string{fmt.Sprintf(`(0|1)[^\n\r]*\s%s`, pin.Name)}
			out, err := h.Servo.RunECCommandGetOutput(ctx, cmd, matchList)
			if err != nil {
				return errors.Wrapf(err, "failed to run cmd %v, got error", cmd)
			}
			enableStatus, err := strconv.Atoi(out[0].([]interface{})[1].(string))
			if err != nil {
				return errors.Wrap(err, "failed to parse usb port state to int value")
			} else if enableStatus != expectedStatus {
				unexpectedStatus[pin.Name] = enableStatus
			}
		}
	} else {
		// Check possible unamed pins.
		portsToCheck := h.Config.USBAPortCount
		if portsToCheck < 0 {
			// If the value is -1, we want to check a bunch of numbers manually.
			portsToCheck = 11
		}
		// If h.Config.USBAPortCount == 0, loop will not run.
		// If h.Config.USBAPortCount > 0, we expect that many ports to exist.
		// If h.Config.USBAPortCount < 0, we probe many to see if any exist.
		for i := 1; i <= portsToCheck; i++ {
			name := fmt.Sprintf("USB%d_ENABLE", i)
			cmd := fmt.Sprintf("gpioget %s", name)
			reFoundPort := regexp.MustCompile(fmt.Sprintf(`(0|1)[^\n\r]*\s%s`, name))
			reNotFoundPort := regexp.MustCompile(`Parameter\s+(\d+)\s+invalid`)
			matchList := []string{"(" + reFoundPort.String() + "|" + reNotFoundPort.String() + ")"}
			out, err := h.Servo.RunECCommandGetOutput(ctx, cmd, matchList)
			if err != nil {
				return errors.Wrap(err, "unexpected output when checking usb ports")
			} else if match := reFoundPort.FindStringSubmatch(out[0].([]interface{})[0].(string)); match != nil {
				testing.ContextLogf(ctx, "Found usb: %v at %v", name, match[1])
				enableStatus, err := strconv.Atoi(match[1])
				if err != nil {
					return errors.Wrap(err, "failed to parse usb port state to int value")
				} else if enableStatus != expectedStatus {
					unexpectedStatus[name] = enableStatus
				}
			} else if h.Config.USBAPortCount > 0 { // If port i doesn't exist (regex fails) but it is expected to (0 < h.Config.USBAPortCount >= i), raise an error.
				return errors.Errorf("explicit port count is %d; expected port %d to exist but it does not", h.Config.USBAPortCount, i)
			}
		}
	}

	if len(unexpectedStatus) != 0 {
		failStr := fmt.Sprintf("The following USB Ports didn't have state %d", expectedStatus)
		for name, state := range unexpectedStatus {
			failStr += fmt.Sprintf(", %q had status %d", name, state)
		}
		return errors.New(failStr)
	}
	return nil
}
