// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package typecutils contains constants & helper functions used by the tests in the typec directory.
package typecutils

import (
	"bytes"
	"context"
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
