// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"path/filepath"
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
)

const (
	dutReconnectionTimeout       = 30
	firstSuspendDuration         = 10
	secondSuspendDuration        = 60
	keyboardDisconnectRetryCount = 5
	keyboardConnectionDelay      = 10
	keyboardAvailabilityTimeout  = 10
	keyboardReconnectionTimeout  = 60
	inputWakeSourceRegex         = `Wakeup\s+type\:\s+input`
	otherWakeSourceRegex         = `Wakeup\s+type\:\s+other`
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HidWake,
		Desc:         "Checks that HID events correctly wake the DUT",
		Contacts:     []string{"jthies@google.com", "chromeos-power@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"usb_hid_wake"},
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
// - Disables the servo keyboard emulator
func HidWake(ctx context.Context, s *testing.State) {
	// Test setup
	d := s.DUT()
	servoSpec, _ := s.Var("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to setup servo proxy: ", err)
	}
	svo := pxy.Servo()

	// Connect RPC service on the DUT
	cl, err := rpc.Dial(ctx, d, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	// Turn on the servo keyboard emulator
	if err := svo.SetOnOff(ctx, servo.USBKeyboard, servo.On); err != nil {
		s.Fatal("Failed to enable the servo keyboard emulator: ", err)
	}

	// Turn off the servo keyboard emulator at the end of the test
	defer func() {
		if err := svo.SetOnOff(ctx, servo.USBKeyboard, servo.Off); err != nil {
			s.Fatal("Failed to disable the servo keyboard emulator: ", err)
		}
	}()

	// Polling getServoKeyboardDir to wait for keyboard availability
	dir := ""
	fs := dutfs.NewClient(cl.Conn)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		dir, err = getServoKeyboardDir(ctx, fs)
		return err
	}, &testing.PollOptions{Timeout: keyboardAvailabilityTimeout * time.Second}); err != nil {
		s.Fatal("Timed out while waiting for servo keyboard: ", err)
	}

	// Make the servo keyboard wake incapable
	if err := setServoKeyboardRemoteWakeup(ctx, fs, dir, false); err != nil {
		s.Fatal("Failed to disable servo keyboard wake capability: ", err)
	}

	// Suspend/Resume the DUT and check the wake source
	if err := attemptSuspendAndWake(ctx, d, firstSuspendDuration, otherWakeSourceRegex, pxy, fs, dir); err != nil {
		s.Fatal("Failed during first suspend: ", err)
	}

	// Reconnect RPC service on the DUT. It may disconnect during suspend/resume.
	cl, err = rpc.Dial(ctx, d, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	fs = dutfs.NewClient(cl.Conn)

	// Make the servo keyboard wake capable
	if err := setServoKeyboardRemoteWakeup(ctx, fs, dir, true); err != nil {
		s.Fatal("Failed to disable servo keyboard wake capability: ", err)
	}

	// Get initial wake count
	count1, err := getServoKeyboardWakeCount(ctx, fs, dir)
	if err != nil {
		s.Fatal("Failed to get wake count for servo keyboard emulator: ", err)
	}

	// Suspend/Resume the DUT and check the wake source
	if err := attemptSuspendAndWake(ctx, d, secondSuspendDuration, inputWakeSourceRegex, pxy, fs, dir); err != nil {
		s.Fatal("Failed during second suspend: ", err)
	}

	// Reconnect RPC service on the DUT. It may disconnect during suspend/resume.
	cl, err = rpc.Dial(ctx, d, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	fs = dutfs.NewClient(cl.Conn)

	// Get updated wake count
	count2, err := getServoKeyboardWakeCount(ctx, fs, dir)
	if err != nil {
		s.Fatal("Failed to get wake count for servo keyboard emulator: ", err)
	}

	// Compare initial and updated wake count for the servo keyboard
	if count2 <= count1 {
		s.Fatal("Servo keyboard wake count did not increase during test. wake count: ", count2)
	}
}

// attemptSuspendAndWake suspends the DUT, tries to wake it with a servo key press, then checks the output of powerd_dbus_suspend for a particular wake source.
// If there is an error during suspend/resume or the wake source does not match wakeSourceRegex, attemptSuspendAndWake will return an error.
// If the keyboard emulator disconnects and reconnects during suspend/resume, it will retry up to keyboardDisconnectRetryCount times.
func attemptSuspendAndWake(ctx context.Context, d *dut.DUT, suspendDuration int, wakeSourceRegex string, pxy *servo.Proxy, fs *dutfs.Client, servoKeyboardDir string) error {

	attemptCount := 0
	for attemptCount < keyboardDisconnectRetryCount {

		// Confirm servo keyboard connection time exceeds keyboardConnectionDelay
		baseConnectedDuration, err := getServoKeyboardConnectedDuration(ctx, fs, servoKeyboardDir)
		if baseConnectedDuration < keyboardConnectionDelay*1000 {
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				baseConnectedDuration, err = getServoKeyboardConnectedDuration(ctx, fs, servoKeyboardDir)
				if baseConnectedDuration > keyboardConnectionDelay*1000 {
					return nil
				}
				return errors.Wrap(err, "Waiting for baseConnectedDuration to reach keyboardConnectionDelay")
			}, &testing.PollOptions{Timeout: keyboardReconnectionTimeout * time.Second}); err != nil {
				return errors.Wrap(err, "Servo keyboard connected_duration was unable to reach keyboardConnectionDelay")
			}
		}

		// Create channel
		suspendErr := make(chan error, 1)
		defer close(suspendErr)

		// Suspend the DUT in go routine, and return once an error is found or powerd_dbus_suspend output is checked
		go func(ctx context.Context, d *dut.DUT, suspendDuration int, wakeSourceRegex string, suspendErr chan error) {
			out, err := d.Conn().CommandContext(ctx, "powerd_dbus_suspend", "--print_wakeup_type", "--suspend_for_sec="+strconv.Itoa(suspendDuration)).CombinedOutput()
			if err != nil {
				suspendErr <- errors.Wrap(err, "Unable to suspend the DUT")
				return
			}

			if match, err := regexp.MatchString(wakeSourceRegex, string(out)); err != nil {
				suspendErr <- errors.Wrap(err, "Unable to check wake source")
				return
			} else if !match {
				suspendErr <- errors.Wrap(err, "Wake source did not match "+wakeSourceRegex)
				return
			}

			suspendErr <- nil
		}(ctx, d, suspendDuration, wakeSourceRegex, suspendErr)

		// Wait for DUT to suspend
		if err := d.WaitUnreachable(ctx); err != nil {
			return errors.Wrap(err, "unable to verify DUT is unreachable")
		}

		// Send HID event from servo keyboard
		if err := pxy.Servo().KeypressWithDuration(ctx, servo.USBEnter, servo.DurPress); err != nil {
			return errors.Wrap(err, "unable to send key press press from servo")
		}

		// Wait for the DUT to resume
		err = <-suspendErr

		// Check ConnectedDuration
		currConnectedDuration, reconnectErr := getServoKeyboardConnectedDuration(ctx, fs, servoKeyboardDir)
		if reconnectErr != nil {
			return errors.Wrap(err, "unable to check for servo keyboard connected_duration")
		}

		// If the current connected_duration is greater than the initial connected_duration, there was no disconnection
		// Return valid suspendErr result
		if currConnectedDuration > baseConnectedDuration {
			return err
		}

		// Otherwise, the servo keyboard disconnected and reconnected during the test
		// Increment attemptCount and repeat suspend/resume if limit has not been reached
		attemptCount = attemptCount + 1
	}

	// keyboardDisconnectRetryCount has been reached
	return errors.New("attemptSuspendAndWake retry limit has been reached")
}

