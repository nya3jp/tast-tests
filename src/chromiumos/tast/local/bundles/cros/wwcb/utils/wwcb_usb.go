// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// UsbController to record USB devices
type UsbController struct {
	systemUsb []string
	dockUsb   []string
	inputUsb  []string
}

// NewUsbController creates object to record system & dock USB devices count then verify it while is connected or not.
func NewUsbController(ctx context.Context, dockingID string, usbsID []string) (*UsbController, error) {
	testing.ContextLog(ctx, "Starting create USB controller")

	// Get system USB devices.
	systemUsbDevices, err := usbDevices(ctx)
	if err != nil {
		return nil, err
	}

	// Get dock USB devices.
	var dockUsbDevices []string
	if err := testing.Poll(ctx, func(c context.Context) error {
		// Plug in dock.
		if err := SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
			return err
		}

		// Delay time for Chromebook detect dock.
		testing.Sleep(ctx, 10*time.Second)

		// Get dock USB count.
		dockUsbDevices, err = usbDevices(ctx)
		if err != nil {
			return err
		}
		if len(systemUsbDevices) >= len(dockUsbDevices) {
			return errors.Errorf("unexpected USB devices count; system got %d, dock got %d", len(systemUsbDevices), len(dockUsbDevices))
		}

		if err := SwitchFixture(ctx, dockingID, "off", "0"); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Second * 30, Interval: 200 * time.Millisecond}); err != nil {
		return nil, err
	}

	testing.ContextLogf(ctx, "Usb controller created: system got %d, dock got %d", len(systemUsbDevices), len(dockUsbDevices))
	return &UsbController{
		systemUsb: systemUsbDevices,
		dockUsb:   dockUsbDevices,
		inputUsb:  usbsID,
	}, nil
}

// usbDevices returns a list of USB devices.
func usbDevices(ctx context.Context) ([]string, error) {
	lsusbOut, err := testexec.CommandContext(ctx, "lsusb").Output()
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSpace(string(lsusbOut)), "\n"), nil
}

// VerifyUsbCount verifies current USB count is as expected when is connected or not
func (uc *UsbController) VerifyUsbCount(ctx context.Context, isConnect bool) error {
	systemCount := len(uc.systemUsb)
	dockCount := len(uc.dockUsb)
	inputCount := len(uc.inputUsb)
	return testing.Poll(ctx, func(c context.Context) error {
		// get current USB count
		usbDevices, err := usbDevices(ctx)
		if err != nil {
			return err
		}
		currentCount := len(usbDevices)

		// verify USB devices count
		if isConnect { // USB devices are connected
			difference := currentCount - dockCount
			if difference != inputCount {
				return errors.Errorf("unexpected USB devices count: system got %d, dock got %d, current is %d:, input is %d", systemCount, dockCount, currentCount, inputCount)
			}
		} else { // USB devices are disconnect
			// 1. USB & station disconnect
			// 2. USB disconnect and station connect
			if currentCount > dockCount {
				return errors.Errorf("currenct USB devices count should less then dock: system is %d, dock is %d, current is %d", systemCount, dockCount, currentCount)
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second, Interval: time.Second})
}
