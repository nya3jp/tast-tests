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
		Fixture:      "chromeLoggedInWith1BTPeers",
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
	_, err := btpeer.FetchSupportedPortIDByType(ctx, tc.PortType, 0)
	if err != nil {
		s.Fatal("Failed to get btpeer PortID for this device type: ", err)
	}

	// Retrieve device info.
	device := tc.SelectDevice(btpeer)
	deviceName, err := device.GetAdvertisedName(ctx)
	if err != nil {
		s.Fatal("Failed to get device name: ", err)
	}
	deviceAddr, err := device.GetLocalBluetoothAddress(ctx)
	if err != nil {
		s.Fatal("Failed to get device address: ", err)
	}

	// Attempt pairing.
	err = device.SetDiscoverable(ctx, true)
	if err != nil {
		s.Fatal("Failed to set device as discoverable: ", err)
	}
	_, err = fv.BTS.PairAndConnectDevice(ctx, &bts.PairAndConnectDeviceRequest{
		Device: &bts.Device{
			Alias:      deviceName,
			MacAddress: deviceAddr,
		},
	})
	if err != nil {
		s.Fatal("Failed to pair and connect device: ", err)
	}
}
