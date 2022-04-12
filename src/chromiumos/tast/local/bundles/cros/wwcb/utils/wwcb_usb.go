// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package utils provides funcs to cleanup folders in ChromeOS.
package utils

import (
	"context"
	"fmt"
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
func NewUsbController(ctx context.Context, s *testing.State) (*UsbController, error) {

	s.Log("Starting create usb recorder")

	// plug in station
	if err := ControlFixture(ctx, s, StationType, StationIndex, ActionPlugin, false); err != nil {
		return nil, err
	}

	// get system + station total usb count
	count, err := GetUsbCount(ctx, s)
	if err != nil {
		return nil, err
	}

	// unplug station
	if err := ControlFixture(ctx, s, StationType, StationIndex, ActionUnplug, false); err != nil {
		return nil, err
	}

	s.Log("Usb recorder created")

	return &UsbController{
		systemCount: count,
	}, nil
}

// GetUsbCount get length of array
func GetUsbCount(ctx context.Context, s *testing.State) (int, error) {

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

	inputCount, err := GetInputArgumentCount(ctx, s, PerpUsb)
	if err != nil {
		return err
	}

	// get current usb count
	currentCount, err := GetUsbCount(ctx, s)
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

// ControlUsbs control usbs to plug in / unplug, one by one
func (ur *UsbController) ControlUsbs(ctx context.Context, s *testing.State, action ActionState, needToDelay bool) error {

	// input argument array
	args, err := GetInputArgument(ctx, s, PerpUsb)
	if err != nil {
		return err
	}

	s.Log(PrettyPrint(args))

	// control fixture by input argument array
	for _, arg := range args {
		for i := 0; i < arg.Count; i++ {
			sIndex := fmt.Sprintf("ID%d", arg.StartIndex+i)
			if err := ControlFixture(ctx, s, arg.SwitchFixtureType, sIndex, action, needToDelay); err != nil {
				return err
			}

		}
	}

	return nil
}
