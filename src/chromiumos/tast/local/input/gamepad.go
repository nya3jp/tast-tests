// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package input

import (
	"context"
	"math/big"
	"os"
)

// GamepadEventWriter supports
type GamepadEventWriter struct {
	rw   *RawEventWriter
	virt *os.File // if non-nil, used to hold a virtual device open
	dev  string   // path to underlying device in /dev/input
}

// Gamepad creates a virtual gamepad device.
func Gamepad(ctx context.Context) (*GamepadEventWriter, error) {
	gw := &GamepadEventWriter{}
	const usbBus = 0x3 // BUS_USB from input.h
	var err error
	if gw.dev, gw.virt, err = createVirtual(
		"Sony Interactive Entertainment Wireless Controller",
		devID{usbBus, 0x054c, 0x09cc, 0x0111}, 0, 0x1b,
		map[EventType]*big.Int{
			EV_KEY: makeBigInt([]uint64{0x3fff000000000000, 0, 0, 0, 0}),
			EV_ABS: big.NewInt(0x26081000003003f),
			EV_MSC: big.NewInt(0x10)}); err != nil {
		return nil, err
	}

	if gw.rw, err = Device(ctx, gw.dev); err != nil {
		gw.Close()
		return nil, err
	}

	return gw, nil
}

// Close closes
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
