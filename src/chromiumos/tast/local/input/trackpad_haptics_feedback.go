// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package input

import (
	"context"
	"fmt"
	"math/big"
	"os"

	"chromiumos/tast/testing"
)

type HapticTrackpadEventReaderWriter struct {
	tew TouchscreenEventWriter
}

var nextVirtHapticTrackpadNum = 1

// Trackpad returns an HapticTrackpad that injects events to a device.
//
// Currently a one-off virtual device is created always.
func HapticTrackpad(ctx context.Context) (*HapticTrackpadEventReaderWriter, error) {
	const (
		// Most trackpads use I2C bus. But hardcoding to USB since it is supported
		// in all Chromebook devices.
		busType = 0x3 // BUS_USB from input.h

		// Device constants taken from Pixelbook.
		vendor  = 0x18d1
		product = 0x5028
		version = 0x100

		// Input characteristics.
		props   = 1<<INPUT_PROP_POINTER | 1<<INPUT_PROP_BUTTONPAD | 1<<(0x07) // INPUT_PROP_HAPTICPAD
		evTypes = 1<<EV_KEY | 1<<EV_ABS | 1<<EV_SYN | 1<<EV_FF

		// Abs axes supported in our virtual device.
		absSupportedAxes = 1<<ABS_X | 1<<ABS_Y | 1<<ABS_PRESSURE | 1<<ABS_MT_SLOT |
			1<<ABS_MT_TOUCH_MAJOR | 1<<ABS_MT_TOUCH_MINOR | 1<<ABS_MT_ORIENTATION |
			1<<ABS_MT_POSITION_X | 1<<ABS_MT_POSITION_Y |
			1<<ABS_MT_TRACKING_ID | 1<<ABS_MT_PRESSURE | 1<<ABS_MT_DISTANCE

		// Abs axis constants. Taken from Pixelbook.
		axisMaxX            = 13184
		axisMaxY            = 8704
		axisMaxTracking     = 65535
		axisMaxPressure     = 255
		axisCoordResolution = 128
	)
	axisMaxTouchSlot := 9

	name := fmt.Sprintf("Tast virtual haptic trackpad %d.%d", os.Getpid(), nextVirtHapticTrackpadNum)
	nextVirtHapticTrackpadNum++
	testing.ContextLogf(ctx, "Creating virtual haptic trackpad device %q", name)

	dev, virt, err := createVirtual(name, devID{busType, vendor, product, version}, props, evTypes,
		map[EventType]*big.Int{
			EV_KEY: makeBigInt([]uint64{0xe520, 0x10000, 0, 0, 0, 0}),
			EV_ABS: big.NewInt(absSupportedAxes),
			EV_FF:  makeBigInt([]uint64{0x8000, 0}), // FF_HID(0x4f) defined in Chromium.
		}, map[EventCode]Axis{
			ABS_X:              {axisMaxX, 0, 0, 0, axisCoordResolution},
			ABS_Y:              {axisMaxY, 0, 0, 0, axisCoordResolution},
			ABS_PRESSURE:       {axisMaxPressure, 0, 0, 0, 0},
			ABS_MT_SLOT:        {int32(axisMaxTouchSlot), 0, 0, 0, 0},
			ABS_MT_TOUCH_MAJOR: {13184, 0, 0, 0, 1},
			ABS_MT_TOUCH_MINOR: {8704, 0, 0, 0, 1},
			ABS_MT_ORIENTATION: {90, -90, 0, 0, 0},
			ABS_MT_POSITION_X:  {axisMaxX, 0, 0, 0, axisCoordResolution},
			ABS_MT_POSITION_Y:  {axisMaxY, 0, 0, 0, axisCoordResolution},
			ABS_MT_TRACKING_ID: {axisMaxTracking, 0, 0, 0, 0},
			ABS_MT_PRESSURE:    {axisMaxPressure, 0, 0, 0, 0},
			ABS_MT_DISTANCE:    {1, 0, 0, 0, 0},
		})
	if err != nil {
		return nil, err
	}

	device, err := Device(ctx, dev)
	if err != nil {
		return nil, err
	}

	// Start reading
	go handleFfUploadErase(virt)

	return &HapticTrackpadEventReaderWriter{
		tew: TouchscreenEventWriter{
			rw:            device,
			dev:           dev,
			virt:          virt,
			width:         axisMaxX,
			height:        axisMaxY,
			maxTouchSlot:  axisMaxTouchSlot,
			maxTrackingID: axisMaxTracking,
			maxPressure:   axisMaxPressure,
		},
	}, nil
}

// Close closes the haptic trackpad device.
func (htrw *HapticTrackpadEventReaderWriter) Close() error {
	firstErr := htrw.tew.Close()

	if htrw.tew.virt != nil {
		if err := htrw.tew.virt.Close(); firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
