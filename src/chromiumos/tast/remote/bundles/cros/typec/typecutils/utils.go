// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package typecutils contains constants & helper functions used by the tests in the typec directory.
package typecutils

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

// Wait for VBUS to discharge in Servo.
// We do this after a CC off, so that a subsequent CC on (either directly or implicitly through src dts changes) doesn't
// confuse certain TCPCs, causing Cr50 UART to be wedged on servo.
// Request = tRequest(24ms) + tSenderResponse(24ms) + tSrcTransition(35ms) + PSTransitionTimer(550ms) = 633ms (rounded up to 650ms)
// Hard Reset = 650ms + tSrcRecover(1s) + tSrcTurnOn(275ms) = 1925ms
// Total time = 1925ms * 1.5 (50% buffer) = 2887ms (rounded up to 3s)
const (
	tVBusDischargeCcOff = 3 * time.Second
)

// Bitmasks for pin assignments C and D in the DisplayPort (DP) alternate mode Vendor-Defined Object (VDO) response.
const (
	pinCBitMask = 0x400
	pinDBitMask = 0x800
)

// Filepath on the DUT for the servo Type C partner device.
const partnerPath = "/sys/class/typec/port0-partner"

// CcOffAndWait performs a CC Off command, followed by a sleep to ensure VBus discharges safely before any further modification.
func CcOffAndWait(ctx context.Context, svo *servo.Servo) error {
	if err := svo.SetCC(ctx, servo.Off); err != nil {
		return errors.Wrap(err, "failed to switch off CC")
	}

	if err := testing.Sleep(ctx, tVBusDischargeCcOff); err != nil {
		return errors.Wrap(err, "failed to sleep after CC off")
	}

	return nil
}

// CheckForDPAltMode verifies that a partner was enumerated with the expected DP altmode with the
// selected pin assignment setting(if provided).
func CheckForDPAltMode(ctx context.Context, d *dut.DUT, s *testing.State, pinAssign string) error {
	// Servo is always on port 0.
	out, err := d.Conn().CommandContext(ctx, "ls", partnerPath).Output()
	if err != nil {
		return errors.Wrap(err, "could not run ls command on DUT")
	}

	altModeDevice := regexp.MustCompile(`port0-partner\.\d`)
	for _, device := range bytes.Split(out, []byte("\n")) {
		// We're only interested in the alternate mode devices.
		if !altModeDevice.Match(device) {
			continue
		}

		modePath := filepath.Join(partnerPath, string(device))

		// Check that the alt mode has the DP SVID: 0xff01.
		svidPath := filepath.Join(modePath, "svid")
		if out, err := linuxssh.ReadFile(ctx, d.Conn(), svidPath); err != nil {
			return errors.Wrap(err, "couldn't read alt mode svid")
		} else if !bytes.Contains(out, []byte("ff01")) {
			continue
		}

		// If we aren't interested in the Pin Assignment, return immediately.
		if pinAssign == "" {
			return nil
		}

		// Read the alt mode's VDO to determine the advertised pin assignment.
		vdoPath := filepath.Join(modePath, "vdo")
		out, err := linuxssh.ReadFile(ctx, d.Conn(), vdoPath)
		if err != nil {
			return errors.Wrap(err, "couldn't read alt mode vdo")
		}

		vdoVal, err := strconv.ParseInt(strings.TrimSpace(string(out)), 0, 64)
		if err != nil {
			errors.Wrap(err, "couldn't parse VDO content of alt mode into int")
		}

		if (pinAssign == "c" && vdoVal&pinCBitMask != 0) ||
			(pinAssign == "d" && vdoVal&pinDBitMask != 0) {
			return nil
		}
	}

	return errors.Errorf("didn't find the right DP alternate mode registered for partner for pin assignment %s", pinAssign)
}

// IsDeviceEnumerated validates device enumeration in DUT.
// device holds the device name of connected TBT/USB4 device.
// port holds the TBT/USB4 port ID in DUT.
func IsDeviceEnumerated(ctx context.Context, dut *dut.DUT, device, port string) (bool, error) {
	deviceNameFile := fmt.Sprintf("/sys/bus/thunderbolt/devices/%s/device_name", port)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := linuxssh.ReadFile(ctx, dut.Conn(), deviceNameFile)
		if err != nil {
			return errors.Wrapf(err, "failed to read %q file", deviceNameFile)
		}

		if strings.TrimSpace(string(out)) != device {
			return errors.New("device enumeration failed")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 1 * time.Second}); err != nil {
		return false, err
	}
	return true, nil
}

// CheckUSBPdMuxinfo verifies whether USB4=1 or not.
func CheckUSBPdMuxinfo(ctx context.Context, dut *dut.DUT, deviceStr string) error {
	out, err := dut.Conn().CommandContext(ctx, "ectool", "usbpdmuxinfo").Output()
	if err != nil {
		return errors.Wrap(err, "failed to execute ectool usbpdmuxinfo command")
	}
	if !strings.Contains(string(out), deviceStr) {
		return errors.Wrapf(err, "failed to find %s in usbpdmuxinfo", deviceStr)
	}
	return nil
}

// CableConnectedPortNumber on success will returns Active/Passive cable connected port number.
func CableConnectedPortNumber(ctx context.Context, dut *dut.DUT, connector string) (string, error) {
	out, err := dut.Conn().CommandContext(ctx, "ectool", "usbpdmuxinfo").Output()
	if err != nil {
		return "", errors.Wrap(err, "failed to execute ectool usbpdmuxinfo command")
	}
	portRe := regexp.MustCompile(fmt.Sprintf(`Port.([0-9]):.*(%s=1)`, connector))
	portNum := portRe.FindStringSubmatch(string(out))
	if len(portNum) == 0 {
		return "", errors.New("failed to get port number from usbpdmuxinfo")
	}
	return portNum[1], nil
}
