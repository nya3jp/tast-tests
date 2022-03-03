// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// UsbController to control usb fixture
type UsbController struct {
	systemCount int
}

// NewUsbController to create object to control usb fixture also record system usb count (as condition: plug in station without any usb)
func NewUsbController(ctx context.Context, dockingID string) (*UsbController, error) {

	testing.ContextLog(ctx, "Starting create usb recorder")

	// plug in station
	if err := SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return nil, err
	}

	// get system + station total usb count
	count, err := GetUsbCount(ctx)
	if err != nil {
		return nil, err
	}

	// unplug station
	if err := SwitchFixture(ctx, dockingID, "off", "0"); err != nil {
		return nil, err
	}

	testing.ContextLog(ctx, "Usb recorder created")

	return &UsbController{
		systemCount: count,
	}, nil
}

// GetUsbCount get length of array
func GetUsbCount(ctx context.Context) (int, error) {

	var array []string

	// use command to list usb devices
	lsusb := testexec.CommandContext(ctx, "lsusb")
	out, err := lsusb.Output()
	if err != nil {
		return -1, err
	}

	// split string
	result := strings.TrimSpace(string(out))
	devices := strings.Split(result, "\n")

	// append to device array
	for _, device := range devices {
		if strings.Contains(strings.ToLower(device), "device") {
			array = append(array, device)
		}
	}

	return len(array), nil
}

// VerifyUsbCount verify current usb count is correct to input count
func (ur *UsbController) VerifyUsbCount(ctx context.Context, s *testing.State, state ConnectState) error {

	inputCount, err := GetInputArgumentCount(ctx, s, "PerpUsb")
	if err != nil {
		return err
	}

	// get current usb count
	currentCount, err := GetUsbCount(ctx)
	if err != nil {
		return err
	}

	// verify usb's count
	if state { // usb connected
		difference := currentCount - ur.systemCount
		if difference != inputCount {
			return errors.Errorf("failed to verify connected usb, system is %d, current is %d:, input is %d ", ur.systemCount, currentCount, inputCount)
		}
	} else { // usb disconnect
		// 1. usb & station disconnect
		// 2. usb disconnect and station connect
		if currentCount > ur.systemCount {
			return errors.Errorf("failed to verify usb when disconnected: system is %d, current is %d", ur.systemCount, currentCount)
		}
	}

	return nil
}

// ControlPeripherals such as ext-display1, ethernet, usbs
func ControlPeripherals(ctx context.Context, s *testing.State, uc *UsbController, todo, delayTime, extDispID1, ethernetID, peripheralID1, peripheralID2, peripheralID3, peripheralID4, peripheralID5 string) error {

	// ext-display 1
	if err := SwitchFixture(ctx, extDispID1, todo, delayTime); err != nil {
		return err
	}

	// ethernet
	if err := SwitchFixture(ctx, ethernetID, todo, delayTime); err != nil {
		return err
	}

	// usbs
	if err := SwitchFixture(ctx, peripheralID1, todo, delayTime); err != nil {
		return err
	}
	if err := SwitchFixture(ctx, peripheralID2, todo, delayTime); err != nil {
		return err
	}
	if err := SwitchFixture(ctx, peripheralID3, todo, delayTime); err != nil {
		return err
	}
	if err := SwitchFixture(ctx, peripheralID4, todo, delayTime); err != nil {
		return err
	}
	if err := SwitchFixture(ctx, peripheralID5, todo, delayTime); err != nil {
		return err
	}
	// audio
	return nil
}
