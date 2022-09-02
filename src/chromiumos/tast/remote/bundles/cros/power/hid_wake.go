// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"regexp"
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

const (
	dutReconnectionTimeout      = 30
	firstSuspendDuration        = 10
	keyboardAvailabilityTimeout = 10
	suspendTimeout              = 120
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HidWake,
		Desc:         "Checks that HID events correctly wake the DUT",
		Contacts:     []string{"jthies@google.com", "chromeos-power@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Vars:         []string{"servo"},
	})
}

// HidWake does the following:
// - Enables the servo keyboard emulator
// - Makes the servo keyboard wake incapable
// - Suspends the DUT
// - Checks a servo keypress does not wake the DUT
// - Makes the servo keyboard wake capable
// - Suspends the DUT
// - Wakes the DUT with a servo keypress
// - Checks the servo keyboard wake count has incremented
func HidWake(ctx context.Context, s *testing.State) {
	// Test setup
	d := s.DUT()
	servoSpec, _ := s.Var("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to setup servo proxy: ", err)
	}
	svo := pxy.Servo()

	// Turn on the servo keyboard emulator
	if err := svo.SetOnOff(ctx, servo.USBKeyboard, servo.On); err != nil {
		s.Fatal("Failed to enable the servo keyboard emulator: ", err)
	}

	// Polling getServoKeyboardDir to wait for keyboard availability
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := getServoKeyboardDir(ctx, d, s); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: keyboardAvailabilityTimeout * time.Second}); err != nil {
		s.Fatal("Timed out while waiting for servo keyboard: ", err)
	}

	// Find servo directory on DUT
	dir, err := getServoKeyboardDir(ctx, d, s)
	if err != nil {
		s.Fatal("Failed to find servo keyboard device location: ", err)
	}

	// Make the servo keyboard wake incapable
	cl, err := rpc.Dial(ctx, d, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	fs := dutfs.NewClient(cl.Conn)
	if err := fs.WriteFile(ctx, dir+"/power/wakeup", []byte("disabled\n"), 0644); err != nil {
		s.Fatal("Failed to disable servo keyboard wake capability: ", err)
	}

	// Suspend the DUT
	done := make(chan error, 1)
	go func(ctx context.Context) {
		defer close(done)
		out, err := d.Conn().CommandContext(ctx, "powerd_dbus_suspend", "--print_wakeup_type", "--timeout="+strconv.Itoa(suspendTimeout), "--suspend_for_sec="+strconv.Itoa(firstSuspendDuration)).CombinedOutput()
		done <- err
		if match, err := regexp.MatchString(`Wakeup\s+type\:\s+other`, string(out)); err != nil {
			s.Fatal("Unable to check wakeup source for first resume: ", err)
		} else if !match {
			s.Fatal("Incorrect wakeup source for first resume")
		}
	}(ctx)

	// Wait for DUT to suspend
	if err := d.WaitUnreachable(ctx); err != nil {
		s.Fatal("Couldn't verify DUT became unreachable after suspend: ", err)
	}

	// Send HID event from servo keyboard
	if err := pxy.Servo().KeypressWithDuration(ctx, servo.USBEnter, servo.DurPress); err != nil {
		s.Fatal("Failed to press key on servo keyboard: ", err)
	}

	// Wait for DUT to finish powerd_dbus_suspend
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return <-done
	}, &testing.PollOptions{Timeout: dutReconnectionTimeout * time.Second}); err != nil {
		s.Fatal("Failed to wake from first powerd_dbus_suspend: ", err)
	}

	// Make the servo wake capable - need to reset the DUT filesystem after suspend/resume
	cl, err = rpc.Dial(ctx, d, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	fs = dutfs.NewClient(cl.Conn)
	if err := fs.WriteFile(ctx, dir+"/power/wakeup", []byte("enabled\n"), 0644); err != nil {
		s.Fatal("Failed to enable servo keyboard wake capability: ", err)
	}

	// Get initial wake count
	count1, err := getServoKeyboardWakeCount(ctx, d, s, dir)
	if err != nil {
		s.Fatal("Failed to get wake count for servo keyboard emulator: ", err)
	}

	// Suspend the DUT
	done = make(chan error, 1)
	go func(ctx context.Context) {
		defer close(done)
		out, err := d.Conn().CommandContext(ctx, "powerd_dbus_suspend", "--print_wakeup_type", "--timeout="+strconv.Itoa(suspendTimeout)).CombinedOutput()
		done <- err
		if match, err := regexp.MatchString(`Wakeup\s+type\:\s+input`, string(out)); err != nil {
			s.Fatal("Unable to check wakeup source for second resume: ", err)
		} else if !match {
			s.Fatal("Incorrect wakeup source for second resume")
		}
	}(ctx)

	// Wait for DUT to suspend
	if err := d.WaitUnreachable(ctx); err != nil {
		s.Fatal("Couldn't verify DUT became unreachable after suspend: ", err)
	}

	// Send HID event from servo keyboard
	if err := pxy.Servo().KeypressWithDuration(ctx, servo.USBEnter, servo.DurPress); err != nil {
		s.Fatal("Failed to press key on servo keyboard: ", err)
	}

	// Wait for DUT to finish powerd_dbus_suspend
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return <-done
	}, &testing.PollOptions{Timeout: dutReconnectionTimeout * time.Second}); err != nil {
		s.Fatal("Failed to wake from second powerd_dbus_suspend: ", err)
	}

	// Get updated wake count
	count2, err := getServoKeyboardWakeCount(ctx, d, s, dir)
	if err != nil {
		s.Fatal("Failed to get wake count for servo keyboard emulator: ", err)
	}

	// Compare initial and updated wake count for the servo keyboard
	if count2 <= count1 {
		s.Fatal("Servo keyboard wake count did not increase during test")
	}
}

