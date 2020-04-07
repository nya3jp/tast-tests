// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package input

import (
	"context"
	// "math/big"
	"os"
	"unsafe"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// TrackpadEventWriter supports injecting events into a virtual trackpad device.
type TrackpadEventWriter struct {
	/*
	rw   *RawEventWriter
	virt *os.File // if non-nil, used to hold a virtual device open
	dev  string   // path to underlying device in /dev/input
	*/
	TouchscreenEventWriter
}

// Trackpad creates a virtual trackpad device and returns and EventWriter that injects events into it.
func Trackpad(ctx context.Context) (*TrackpadEventWriter, error) {
	/*
	tw := &TrackpadEventWriter{}
	const i2cBus = 0x18 // BUS_I2C from input.h
	var err error
	if tw.dev, tw.virt, err = createVirtual(
		"ACPI0C50:00 18D1:5028 Touchpad",
		devID{i2cBus, 0x18d1, 0x5028, 0x0100}, 5, 0xb,
		map[EventType]*big.Int{
			EV_KEY: makeBigInt([]uint64{0xe520, 0x10000, 0, 0, 0, 0}),
			EV_ABS: big.NewInt(0xe73800001000003)},
		nil); err != nil {
		return nil, err
	}
	return tw, nil
	*/
	infos, err := readDevices("")
	if err != nil {
		return nil, errors.Wrap(err, "failed to read devices")
	}
	for _, info := range infos {
		if !info.isTrackpad() {
			continue
		}
		testing.ContextLogf(ctx, "Opening touchscreen device %+v", info)

		// Get touchscreen properties: bounds, max touches, max pressure and max track id.
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
	return nil, errors.New("cannot open an existing touchscreen device")
}

// Close closes the trackpad device.
func (tw *TrackpadEventWriter) Close() error {
	return tw.TouchscreenEventWriter.Close()
	/*
	var firstErr error
	if tw.rw != nil {
		firstErr = tw.rw.Close()
	}
	if tw.virt != nil {
		if err := tw.virt.Close(); firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
	*/
}

// I: Bus=0018 Vendor=18d1 Product=5028 Version=0100
// N: Name="ACPI0C50:00 18D1:5028 Touchpad"
// P: Phys=i2c-ACPI0C50:00
// S: Sysfs=/devices/pci0000:00/0000:00:15.2/i2c_designware.2/i2c-8/i2c-ACPI0C50:00/0018:18D1:5028.0003/input/input9
// U: Uniq=
// H: Handlers=event6
// B: PROP=5
// B: EV=b
// B: KEY=e520 10000 0 0 0 0
// B: ABS=e73800001000003
