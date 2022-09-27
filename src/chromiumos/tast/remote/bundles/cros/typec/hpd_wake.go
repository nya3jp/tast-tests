// Copyright 2021 The ChromiumOS Authors
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
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/typec/fixture"
	"chromiumos/tast/remote/bundles/cros/typec/typecutils"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HpdWake,
		Desc:         "Checks that Display Port HPD (Hot Plug Detect) events can wake the system",
		Contacts:     []string{"pmalani@chromium.org", "chromeos-power@google.com"},
		Attr:         []string{"group:mainline", "group:typec", "informational"},
		HardwareDeps: hwdep.D(hwdep.ECFeatureTypecCmd(), hwdep.ChromeEC()),
		Vars:         []string{"servo"},
		Fixture:      "typeCServo",
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
	d := s.DUT()
	svo := s.FixtValue().(*fixture.Value).Servo()
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
}

// enumerateDP configures the servo as a DP device and verifies that the DUT can detect it.
// Returns nil on success, otherwise the error message.
func enumerateDP(ctx context.Context, svo *servo.Servo, d *dut.DUT, s *testing.State) error {
	s.Log("Simulating servo disconnect")
	if err := typecutils.CcOffAndWait(ctx, svo); err != nil {
		return errors.Wrap(err, "failed CC off and wait")
	}

	if err := d.Disconnect(ctx); err != nil {
		return errors.Wrap(err, "failed to close the current DUT ssh connection")
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

// getWakeCount returns the number of wake ups from the ChromeOS EC device (through which HPD wakeups are passed).
// Uses the wakeup class object attached to the chromeos class cros_ec device in sysfs
func getWakeCount(ctx context.Context, d *dut.DUT) (int64, error) {
	base := "/sys/class/chromeos/cros_ec/"

	link, err := d.Conn().CommandContext(ctx, "readlink", base+"device").Output()
	if err != nil {
		return -1, errors.Wrap(err, "could not find cros_ec device in sysfs")
	}

	ec, err := d.Conn().CommandContext(ctx, "dirname", strings.TrimSpace(string(link))).Output()
	if err != nil {
		return -1, errors.Wrap(err, "could not get dirname of cros_ec device")
	}

	path := base + strings.TrimSpace(string(ec)) + "/wakeup/"
	wakeup, err := d.Conn().CommandContext(ctx, "ls", path).Output()
	if err != nil {
		return -1, errors.Wrap(err, "could not find wakeup class for EC device")
	}

	out, err := d.Conn().CommandContext(ctx, "cat", path+strings.TrimSpace((string(wakeup)))+"/wakeup_count").Output()
	if err != nil {
		return -1, errors.Wrap(err, "could not cat cros_ec wakeup_count on DUT")
	}

	count, err := strconv.ParseInt(strings.TrimSpace(string(out)), 0, 64)
	if err != nil {
		return -1, errors.Wrap(err, "couldn't parse wakeup count for EC device")
	}

	return count, nil
}
