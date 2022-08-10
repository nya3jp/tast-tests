// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"time"

	cbt "chromiumos/tast/common/chameleon/devices/common/bluetooth"
	"chromiumos/tast/remote/bluetooth"
	bts "chromiumos/tast/services/cros/bluetooth"
	"chromiumos/tast/testing"
)

type simpleBTPeerTestCase struct {
	DeviceType cbt.DeviceType
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         SimpleBTPeerPair,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests pairing of classic and LE btpeers, with pairing done through dbus",
		Contacts: []string{
			"jaredbennett@google.com",
			"cros-connectivity@google.com",
		},
		Attr: []string{
			"group:bluetooth",
			"bluetooth_core",
			"bluetooth_btpeers_1",
			"bluetooth_flaky",
		},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.bluetooth.BTTestService"},
		Fixture:      "chromeLoggedInWith1BTPeer",
		Timeout:      90 * time.Second,
		Params: []testing.Param{
			{
				Name: "le_keyboard",
				Val: &simpleBTPeerTestCase{
					DeviceType: cbt.DeviceTypeLEKeyboard,
				},
			},
			{
				Name: "le_mouse",
				Val: &simpleBTPeerTestCase{
					DeviceType: cbt.DeviceTypeLEMouse,
				},
			},
			{
				Name: "le_phone",
				Val: &simpleBTPeerTestCase{
					DeviceType: cbt.DeviceTypeLEPhone,
				},
			},
			{
				Name: "keyboard",
				Val: &simpleBTPeerTestCase{
					DeviceType: cbt.DeviceTypeKeyboard,
				},
			},
			{
				Name: "mouse",
				Val: &simpleBTPeerTestCase{
					DeviceType: cbt.DeviceTypeMouse,
				},
			},
		},
	})
}

// SimpleBTPeerPair tests pairing of classic and LE btpeers, with pairing done
// through dbus.
func SimpleBTPeerPair(ctx context.Context, s *testing.State) {
	fv := s.FixtValue().(*bluetooth.FixtValue)
	btpeer := fv.BTPeers[0]
	tc := s.Param().(*simpleBTPeerTestCase)

	// Emulate the desired device type with btpeer.
	testing.ContextLogf(ctx, "Configuring btpeer as %q device", tc.DeviceType.String())
	device, err := bluetooth.NewEmulatedBTPeerDevice(ctx, btpeer, &bluetooth.EmulatedBTPeerDeviceConfig{
		DeviceType: tc.DeviceType,
	})
	if err != nil {
		s.Fatal("Failed to call NewEmulatedBTPeerDevice: ", err)
	}
	testing.ContextLogf(ctx, "Device %s ready to pair", device.String())

	// Attempt pairing device with DUT.
	testing.ContextLogf(ctx, "Paring %s", device.String())
	if _, err = fv.BTS.PairAndConnectDevice(ctx, &bts.PairAndConnectDeviceRequest{
		Device:          device.BTSDevice(),
		ForceNewPair:    true,
		ForceNewConnect: true,
	}); err != nil {
		s.Fatalf("Failed to pair and connect %s: %v", device.String(), err)
	}
	testing.ContextLogf(ctx, "Successfully paired %s", device.String())
}
