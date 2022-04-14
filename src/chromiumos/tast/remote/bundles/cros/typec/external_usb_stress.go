// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// single iteration by default to stay within context deadline
const defaultIterations = 1
const longWaitDelay = 15
const waitDelay = 5

func init() {
	testing.AddTest(&testing.Test{
		Func:     ExternalUSBStress,
		Desc:     "Checks if USB changes can be detected correctly",
		Contacts: []string{"wonchung@google.com", "chromeos-usb@google.com"},
		Attr:     []string{"group:mainline", "group:typec", "informational"},
		Vars:     []string{"servo", "typec.iterations"},
	})
}

// ExternalUSBStress checks if USB device connect/disconnect is detected properly
func ExternalUSBStress(ctx context.Context, s *testing.State) {
	d := s.DUT()
	if !d.Connected(ctx) {
		s.Fatal("Failed DUT connection check at the beginning")
	}

	servoSpec, _ := s.Var("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)
	svo := pxy.Servo()

	connectUsbDevice(ctx, svo, false)
	offList, err := getUsbDevices(ctx, d)
	if err != nil {
		s.Fatal("Failed to get USB devices: ", err)
	}

	connectUsbDevice(ctx, svo, true)
	onList, err := getUsbDevices(ctx, d)
	if err != nil {
		s.Fatal("Failed to get USB devices: ", err)
	}

	usbDevices := difference(onList, offList)
	if len(usbDevices) == 0 {
		s.Fatal("No connected devices detected")
	}
	s.Log("Connected devies list: ", usbDevices)

	iterations := defaultIterations
	customIterations, ok := s.Var("typec.iterations")
	if ok {
		iterations, _ = strconv.Atoi(customIterations)
	}
	for i := 0; i < iterations; i++ {
		if err := testSuspend(ctx, svo, d, usbDevices, false, false); err != nil {
			s.Error("Failed test case: disconnect -> suspend -> disconnect -> resume: ", err)
		}
		if err := testSuspend(ctx, svo, d, usbDevices, true, false); err != nil {
			s.Error("Failed test case: connect -> suspend -> disconnect -> resume: ", err)
		}
		if err := testSuspend(ctx, svo, d, usbDevices, false, true); err != nil {
			s.Error("Failed test case: disconnect -> suspend -> connect -> resume: ", err)
		}
		if err := testSuspend(ctx, svo, d, usbDevices, true, true); err != nil {
			s.Error("Failed test case: connect -> suspend -> connect -> resume: ", err)
		}
	}
}

// testSuspend simulates USB device connect/disconnect before DUT suspend/resume,
// then returns an error if the connect/disconnect is not detected properly.
func testSuspend(ctx context.Context,
	svo *servo.Servo,
	d *dut.DUT,
	usbDevices []string,
	pluggedBeforeSuspend,
	pluggedBeforeResume bool) error {

	connectUsbDevice(ctx, svo, pluggedBeforeSuspend)
	svo.CloseLid(ctx)
	testing.Sleep(ctx, waitDelay*time.Second)

	if pluggedBeforeSuspend != pluggedBeforeResume {
		connectUsbDevice(ctx, svo, pluggedBeforeResume)
	}
	svo.OpenLid(ctx)
	testing.Sleep(ctx, waitDelay*time.Second)

	if err := d.WaitConnect(ctx); err != nil {
		return errors.Wrap(err, "failed to reconnect to DUT after resume")
	}

	if !pluggedBeforeResume {
		usbList, err := getUsbDevices(ctx, d)
		if err != nil {
			return errors.Wrap(err, "failed to get USB devices after resume")
		}
		if len(difference(usbDevices, usbList)) != len(usbDevices) {
			return errors.New("devices are not disconnected on resume")
		}
		return nil
	}

	// Wait at most 15 seconds after resume to detect USB device connection
	startTime := time.Now()
	for time.Now().Sub(startTime).Seconds() < longWaitDelay {
		usbList, err := getUsbDevices(ctx, d)
		if err != nil {
			return errors.Wrap(err, "failed to get USB devices after resume")
		}
		if len(difference(usbDevices, usbList)) == 0 {
			return nil
		}
		testing.Sleep(ctx, time.Second)
	}
	return errors.New("devices are not connected on resume")
}

// connectUsbDevice simulates USB device connect/disconnect through servo USB Mux
func connectUsbDevice(ctx context.Context, svo *servo.Servo, connect bool) {
	if connect {
		svo.SetUSBMuxState(ctx, servo.USBMuxDUT)
	} else {
		svo.SetUSBMuxState(ctx, servo.USBMuxHost)
	}
	testing.Sleep(ctx, waitDelay*time.Second)
}

// getUsbDevices returns a list of USB devices connected to DUT
func getUsbDevices(ctx context.Context, d *dut.DUT) ([]string, error) {
	var deviceNameList []string
	lsusb, err := d.Conn().CommandContext(ctx, "lsusb").Output()
	if err != nil {
		return deviceNameList, errors.Wrap(err, "failed to read lsusb output")
	}

	unnamedDeviceCount := 0
	for _, device := range strings.Split(string(lsusb), "\n") {
		var deviceName string
		columns := strings.Split(device, " ")
		if len(columns) <= 6 || len(strings.Trim(strings.Join(columns[6:], " "), " ")) == 0 {
			deviceName = fmt.Sprintf("Unnamed device %d", unnamedDeviceCount)
			unnamedDeviceCount++
		} else {
			deviceName = strings.Trim(strings.Join(columns[6:], " "), " ")
		}
		deviceNameList = append(deviceNameList, deviceName)
	}
	return deviceNameList, nil
}

// difference returns the elements in `a` that aren't in `b`.
func difference(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}
