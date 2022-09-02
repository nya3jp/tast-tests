// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/rpc"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HidWake,
		Desc:         "Checks that HID events can wake the system",
		Contacts:     []string{"jthies@google.com", "chromeos-power@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.ECFeatureTypecCmd(), hwdep.ChromeEC()),
		Vars:         []string{"servo"},
	})
}

// HidWake does the following:
// - Enables the servo keyboard emulator.
// - Makes the servo keyboard wake incapable.
// - Suspends the DUT.
// - Checks a servo keypress does not wake the DUT.
// - Makes the servo keyboard wake capable.
// - Suspends the DUT.
// - Wakes the DUT with a servo keypress.
// - Checks the servo wake count has incremented.
func HidWake(ctx context.Context, s *testing.State) {

	// Test setup
	d := s.DUT()
	servoSpec, _ := s.Var("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, d.KeyFile(), d.KeyDir())
	cl, err := rpc.Dial(ctx, d, s.RPCHint())
	fs := dutfs.NewClient(cl.Conn)
	svo := pxy.Servo()

	// Turn on the servo keyboard emulator and wait for kernel to register it.
	s.Log("Turning on servo keyboard emulator")
	if err := svo.SetOnOff(ctx, servo.USBKeyboard, servo.On); err != nil {
		s.Fatal("Failed to enable servo keybpoard emulator: ", err)
	}

	if err := testing.Sleep(ctx, 2*time.Second); err != nil {
		s.Fatal("Failed to sleep while servo keyboard emulator turns on: ", err)
	}

	// Find servo directory on DUT
	dir, err := getServoDir(ctx, d)
	if err != nil {
		s.Fatal("Failed to find servo device location: ", err)
	} else {
		s.Logf("Found servo device location at %s", dir)
	}

	// Make the servo wake incapable
	s.Log("Disabling servo wake capability")
	if err := fs.WriteFile(ctx, dir+"/power/wakeup", []byte("disabled\n"), 0644); err != nil {
		s.Fatal("Failed to disable servo wake capability: ", err)
	}

	// Suspend the DUT for 12 seconds
	s.Log("Suspending DUT for 12 seconds")
	suspend1 := make(chan error, 1)
	go func(ctx context.Context) {
		defer close(suspend1)
		out, err := d.Conn().CommandContext(ctx, "powerd_dbus_suspend", "--print_wakeup_type", "--suspend_for_sec=12").CombinedOutput()
		testing.ContextLog(ctx, "powerd_dbus_suspend output: ", string(out))
		suspend1 <- err
	}(ctx)

	// At 5 seconds, confirm DUT is suspended
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to wait for device to suspend: ", err)
	}
	if d.Connected(ctx) {
		s.Fatal("Failed to suspend DUT")
	}

	// Send HID event from servo keyboard
	s.Log("Sending HID event to DUT")
	pxy.Servo().KeypressWithDuration(ctx, servo.USBEnter, servo.DurPress)

	// At 10 seconds confirm DUT is still suspended
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to wait after HID event does not wake the DUT: ", err)
	}
	if d.Connected(ctx) {
		s.Fatal("DUT is awake after HID event which should not wake the DUT")
	} else {
		s.Log("DUT is still asleep after sending HID event from servo")
	}

	// TODO: replace with poll
	// Wait 20 seconds for DUT to wake and connect to servo
	s.Log("Waiting for DUT to wake up and connect")
	if err := testing.Sleep(ctx, 20*time.Second); err != nil {
		s.Fatal("Failed to wait for DUT to wake up: ", err)
	}
	s.Log("Checking DUT is connected")
	if !d.Connected(ctx) {
		s.Fatal("DUT is not connected when it should be")
	} else {
		s.Log("DUT is connected")
	}

	// Make the servo wake capable
	s.Log("Enabling servo wake capability")
	if err := fs.WriteFile(ctx, dir+"/power/wakeup", []byte("enabled\n"), 0644); err != nil {
		s.Fatal("Failed to enable servo wake capability: ", err)
	}

	// Get initial wake count
	count1, err := getServoWakeCount(ctx, d, dir)
	if err != nil {
		s.Fatal("Failed to get wake count for servo keyboard emulator: ", err)
	} else {
		s.Logf("Initial wake count for servo keyboard emulator: %d", count1)
	}

	// Suspend the DUT, timeout after 60 seconds
	s.Log("Suspending DUT with 60 second timeout")
	suspend2 := make(chan error, 1)
	go func(ctx context.Context) {
		defer close(suspend2)
		out, err := d.Conn().CommandContext(ctx, "powerd_dbus_suspend", "--timeout=60", "--print_wakeup_type").CombinedOutput()
		testing.ContextLog(ctx, "powerd_dbus_suspend output: ", string(out))
		suspend2 <- err
	}(ctx)

	// At 5 seconds, confirm DUT is suspended
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to wait for device to suspend: ", err)
	}
	if d.Connected(ctx) {
		s.Fatal("DUT is connected when it shouldn't be")
	}

	// Send HID event from servo keyboard
	s.Log("Sending HID event to DUT")
	pxy.Servo().KeypressWithDuration(ctx, servo.USBEnter, servo.DurPress)

	// TODO: replace with poll
	// Wait 20 seconds, confirm dut is awake and wake count incremented
	s.Log("Waiting for DUT to wake up and connect")
	if err := testing.Sleep(ctx, 20*time.Second); err != nil {
		s.Fatal("Failed to wait for DUT to wake up: ", err)
	}
	if !d.Connected(ctx) {
		s.Fatal("DUT is not connected when it should be")
	} else {
		s.Log("DUT is connected")
	}

	count2, err := getServoWakeCount(ctx, d, dir)
	if err != nil {
		s.Fatal("Failed to get wake count for servo keyboard emulator: ", err)
	} else {
		s.Logf("Updated wake count for servo keyboard emulator: %d", count2)
	}

	if count2 <= count1 {
		s.Fatal("Wake count did not increase during test")
	} else {
		s.Log("Wake count increased")
	}

	s.Log("HidWake test complete")
}