// getServoKeyboardDir returns the linux device directory for the servo keyboard emulator on the DUT.
// This directory includes the files to update wake capability and read wakeup count.
func getServoKeyboardDir(ctx context.Context, d *dut.DUT, s *testing.State) (string, error) {
	const usbDeviceDir = "/sys/bus/usb/devices/"
	const servoKeyboardVid = "03eb"
	const servoKeyboardPid = "2042"

	cl, err := rpc.Dial(ctx, d, s.RPCHint())
	if err != nil {
		return "", errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	fs := dutfs.NewClient(cl.Conn)
	usbDevices, err := fs.ReadDir(ctx, usbDeviceDir)
	if err != nil {
		return "", errors.Wrap(err, "unable to read usb device deirectory")
	}

	for _, device := range usbDevices {
		vid, err := fs.ReadFile(ctx, usbDeviceDir+device.Name()+"/idVendor")
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(vid)) != servoKeyboardVid {
			continue
		}

		pid, err := fs.ReadFile(ctx, usbDeviceDir+device.Name()+"/idProduct")
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(pid)) != servoKeyboardPid {
			continue
		}

		return (usbDeviceDir + device.Name()), nil
	}

	return "", errors.Wrap(err, "Unable to find servo keyboard path")
}

// getServoKeyboardWakeCount returns an integer with the number of times the servo keyboard has woken up the DUT.
// This is used to confirm HID events from the servo keyboard is the source of the DUT's power state change.
func getServoKeyboardWakeCount(ctx context.Context, d *dut.DUT, s *testing.State, servoKeyboardDir string) (int64, error) {
	cl, err := rpc.Dial(ctx, d, s.RPCHint())
	if err != nil {
		return -1, errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	fs := dutfs.NewClient(cl.Conn)

	out, err := fs.ReadFile(ctx, servoKeyboardDir+"/power/wakeup_count")
	if err != nil {
		return -1, errors.Wrap(err, "could not cat servo keyboard wakeup_count on DUT")
	}

	count, err := strconv.ParseInt(strings.TrimSpace(string(out)), 0, 64)
	if err != nil {
		return -1, errors.Wrap(err, "could not parse wakeup count for servo keyboard")
	}

	return count, nil
}
