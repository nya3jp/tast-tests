// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/typec/typecutils"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     HpdWake,
		Desc:     "Checks that Display Port HPD (Hot Plug Detect) events can wake the system",
		Contacts: []string{"pmalani@chromium.org", "chromeos-power@google.com"},
		Attr:     []string{"group:mainline", "group:typec", "informational"},
		// TODO(b/184925712): Switch this to rely on SoftwareDeps (for TCPMv2 and kernel >= v5.4) rather
		// than relying on platform HardwareDeps.
		HardwareDeps: hwdep.D(hwdep.Platform("dedede", "volteer")),
		Vars:         []string{"servo"},
	})
}

// HpdWake does the following:
// - Simulate a servo disconnect.
// - Reconfigure the servo as a DP device with HPD "low".
// - Reconnect the servo.
// - Verify that the kernel recognizes the servo partner and DP alt mode.
// - Measure the EC device wake event count.
// - Suspend the DUT.
// - Make the servo's HPD state to "high".
// - Check that the DUT woke, count the EC wake events and confirm that the wake count increased.
func HpdWake(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	d := s.DUT()
	if !d.Connected(ctx) {
		s.Fatal("Failed DUT connection check at the beginning")
	}

	servoSpec, _ := s.Var("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctxForCleanUp)

	// Configure Servo to be OK with CC being off.
	svo := pxy.Servo()
	if err := svo.SetOnOff(ctx, servo.CCDKeepaliveEn, servo.Off); err != nil {
		s.Fatal("Failed to disable CCD keepalive: ", err)
	}
	defer func() {
		if err := svo.SetOnOff(ctxForCleanUp, servo.CCDKeepaliveEn, servo.On); err != nil {
			s.Error("Unable to enable CCD keepalive: ", err)
		}
	}()

	// Wait for servo control to take effect.
	if err := testing.Sleep(ctx, time.Second); err != nil {
		s.Fatal("Failed to sleep after CCD keepalive disable: ", err)
	}

	if err := svo.WatchdogRemove(ctx, servo.WatchdogCCD); err != nil {
		s.Fatal("Failed to switch CCD watchdog off: ", err)
	}
	defer func() {
		if err := svo.WatchdogAdd(ctxForCleanUp, servo.WatchdogCCD); err != nil {
			s.Error("Unable to switch CCD watchdog on: ", err)
		}
	}()

	// Wait for servo control to take effect.
	if err := testing.Sleep(ctx, time.Second); err != nil {
		s.Fatal("Failed to sleep after CCD watchdog off: ", err)
	}

	// Make sure that CC is switched on at the end of the test.
	defer func() {
		if err := svo.SetCC(ctxForCleanUp, servo.On); err != nil {
			s.Error("Unable to enable Servo CC: ", err)
		}
	}()

	// Turn CC Off before modifying DTS Mode.
	if err := typecutils.CcOffAndWait(ctx, svo); err != nil {
		s.Fatal("Failed CC off and wait: ", err)
	}

	// Servo DTS mode needs to be off to configure enable DP alternate mode support.
	if err := svo.SetOnOff(ctx, servo.DTSMode, servo.Off); err != nil {
		s.Fatal("Failed to disable Servo DTS mode: ", err)
	}
	defer func() {
		if err := svo.SetOnOff(ctxForCleanUp, servo.DTSMode, servo.On); err != nil {
			s.Error("Unable to enable Servo DTS mode: ", err)
		}
	}()

	// Wait for DTS-off PD negotiation to complete.
	if err := testing.Sleep(ctx, 2500*time.Millisecond); err != nil {
		s.Fatal("Failed to sleep for DTS-off power negotiation: ", err)
	}

	if err := enumerateDP(ctx, svo, d, s); err != nil {
		s.Fatal("DP enumeration failed: ", err)
	}

	// Count wake sources before.
	wakesBefore, err := getWakeCount(ctx, d)
	if err != nil {
		s.Fatal("Failed to count wakeup sources: ", err)
	}

	// Suspend DUT. Run in a separate thread since this function blocks until resume.
	done := make(chan error, 1)
	go func(ctx context.Context) {
		defer close(done)
		out, err := d.Conn().CommandContext(ctx, "powerd_dbus_suspend", "--timeout=120").CombinedOutput()
		testing.ContextLog(ctx, "powerd_dbus_suspend output: ", string(out))
		done <- err
	}(ctx)

	if err := d.WaitUnreachable(ctx); err != nil {
		s.Fatal("Couldn't verify DUT become unreachable after suspend: ", err)
	}

	s.Log("Setting HPD to high")
	if err := svo.RunUSBCDPConfigCommand(ctx, "hpd", "h"); err != nil {
		s.Fatal("Failed to set HPD high: ", err)
	}

	// Verify DUT reconnected.
	if err := testing.Poll(ctx, d.Connect, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Fatal("Failed to re-connect to DUT after suspend: ", err)
	}

	// Count wake source.
	wakesAfter, err := getWakeCount(ctx, d)
	if err != nil {
		s.Fatal("Failed to count wakeup sources: ", err)
	}

	// Check the difference between wake sources.
	if wakesAfter <= wakesBefore {
		s.Fatal("Wakeup event not registered")
	}

	// We should make a note if there were other wakeup events for cros_ec.
	if wakesAfter != wakesBefore+1 {
		s.Logf("Registered more than 1 wakeup event, before:%d, after:%d", wakesBefore, wakesAfter)
	}

	// Verify that the suspend command didn't return an unexpected error.
	// TODO(b/197903975): Try to parse actual output of powerd_dbus_suspend.
	if err := <-done; err != nil {
		if !bytes.Contains([]byte(err.Error()), []byte("remote command exited without exit status")) {
			s.Fatal("Suspend command returned unexpected error: ", err)
		}
	}

	// Turn CC Off before modifying DTS Mode in cleanup.
	if err := typecutils.CcOffAndWait(ctx, svo); err != nil {
		s.Fatal("Failed CC off and wait: ", err)
	}
}

// enumerateDP configures the servo as a DP device and verifies that the DUT can detect it.
// Returns nil on success, otherwise the error message.
func enumerateDP(ctx context.Context, svo *servo.Servo, d *dut.DUT, s *testing.State) error {
	s.Log("Simulating servo disconnect")
	if err := typecutils.CcOffAndWait(ctx, svo); err != nil {
		return errors.Wrap(err, "failed CC off and wait")
	}

	if err := svo.RunUSBCDPConfigCommand(ctx, "disable"); err != nil {
		return errors.Wrap(err, "failed to disable DP support")
	}

	if err := svo.RunUSBCDPConfigCommand(ctx, "hpd", "l"); err != nil {
		return errors.Wrap(err, "failed to set DP multi-function")
	}

	if err := svo.RunUSBCDPConfigCommand(ctx, "enable"); err != nil {
		return errors.Wrap(err, "failed to enable DP support")
	}

	s.Log("Simulating servo reconnect")
	if err := svo.SetCC(ctx, servo.On); err != nil {
		return errors.Wrap(err, "failed to switch on CC")
	}

	if err := testing.Poll(ctx, d.Connect, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to connect to DUT")
	}

	// Wait for PD negotiation to stabilize.
	if err := testing.Sleep(ctx, 2500*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to sleep for PD negotiation")
	}

	s.Log("Verifying DP alt mode detection")
	if err := typecutils.CheckForDPAltMode(ctx, d, s, ""); err != nil {
		return errors.Wrap(err, "failed to find the expected partner")
	}

	return nil
}

// getWakeCount returns the number of wake ups from the Chrome OS EC device (through which HPD wakeups are passed).
func getWakeCount(ctx context.Context, d *dut.DUT) (int64, error) {
	out, err := d.Conn().CommandContext(ctx, "cat", "/sys/kernel/debug/wakeup_sources").Output()
	if err != nil {
		return -1, errors.Wrap(err, "could not cat wakeup_sources on DUT")
	}

	for _, device := range bytes.Split(out, []byte("\n")) {
		// Search for the Chrome OS EC device.
		if !bytes.Contains(device, []byte("GOOG0004")) {
			continue
		}

		// The format of /sys/kernel/wakeup_sources is always of the form:
		// name            active_count    event_count     wakeup_count ....
		//
		// So, we need to look for the 4th word in the line.
		for i, word := range strings.Fields(string(device)) {
			if i != 3 {
				continue
			}

			count, err := strconv.ParseInt(strings.TrimSpace(word), 0, 64)
			if err != nil {
				return -1, errors.Wrap(err, "couldn't parse wakeup count for EC device")
			}

			return count, nil
		}
	}

	return -1, errors.New("no Chrome OS device found in wakeup sources list")
}