// getServoKeyboardDir returns the linux device directory for the servo keyboard emulator on the DUT.
// This directory includes the files to update wake capability and read wakeup count.
func getServoKeyboardDir(ctx context.Context, fs *dutfs.Client) (string, error) {
	const usbDeviceDir = "/sys/bus/usb/devices/"
	const servoKeyboardVid = "03eb"
	const servoKeyboardPid = "2042"

	usbDevices, err := fs.ReadDir(ctx, usbDeviceDir)
	if err != nil {
		return "", errors.Wrap(err, "unable to read usb device directory")
	}

	for _, device := range usbDevices {
		if vid, err := fs.ReadFile(ctx, filepath.Join(usbDeviceDir, device.Name(), "idVendor")); err != nil || strings.TrimSpace(string(vid)) != servoKeyboardVid {
			continue
		}

		if pid, err := fs.ReadFile(ctx, filepath.Join(usbDeviceDir, device.Name(), "idProduct")); err != nil || strings.TrimSpace(string(pid)) != servoKeyboardPid {
			continue
		}

		return filepath.Join(usbDeviceDir, device.Name()), nil
	}

	return "", errors.Wrap(err, "Unable to find servo keyboard path")
}

// getServoKeyboardWakeCount returns an integer with the number of times the servo keyboard has woken up the DUT.
// This is used to confirm HID events from the servo keyboard is the source of the DUT's power state change.
func getServoKeyboardWakeCount(ctx context.Context, fs *dutfs.Client, servoKeyboardDir string) (int, error) {
	out, err := fs.ReadFile(ctx, filepath.Join(servoKeyboardDir, "power/wakeup_count"))
	if err != nil {
		return -1, errors.Wrap(err, "could not read servo keyboard wakeup_count on DUT")
	}

	return strconv.Atoi(strings.TrimSpace(string(out)))
}

// setServoKeyboardRemoteWakeup writes to the power/wakeup file for a USB device to set the remote wakeup property
func setServoKeyboardRemoteWakeup(ctx context.Context, fs *dutfs.Client, servoKeyboardDir string, enable bool) error {
	remoteWake := "disabled"
	if enable {
		remoteWake = "enabled"
	}

	return fs.WriteFile(ctx, filepath.Join(servoKeyboardDir, "power/wakeup"), []byte(remoteWake), 0644)
}

// getServoKeyboardConnectedDuration returns the time in miliseconds that the servo keyboard has been connected
func getServoKeyboardConnectedDuration(ctx context.Context, fs *dutfs.Client, servoKeyboardDir string) (int, error) {
	out, err := fs.ReadFile(ctx, filepath.Join(servoKeyboardDir, "power/connected_duration"))
	if err != nil {
		return -1, errors.Wrap(err, "could not read servo keyboard connection_duration on DUT")
	}

	return strconv.Atoi(strings.TrimSpace(string(out)))
}
