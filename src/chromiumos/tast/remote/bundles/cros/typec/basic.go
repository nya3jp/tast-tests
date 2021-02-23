// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"bytes"
	"context"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// Bitmasks for pin assignments C and D in the DisplayPort (DP) alternate mode Vendor-Defined Object (VDO) response.
const (
	pinCBitMask = 0x400
	pinDBitMask = 0x800
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Basic,
		Desc:         "Checks basic typec kernel driver functionality",
		Contacts:     []string{"pmalani@chromium.org", "chromeos-power@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.Model("drawlat")),
		Vars:         []string{"servo"},
	})
}

// Basic does the following:
// - Simulate a servo disconnect.
// - Reconfigure the servo as DP device supporting pin assignment C.
// - Reconnect the servo.
// - Verify that the kernel recognizes the servo partner and can parse its DP VDO data.
//
// It then repeats the process with the servo configured as a pin assignment D DP device.
//
// Since it's not possible to verify that the DUT detected a disconnect (since the DUT loses its
// network connection during servo disconnect), we check the DUT uptime before and after the servo
// configuration. If the end time is greater than the start time, we can infer that the partner
// detected was due to a hotplug and not at reboot (since the partner PD data gets parsed only once
// on each connect).
func Basic(ctx context.Context, s *testing.State) {
	d := s.DUT()
	if !d.Connected(ctx) {
		s.Fatal("Failed DUT connection check at the beginning")
	}

	s.Log("Rebooting the DUT")
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	if !d.Connected(ctx) {
		s.Fatal("Failed to connect to DUT post reboot")
	}

	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	// Configure Servo to be OK with CC being off.
	// TODO(b/<add_bug>): ccd_keepalive_en set commands don't always "take" so we have to repeat it..
	if err := pxy.Servo().SetString(ctx, servo.CCDKeepaliveEn, string(servo.Off)); err != nil {
		s.Fatal("Failed to disable CCD keepalive: ", err)
	}

	if err := pxy.Servo().SetString(ctx, servo.CCDKeepaliveEn, string(servo.Off)); err != nil {
		s.Fatal("Failed to disable CCD keepalive: ", err)
	}
	defer pxy.Servo().SetString(ctx, servo.CCDKeepaliveEn, string(servo.On))

	if err := pxy.Servo().WatchdogRemove(ctx, servo.WatchdogCCD); err != nil {
		s.Fatal("Failed to switch CCD watchdog off: ", err)
	}
	defer pxy.Servo().WatchdogAdd(ctx, servo.WatchdogCCD)

	// Servo DTS mode needs to be off to configure enable DP alternate mode support.
	if err := pxy.Servo().SetString(ctx, servo.DTSMode, string(servo.Off)); err != nil {
		s.Fatal("Failed to disable Servo DTS mode: ", err)
	}
	defer pxy.Servo().SetString(ctx, servo.DTSMode, string(servo.On))

	// Make sure that CC is switched on at the end of the test.
	defer pxy.Servo().SetCC(ctx, servo.On)

	s.Log("Checking DP pin C")
	if err := runDPTest(ctx, pxy, d, s, "c"); err != nil {
		s.Fatal("DP pin C check failed: ", err)
	}

	s.Log("Checking DP pin D")
	if err := runDPTest(ctx, pxy, d, s, "d"); err != nil {
		s.Fatal("DP pin D check failed: ", err)
	}
}

