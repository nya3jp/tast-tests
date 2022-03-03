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

// UsbController to control usb fixture
type UsbController struct {
	systemUsb []string
	dockUsb   []string
	inputUsb  []string
}

// NewUsbController to create object to control usb fixture also record system usb count (as condition: plug in station without any usb)
func NewUsbController(ctx context.Context, dockingID string, usbsID []string) (*UsbController, error) {

	testing.ContextLog(ctx, "Starting create usb controller")

	systemUsbDevices, err := usbDevices(ctx)
	if err != nil {
		return nil, err
	}
	testing.ContextLogf(ctx, "system  %d", len(systemUsbDevices))
	if err := testing.Poll(ctx, func(c context.Context) error {
		// plug in dock
		if err := SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
			return err
		}

		testing.Sleep(ctx, time.Second*5)
		// get system + dock usb count
		dockUsbDevices, err := usbDevices(ctx)
		if err != nil {
			return err
		}
		testing.ContextLog(ctx, len(dockUsbDevices))
		if len(systemUsbDevices) >= len(dockUsbDevices) {
			return errors.Errorf("failed to create usb controller; system is %d, dock is %d", len(systemUsbDevices), len(dockUsbDevices))
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Second * 30, Interval: time.Second}); err != nil {
		return nil, err
	}

	dockUsbDevices, err := usbDevices(ctx)
	if err != nil {
		return nil, err
	}
	testing.ContextLogf(ctx, "Usb controller created %d", len(dockUsbDevices))

	if err := SwitchFixture(ctx, dockingID, "off", "0"); err != nil {
		return nil, err
	}

	return &UsbController{
		systemUsb: systemUsbDevices,
		dockUsb:   dockUsbDevices,
		inputUsb:  usbsID,
	}, nil
}

func usbDevices(ctx context.Context) ([]string, error) {
	lsusbOut, err := testexec.CommandContext(ctx, "lsusb").Output()
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSpace(string(lsusbOut)), "\n"), nil
}

// VerifyUsbCount verify current usb count is correct when is connected or not
func (uc *UsbController) VerifyUsbCount(ctx context.Context, state ConnectState) error {
	systemCount := len(uc.systemUsb)
	dockCount := len(uc.dockUsb)
	inputCount := len(uc.inputUsb)
	return testing.Poll(ctx, func(c context.Context) error {
		// get current usb count
		usbDevices, err := usbDevices(ctx)
		if err != nil {
			return err
		}
		currentCount := len(usbDevices)

		// verify usb's count
		if state { // usb connected
			difference := currentCount - dockCount
			if difference != inputCount {
				return errors.Errorf("failed to verify connected usb, system is %d, dock is %d, current is %d:, input is %d; usb devices %v", systemCount, dockCount, currentCount, inputCount, usbDevices)
			}
		} else { // usb disconnect
			// 1. usb & station disconnect
			// 2. usb disconnect and station connect
			if currentCount > dockCount {
				return errors.Errorf("failed to verify usb when disconnected: system is %d, dock is %d, current is %d", systemCount, dockCount, currentCount)
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: EthernetTimeout, Interval: time.Second})
}

// Controlusbs such as ext-display1, ethernet, usbs
func Controlusbs(ctx context.Context, uc *UsbController, todo, delayTime, extDispID1, ethernetID string, usbsID []string) error {
	// ext-display 1
	if err := SwitchFixture(ctx, extDispID1, todo, delayTime); err != nil {
		return err
	}
	// ethernet
	if err := SwitchFixture(ctx, ethernetID, todo, delayTime); err != nil {
		return err
	}
	// usbs
	for _, uid := range usbsID {
		if err := SwitchFixture(ctx, uid, todo, delayTime); err != nil {
			return err
		}
	}
	// audio
	return nil
}
