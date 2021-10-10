// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package input

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"unsafe"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// TrackpadEventWriter supports injecting events into a virtual trackpad device.
type TrackpadEventWriter struct {
	TouchscreenEventWriter
}

var nextVirtTrackpadNum = 1 // appended to virtual trackpad device name

// Trackpad returns an EventWriter that injects events a trackpad device.
//
// If a physical trackpad is present, it is used.
// Otherwise, a one-off virtual device is created.
func Trackpad(ctx context.Context) (*TrackpadEventWriter, error) {
	infos, err := readDevices("")
	if err != nil {
		return nil, errors.Wrap(err, "failed to read devices")
	}
	for _, info := range infos {
		// When the trackpad doesn't support multitouch, use a virtual trackpad instead.
		if !info.isTrackpad() || !info.hasBit(absGroup, uint16(ABS_MT_SLOT)) {
			continue
		}
		testing.ContextLogf(ctx, "Opening trackpad device %+v", info)

		// Get trackpad properties: bounds, max touches, max pressure and max track id.
		f, err := os.Open(info.path)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		var infoX, infoY, infoSlot, infoTrackingID, infoPressure absInfo
		for _, entry := range []struct {
			ec  EventCode
			dst *absInfo
		}{
			{ABS_X, &infoX},
			{ABS_Y, &infoY},
			{ABS_MT_SLOT, &infoSlot},
			{ABS_MT_TRACKING_ID, &infoTrackingID},
			{ABS_MT_PRESSURE, &infoPressure},
		} {
			if err := ioctl(int(f.Fd()), evIOCGAbs(uint(entry.ec)), uintptr(unsafe.Pointer(entry.dst))); err != nil {
				return nil, err
			}
		}

		if infoTrackingID.maximum < infoSlot.maximum {
			return nil, errors.Errorf("invalid MT tracking ID %d; should be >= max slots %d",
				infoTrackingID.maximum, infoSlot.maximum)
		}

		device, err := Device(ctx, info.path)
		if err != nil {
			return nil, err
		}
		return &TrackpadEventWriter{TouchscreenEventWriter{
			rw:            device,
			width:         TouchCoord(infoX.maximum),
			height:        TouchCoord(infoY.maximum),
			maxTouchSlot:  int(infoSlot.maximum),
			maxTrackingID: int(infoTrackingID.maximum),
			maxPressure:   int(infoPressure.maximum),
		}}, nil
	}
	// If we didn't find a real trackpad, create a virtual one.
	return VirtualTrackpad(ctx)
}

// VirtualTrackpad creates a virtual trackpad device and returns an EventWriter that injects events into it.
func VirtualTrackpad(ctx context.Context) (*TrackpadEventWriter, error) {
	const (
		// Most trackpads use I2C bus. But hardcoding to USB since it is supported
		// in all Chromebook devices.
		busType = 0x3 // BUS_USB from input.h

		// Device constants taken from Pixelbook.
		vendor  = 0x18d1
		product = 0x5028
		version = 0x100

		// Input characteristics.
		props   = 1<<INPUT_PROP_POINTER | 1<<INPUT_PROP_BUTTONPAD
		evTypes = 1<<EV_KEY | 1<<EV_ABS | 1<<EV_SYN

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

	// Include our PID in the device name to be extra careful in case an old bundle process hasn't exited.
	name := fmt.Sprintf("Tast virtual trackpad %d.%d", os.Getpid(), nextVirtTrackpadNum)
	nextVirtTrackpadNum++
	testing.ContextLogf(ctx, "Creating virtual trackpad device %q", name)

	dev, virt, err := createVirtual(name, devID{busType, vendor, product, version}, props, evTypes,
		map[EventType]*big.Int{
			EV_KEY: makeBigInt([]uint64{0xe520, 0x10000, 0, 0, 0, 0}),
			EV_ABS: big.NewInt(absSupportedAxes),
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
	return &TrackpadEventWriter{TouchscreenEventWriter{
		rw:            device,
		dev:           dev,
		virt:          virt,
		width:         axisMaxX,
		height:        axisMaxY,
		maxTouchSlot:  axisMaxTouchSlot,
		maxTrackingID: axisMaxTracking,
		maxPressure:   axisMaxPressure,
	}}, nil
}

// MaxPressure returns the max pressure for the touchpad.
func (tew *TrackpadEventWriter) MaxPressure() int {
	return tew.maxPressure
}
