// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package input

import (
	"context"
	"math/big"
	"os"

	"chromiumos/tast/errors"
)

// Axis contains information about a gamepad axis.
type Axis struct {
	Maximum    int32
	Minimum    int32
	Fuzz       int32
	Flat       int32
	Resolution int32
}

// GamepadEvent contains information about button or axis event.
type GamepadEvent struct {
	Et  EventType
	Ec  EventCode
	Val int32
}

// GamepadEventWriter supports injecting events into a virtual gamepad device.
type GamepadEventWriter struct {
	rw   *RawEventWriter
	virt *os.File // if non-nil, used to hold a virtual device open
	dev  string   // path to underlying device in /dev/input
}

// Gamepad creates a virtual gamepad device and returns and EventWriter that injects events into it.
func Gamepad(ctx context.Context) (*GamepadEventWriter, error) {
	gw := &GamepadEventWriter{}
	const usbBus = 0x3 // BUS_USB from input.h
	var err error
	if gw.dev, gw.virt, err = createVirtual(
		gw.DeviceName(),
		devID{usbBus, gw.VendorID(), gw.ProductID(), 0x0111}, 0, 0x1b,
		map[EventType]*big.Int{
			EV_KEY: makeBigInt([]uint64{0x3fff000000000000, 0, 0, 0, 0}),
			EV_ABS: big.NewInt(0x26081000003003f),
			EV_MSC: big.NewInt(0x10)},
		gw.Axes()); err != nil {
		return nil, err
	}

	if gw.rw, err = Device(ctx, gw.dev); err != nil {
		gw.Close()
		return nil, err
	}

	return gw, nil
}

// Close closes the gamepad device.
func (gw *GamepadEventWriter) Close() error {
	var firstErr error
	if gw.rw != nil {
		firstErr = gw.rw.Close()
	}
	if gw.virt != nil {
		if err := gw.virt.Close(); firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Device returns the path of the underlying device, e.g. "/dev/input/event3".
func (gw *GamepadEventWriter) Device() string { return gw.dev }

// VendorID returns the vendor ID of the virtual gamepad device.
func (gw *GamepadEventWriter) VendorID() uint16 { return 0x054c }

// ProductID returns the product ID of the virtual gamepad device.
func (gw *GamepadEventWriter) ProductID() uint16 { return 0x09cc }

// DeviceName returns the device name of the virtual gamepad device.
func (gw *GamepadEventWriter) DeviceName() string {
	return "Wireless Controller"
}

// Axes returns the absolute axes of the virtual gamepad device.
func (gw *GamepadEventWriter) Axes() map[EventCode]Axis {
	// The values are taken from the actual device.
	return map[EventCode]Axis{
		ABS_X:              {255, 0, 0, 15, 0},
		ABS_Y:              {255, 0, 0, 15, 0},
		ABS_Z:              {255, 0, 0, 15, 0},
		ABS_RX:             {255, 0, 0, 15, 0},
		ABS_RY:             {255, 0, 0, 15, 0},
		ABS_RZ:             {255, 0, 0, 15, 0},
		ABS_HAT0X:          {1, -1, 0, 0, 0},
		ABS_HAT0Y:          {1, -1, 0, 0, 0},
		ABS_MISC:           {32512, -32768, 255, 4080, 0},
		ABS_MT_SLOT:        {1, 0, 0, 0, 0},
		ABS_MT_POSITION_X:  {1920, 0, 0, 0, 0},
		ABS_MT_POSITION_Y:  {942, 0, 0, 0, 0},
		ABS_MT_TRACKING_ID: {65535, 0, 0, 0, 0}}
}

// sendKey writes a EV_KEY event containing the specified code and value, followed by a EV_SYN event.
func (gw *GamepadEventWriter) sendKey(ctx context.Context, ec EventCode, val int32) error {
	if err := gw.rw.Event(EV_KEY, ec, val); err != nil {
		return err
	}
	return gw.rw.Sync()
}

// PressButton presses the gamepad button specified by EventCode.
//
// Caveat: UIAutomator will never return after pressing a button and it will
// keep sending pressing key event after calling this function, so remember to
// call ReleaseButton() to release button after calling PressButton().
// Or call TapButton() including PressButton() and ReleaseButton() instead.
func (gw *GamepadEventWriter) PressButton(ctx context.Context, ec EventCode) error {
	return gw.sendKey(ctx, ec, 1)
}

// ReleaseButton releases the gamepad button specified by EventCode.
func (gw *GamepadEventWriter) ReleaseButton(ctx context.Context, ec EventCode) error {
	return gw.sendKey(ctx, ec, 0)
}

// TapButton presses and releases the gamepad button specified by EventCode.
func (gw *GamepadEventWriter) TapButton(ctx context.Context, ec EventCode) error {
	if err := gw.PressButton(ctx, ec); err != nil {
		return err
	}
	return gw.ReleaseButton(ctx, ec)
}

// MoveAxis moves the gamepad axis specified by EventCode to the value.
func (gw *GamepadEventWriter) MoveAxis(ctx context.Context, ec EventCode, val int32) error {
	if err := gw.rw.Event(EV_ABS, ec, val); err != nil {
		return err
	}
	return gw.rw.Sync()
}

// PressButtonsAndAxes allows to send more than one gamepad events at the same times.
//
// Caveat: UIAutomator will never return after pressing a button and it will
// keep sending pressing key event if there is no releasing key event, so after
// pressing key button, remember to release key button.
func (gw *GamepadEventWriter) PressButtonsAndAxes(ctx context.Context, events []GamepadEvent) error {
	if len(events) == 0 {
		return errors.New("no gamepad events found")
	}
	for _, event := range events {
		if err := gw.rw.Event(event.Et, event.Ec, event.Val); err != nil {
			return err
		}
	}
	return gw.rw.Sync()
}