// runDPTest performs the DP alternate mode detection test for a specified pin assignment.
// Returns nil on success, otherwise the error message.
func runDPTest(ctx context.Context, pxy *servo.Proxy, d *dut.DUT, s *testing.State, pinAssign string) error {
	startTime, err := getUpTime(ctx, d)
	if err != nil {
		return errors.Wrap(err, "failed to get DUT uptime")
	}

	s.Log("Simulating servo disconnect")
	if err := pxy.Servo().SetCC(ctx, servo.Off); err != nil {
		return errors.Wrap(err, "failed to switch off CC")
	}

	s.Log("Configuring Servo to enable DP")
	if err := setServoDPMode(ctx, pxy, pinAssign); err != nil {
		return errors.Wrap(err, "failed to configure servo for DP")
	}

	s.Log("Simulating servo reconnect")
	if err := pxy.Servo().SetCC(ctx, servo.On); err != nil {
		return errors.Wrap(err, "failed to switch on CC")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return d.Connect(ctx)
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to connect to DUT")
	}

	endTime, err := getUpTime(ctx, d)
	if err != nil {
		return errors.Wrap(err, "failed to get DUT uptime")
	}

	// Check if we might have undergone a reboot.
	if endTime < startTime {
		return errors.New("uptime after reconnect is lower than before it; suggests reboot")
	}

	// Check that the partner DP alternate mode is found.
	if err := checkForDPAltMode(ctx, d, s, pinAssign); err != nil {
		return errors.Wrap(err, "failed to find the expected partner")
	}

	return nil
}

// getUpTime is a utility function that returns the seconds value of "/proc/uptime"
// from the DUT, else -1 along with an error message in case of an error.
func getUpTime(ctx context.Context, d *dut.DUT) (int, error) {
	out, err := d.Command("cat", "/proc/uptime").Output(ctx)
	if err != nil {
		return -1, errors.Wrap(err, "could not run cat /proc/uptime on the DUT")
	}

	// The first float constitutes time since power on.
	timeVal := -1
	re := regexp.MustCompile(`\d+\.\d+`)
	timeStr := re.FindString(string(out))
	if timeStr != "" {
		f, err := strconv.ParseFloat(timeStr, 64)
		if err != nil {
			return timeVal, errors.Wrap(err, "coudn't parse uptime float value")
		}
		timeVal = int(f)
	}

	return timeVal, nil
}

// setServoDPMode runs some servo console commands to configure the servo to advertise
// DP alternate mode support with the selected pin assignment setting.
func setServoDPMode(ctx context.Context, pxy *servo.Proxy, pinAssign string) error {
	if err := pxy.Servo().RunUsbcDPConfigCommand(ctx, "disable"); err != nil {
		return errors.Wrap(err, "failed to disable DP support")
	}

	if err := pxy.Servo().RunUsbcDPConfigCommand(ctx, "pins", pinAssign); err != nil {
		return errors.Wrap(err, "failed to set DP pin assignment")
	}

	if err := pxy.Servo().RunUsbcDPConfigCommand(ctx, "mf", "0"); err != nil {
		return errors.Wrap(err, "failed to set DP multi-function")
	}

	if err := pxy.Servo().RunUsbcDPConfigCommand(ctx, "enable"); err != nil {
		return errors.Wrap(err, "failed to enable DP support")
	}

	return nil
}

// checkForDPAltMode verifies that a partner was enumerated with the expected DP altmode with the
// selected pin assignment setting.
func checkForDPAltMode(ctx context.Context, d *dut.DUT, s *testing.State, pinAssign string) error {
	// Servo is always on port 0.
	partnerPath := "/sys/class/typec/port0-partner"
	out, err := d.Command("ls", partnerPath).Output(ctx)
	if err != nil {
		return errors.Wrap(err, "could not run ls command on DUT")
	}

	for _, device := range strings.Split(string(out), "\n") {
		// We're only interested in the alternate mode devices.
		if matched, err := regexp.MatchString(`port0-partner\.\d`, device); err != nil {
			return errors.Wrap(err, "couldn't run regex")
		} else if !matched {
			continue
		}

		modePath := filepath.Join(partnerPath, device)

		svidPath := filepath.Join(modePath, "svid")
		if out, err := d.Command("cat", svidPath).Output(ctx); err != nil {
			return errors.Wrap(err, "couldn't read alt mode svid")
		} else if !bytes.Contains(out, []byte("ff01")) {
			continue
		}

		vdoPath := filepath.Join(modePath, "vdo")
		out, err := d.Command("cat", vdoPath).Output(ctx)
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

	return errors.New("didn't find the right DP alternate mode registered for partner")
}
