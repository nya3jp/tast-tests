// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package input

import (
	"context"
	"math/big"
	"os"
)

// TrackpadEventWriter supports injecting events into a virtual trackpad device.
type TrackpadEventWriter struct {
	rw   *RawEventWriter
	virt *os.File // if non-nil, used to hold a virtual device open
	dev  string   // path to underlying device in /dev/input
}

// Trackpad creates a virtual trackpad device and returns and EventWriter that injects events into it.
func Trackpad(ctx context.Context) (*TrackpadEventWriter, error) {
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
}

// Close closes the trackpad device.
func (tw *TrackpadEventWriter) Close() error {
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
