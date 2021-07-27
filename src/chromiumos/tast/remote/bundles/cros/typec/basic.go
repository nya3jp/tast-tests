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

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// Bitmasks for pin assignments C and D in the DisplayPort (DP) alternate mode Vendor-Defined Object (VDO) response.
const (
	pinCBitMask = 0x400
	pinDBitMask = 0x800
)

// Filepath on the DUT for the servo Type C partner device.
const partnerPath = "/sys/class/typec/port0-partner"

func init() {
	testing.AddTest(&testing.Test{
		Func:     Basic,
		Desc:     "Checks basic typec kernel driver functionality",
		Contacts: []string{"pmalani@chromium.org", "chromeos-power@google.com"},
		Attr:     []string{"group:mainline", "group:typec"},
		// TODO(b/184925712): Switch this to rely on SoftwareDeps (for TCPMv2 and kernel >= v5.4) rather
		// than relying on platform HardwareDeps.
		HardwareDeps: hwdep.D(hwdep.Platform("dedede", "trogdor", "volteer")),
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
// network connection during servo disconnect), we check the DUT uptime before and after the
// test. If the end time is greater than the start time, we can infer that the partner
// detected was due to a hotplug and not at reboot (since the partner PD data gets parsed only once
// on each connect).
func Basic(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	d := s.DUT()
	if !d.Connected(ctx) {
		s.Fatal("Failed DUT connection check at the beginning")
	}

	startTime, err := getUpTime(ctx, d)
	if err != nil {
		s.Fatal("Failed to get DUT uptime: ", err)
	} else if startTime == 0 {
		s.Fatal("DUT didn't return a valid uptime")
	}

	servoSpec, _ := s.Var("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctxForCleanUp)

	// Configure Servo to be OK with CC being off.
	// TODO(pmalani): ccd_keepalive_en set commands don't always "take" so we have to repeat it.
	svo := pxy.Servo()
	if err := svo.SetOnOff(ctx, servo.CCDKeepaliveEn, servo.Off); err != nil {
		s.Fatal("Failed to disable CCD keepalive: ", err)
	}

	if err := svo.SetOnOff(ctx, servo.CCDKeepaliveEn, servo.Off); err != nil {
		s.Fatal("Failed to disable CCD keepalive: ", err)
	}
	defer func() {
		if err := svo.SetOnOff(ctxForCleanUp, servo.CCDKeepaliveEn, servo.On); err != nil {
			s.Error("Unable to enable CCD keepalive: ", err)
		}
	}()

	if err := svo.WatchdogRemove(ctx, servo.WatchdogCCD); err != nil {
		s.Fatal("Failed to switch CCD watchdog off: ", err)
	}
	defer func() {
		if err := svo.WatchdogAdd(ctxForCleanUp, servo.WatchdogCCD); err != nil {
			s.Error("Unable to switch CCD watchdog on: ", err)
		}
	}()

	// Servo DTS mode needs to be off to configure enable DP alternate mode support.
	if err := svo.SetOnOff(ctx, servo.DTSMode, servo.Off); err != nil {
		s.Fatal("Failed to disable Servo DTS mode: ", err)
	}
	defer func() {
		if err := svo.SetOnOff(ctxForCleanUp, servo.DTSMode, servo.On); err != nil {
			s.Error("Unable to enable Servo DTS mode: ", err)
		}
	}()

	// Make sure that CC is switched on at the end of the test.
	defer func() {
		if err := svo.SetCC(ctxForCleanUp, servo.On); err != nil {
			s.Error("Unable to enable Servo CC: ", err)
		}
	}()

	s.Log("Checking DP pin C")
	if err := runDPTest(ctx, svo, d, s, "c"); err != nil {
		s.Fatal("DP pin C check failed: ", err)
	}

	s.Log("Checking DP pin D")
	if err := runDPTest(ctx, svo, d, s, "d"); err != nil {
		s.Fatal("DP pin D check failed: ", err)
	}

	endTime, err := getUpTime(ctx, d)
	if err != nil {
		s.Fatal("Failed to get DUT uptime: ", err)
	}

	// Check if we might have undergone a reboot.
	if endTime < startTime {
		s.Fatalf("End uptime (%d) lower than start uptime (%d); suggests unexpected reboot", endTime, startTime)
	} else if endTime == 0 {
		s.Fatal("DUT didn't return a valid uptime")
	}
}

// runDPTest performs the DP alternate mode detection test for a specified pin assignment.
// Returns nil on success, otherwise the error message.
func runDPTest(ctx context.Context, svo *servo.Servo, d *dut.DUT, s *testing.State, pinAssign string) error {

	s.Log("Simulating servo disconnect")
	if err := svo.SetCC(ctx, servo.Off); err != nil {
		return errors.Wrap(err, "failed to switch off CC")
	}

	s.Log("Configuring Servo to enable DP")
	if err := setServoDPMode(ctx, svo, pinAssign); err != nil {
		return errors.Wrap(err, "failed to configure servo for DP")
	}

	s.Log("Simulating servo reconnect")
	if err := svo.SetCC(ctx, servo.On); err != nil {
		return errors.Wrap(err, "failed to switch on CC")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return d.Connect(ctx)
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to connect to DUT")
	}

	// Check that the partner DP alternate mode is found.
	if err := checkForDPAltMode(ctx, d, s, pinAssign); err != nil {
		return errors.Wrap(err, "failed to find the expected partner")
	}

	return nil
}

// getUpTime is a utility function that returns the seconds value of "/proc/uptime"
// from the DUT, else 0 along with an error message in case of an error.
func getUpTime(ctx context.Context, d *dut.DUT) (int, error) {
	out, err := linuxssh.ReadFile(ctx, d.Conn(), "/proc/uptime")
	if err != nil {
		return 0, errors.Wrap(err, "could not run cat /proc/uptime on the DUT")
	}

	// The first float constitutes time since power on.
	re := regexp.MustCompile(`\d+\.\d+`)
	timeStr := re.FindString(string(out))
	if timeStr != "" {
		f, err := strconv.ParseFloat(timeStr, 64)
		if err != nil {
			return 0, errors.Wrap(err, "coudn't parse uptime float value")
		}
		return int(f), nil
	}

	return 0, errors.New("couldn't find a valid uptime")
}

// setServoDPMode runs some servo console commands to configure the servo to advertise
// DP alternate mode support with the selected pin assignment setting.
func setServoDPMode(ctx context.Context, svo *servo.Servo, pinAssign string) error {
	if err := svo.RunUSBCDPConfigCommand(ctx, "disable"); err != nil {
		return errors.Wrap(err, "failed to disable DP support")
	}

	if err := svo.RunUSBCDPConfigCommand(ctx, "pins", pinAssign); err != nil {
		return errors.Wrap(err, "failed to set DP pin assignment")
	}

	if err := svo.RunUSBCDPConfigCommand(ctx, "mf", "0"); err != nil {
		return errors.Wrap(err, "failed to set DP multi-function")
	}

	if err := svo.RunUSBCDPConfigCommand(ctx, "enable"); err != nil {
		return errors.Wrap(err, "failed to enable DP support")
	}

	return nil
}

// checkForDPAltMode verifies that a partner was enumerated with the expected DP altmode with the
// selected pin assignment setting.
func checkForDPAltMode(ctx context.Context, d *dut.DUT, s *testing.State, pinAssign string) error {
	// Servo is always on port 0.
	out, err := d.Command("ls", partnerPath).Output(ctx)
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
