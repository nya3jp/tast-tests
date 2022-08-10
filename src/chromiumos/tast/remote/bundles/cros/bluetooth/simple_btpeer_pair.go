// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"time"

	"chromiumos/tast/common/chameleon"
	cbt "chromiumos/tast/common/chameleon/devices/common/bluetooth"
	"chromiumos/tast/remote/bluetooth"
	bts "chromiumos/tast/services/cros/bluetooth"
	"chromiumos/tast/testing"
)

type simpleBTPeerTestCase struct {
	PortType     chameleon.PortType
	SelectDevice func(btpeer chameleon.Chameleond) cbt.BluezPeripheral
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         SimpleBTPeerPair,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that a btpeer bluetooth devices can be paired and connected to",
		Contacts: []string{
			"jaredbennett@google.com",
			"cros-connectivity@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.bluetooth.BTTestService"},
		Fixture:      "chromeLoggedInWith1BTPeer",
		Timeout:      1 * time.Minute,
		Params: []testing.Param{
			{
				Name: "ble_fast_pair",
				Val: &simpleBTPeerTestCase{
					PortType: chameleon.PortTypeBLEFastPair,
					SelectDevice: func(btpeer chameleon.Chameleond) cbt.BluezPeripheral {
						return btpeer.BLEFastPair()
					},
				},
			},
			{
				Name: "ble_keyboard",
				Val: &simpleBTPeerTestCase{
					PortType: chameleon.PortTypeBLEKeyboard,
					SelectDevice: func(btpeer chameleon.Chameleond) cbt.BluezPeripheral {
						return btpeer.BLEKeyboard()
					},
				},
			},
			{
				Name: "ble_mouse",
				Val: &simpleBTPeerTestCase{
					PortType: chameleon.PortTypeBLEMouse,
					SelectDevice: func(btpeer chameleon.Chameleond) cbt.BluezPeripheral {
						return btpeer.BLEMouse()
					},
				},
			},
			{
				Name: "ble_phone",
				Val: &simpleBTPeerTestCase{
					PortType: chameleon.PortTypeBLEPhone,
					SelectDevice: func(btpeer chameleon.Chameleond) cbt.BluezPeripheral {
						return btpeer.BLEPhone()
					},
				},
			},
			{
				Name: "bluetooth_audio",
				Val: &simpleBTPeerTestCase{
					PortType: chameleon.PortTypeBluetoothAudio,
					SelectDevice: func(btpeer chameleon.Chameleond) cbt.BluezPeripheral {
						return btpeer.BluetoothAudioDevice()
					},
				},
			},
			{
				Name: "bluetooth_hid_keyboard",
				Val: &simpleBTPeerTestCase{
					PortType: chameleon.PortTypeBluetoothHIDKeyboard,
					SelectDevice: func(btpeer chameleon.Chameleond) cbt.BluezPeripheral {
						return btpeer.BluetoothKeyboardDevice()
					},
				},
			},
			{
				Name: "bluetooth_hid_mouse",
				Val: &simpleBTPeerTestCase{
					PortType: chameleon.PortTypeBluetoothHIDMouse,
					SelectDevice: func(btpeer chameleon.Chameleond) cbt.BluezPeripheral {
						return btpeer.BluetoothMouseDevice()
					},
				},
			},
		},
	})
}

// SimpleBTPeerPair tests that a given btpeer audio device can be paired and
// connected to.
func SimpleBTPeerPair(ctx context.Context, s *testing.State) {
	fv := s.FixtValue().(*bluetooth.FixtValue)
	btpeer := fv.BTPeers[0]
	tc := s.Param().(*simpleBTPeerTestCase)

	// Confirm that the btpeer supports this type of device.
	testing.ContextLogf(ctx, "Validating btpeer support for devices of type %q", tc.PortType)
	_, err := btpeer.FetchSupportedPortIDByType(ctx, tc.PortType, 0)
	if err != nil {
		s.Fatal("Failed to get btpeer PortID for this device type: ", err)
	}

	// Select and prepare device for pairing.
	testing.ContextLogf(ctx, "Selecting emulated %q device", tc.PortType)
	device, err := bluetooth.NewEmulatedBTPeerDevice(ctx, tc.SelectDevice(btpeer))
	if err != nil {
		s.Fatal("Failed to call NewEmulatedBTPeerDevice: ", err)
	}
	testing.ContextLogf(ctx, "Setting %s as discoverable", device.String())
	if err := device.RPC().SetDiscoverable(ctx, true); err != nil {
		s.Fatal("Failed to set device as discoverable: ", err)
	}
	testing.ContextLogf(ctx, "Selected %s ready to pair", device.String())

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