// getServoDir returns the linux device directory for the servo on the DUT.
// This includes the files to update wake capability and read wakeup count.
func getServoDir(ctx context.Context, d *dut.DUT) (string, error) {
	base := "/sys/bus/hid/devices/"

	hidLink, err := d.Conn().CommandContext(ctx, "find", base, "-name", "*\\:03EB\\:2042\\.*").Output()
	if err != nil {
		return "~", errors.Wrap(err, "failed to find any HID links matching servo VID/PID")
	}

	hidPath, err := d.Conn().CommandContext(ctx, "readlink", "-f", strings.TrimSpace(string(hidLink))).Output()
	if err != nil {
		return "~", errors.Wrap(err, "failed to read link for servo")
	}

	servoPath, err := d.Conn().CommandContext(ctx, "realpath", strings.TrimSpace(string(hidPath))+"/../../").Output()
	if err != nil {
		return "~", errors.Wrap(err, "failed to find servo device path from HID link")
	}

	vid, err := d.Conn().CommandContext(ctx, "cat", strings.TrimSpace(string(servoPath))+"/idVendor").Output()
	if err != nil {
		return "~", errors.Wrap(err, "failed to check servo VID")
	}
	if strings.TrimSpace(string(vid)) != "03eb" {
		return "~", errors.Wrap(err, "VID check for servo failed")
	}

	pid, err := d.Conn().CommandContext(ctx, "cat", strings.TrimSpace(string(servoPath))+"/idProduct").Output()
	if err != nil {
		return "~", errors.Wrap(err, "failed to check servo PID")
	}
	if strings.TrimSpace(string(pid)) != "2042" {
		return "~", errors.Wrap(err, "PID check for servo failed")
	}

	return strings.TrimSpace(string(servoPath)), nil
}

// getServoWakeCount returns an integer with the number of times the servo has woken up the DUT.
// This is used to confirm HID events from the servo are the source of the DUT's power state change.
func getServoWakeCount(ctx context.Context, d *dut.DUT, servoHidDir string) (int64, error) {

	out, err := d.Conn().CommandContext(ctx, "cat", servoHidDir+"/power/wakeup_count").Output()
	if err != nil {
		return -1, errors.Wrap(err, "could not cat servo wakeup_count on DUT")
	}

	count, err := strconv.ParseInt(strings.TrimSpace(string(out)), 0, 64)
	if err != nil {
		return -1, errors.Wrap(err, "could not parse wakeup count for servo")
	}

	return count, nil
}
