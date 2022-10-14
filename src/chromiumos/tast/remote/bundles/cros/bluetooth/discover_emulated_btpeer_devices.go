// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	cbt "chromiumos/tast/common/chameleon/devices/common/bluetooth"
	"chromiumos/tast/remote/bluetooth"
	bluetoothService "chromiumos/tast/services/cros/bluetooth"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DiscoverEmulatedBTPeerDevices,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests that btpeers can be set to emulate a type device and that the DUT can discover them as those devices",
		Contacts: []string{
			"jaredbennett@google.com",
			"cros-connectivity@google.com",
		},
		// TODO(b/245584709): Need to make new btpeer test attributes.
		Attr:         []string{},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.bluetooth.BluetoothUIService"},
		Fixture:      "chromeLoggedInWith2BTPeers",
		Timeout:      time.Second * 30,
	})
}

func deviceWithAddressExists(address string, deviceInfos []*bluetoothService.DeviceInfo) bool {
	for _, deviceInfo := range deviceInfos {
		if deviceInfo.Address == address {
			return true
		}
	}
	return false
}

// DiscoverEmulatedBTPeerDevices tests that btpeers can be set to emulate a
// type device and that the DUT can discover them as those devices.
func DiscoverEmulatedBTPeerDevices(ctx context.Context, s *testing.State) {
	fv := s.FixtValue().(*bluetooth.FixtValue)

	// Initialize one Bluetooth peer as a keyboard and make it discoverable.
	keyboardDevice, err := bluetooth.NewEmulatedBTPeerDevice(ctx, fv.BTPeers[0].BluetoothKeyboardDevice())
	if err != nil {
		s.Fatal("Failed to configure btpeer1 as a keyboard device: ", err)
	}
	if keyboardDevice.DeviceType() != cbt.DeviceTypeKeyboard {
		s.Fatalf("Attempted to emulate btpeer device as a %s, but the actual device type is %s", cbt.DeviceTypeKeyboard, keyboardDevice.DeviceType())
	}

	// Initialize another Bluetooth peer as a mouse and make it discoverable.
	mouseDevice, err := bluetooth.NewEmulatedBTPeerDevice(ctx, fv.BTPeers[1].BluetoothMouseDevice())
	if err != nil {
		s.Fatal("Failed to configure btpeer2 as a mouse device: ", err)
	}
	if mouseDevice.DeviceType() != cbt.DeviceTypeMouse {
		s.Fatalf("Attempted to emulate btpeer device as a %s, but the actual device type is %s", cbt.DeviceTypeMouse, mouseDevice.DeviceType())
	}

	if _, err = fv.BluetoothUIService.StartDiscovery(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to start discovery: ", err)
	}

	res, err := fv.BluetoothUIService.Devices(ctx, &emptypb.Empty{})
	if err != nil {
		s.Fatal("Failed to get devices: ", err)
	}

	// Check that both the keyboard and the mouse are discoverable.
	if !deviceWithAddressExists(keyboardDevice.LocalBluetoothAddress(), res.DeviceInfos) {
		s.Fatal("Failed to discover the keyboard")
	}
	if !deviceWithAddressExists(mouseDevice.LocalBluetoothAddress(), res.DeviceInfos) {
		s.Fatal("Failed to discover the mouse")
	}

	if _, err = fv.BluetoothUIService.StopDiscovery(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to stop discovery: ", err)
	}
}
