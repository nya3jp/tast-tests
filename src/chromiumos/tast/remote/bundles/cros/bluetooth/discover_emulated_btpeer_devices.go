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

func init() {
	testing.AddTest(&testing.Test{
		Func:         DiscoverEmulatedBTPeerDevices,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests that btpeers can be set to emulate a type device and that the DUT can discover them as those devices",
		Contacts: []string{
			"jaredbennett@google.com",
			"cros-connectivity@google.com",
		},
		Attr: []string{
			"group:bluetooth",
			"bluetooth_btpeers_2",
			"bluetooth_flaky",
		},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.bluetooth.BTTestService"},
		Fixture:      "chromeLoggedInWith2BTPeers",
		Timeout:      1 * time.Minute,
	})
}

// DiscoverEmulatedBTPeerDevices tests that btpeers can be set to emulate a
// type device and that the DUT can discover them as those devices.
func DiscoverEmulatedBTPeerDevices(ctx context.Context, s *testing.State) {
	fv := s.FixtValue().(*bluetooth.FixtValue)

	// Discover btpeer1 as a keyboard.
	keyboardDevice, err := bluetooth.NewEmulatedBTPeerDevice(ctx, fv.BTPeers[0], &bluetooth.EmulatedBTPeerDeviceConfig{
		DeviceType: cbt.DeviceTypeKeyboard,
	})
	if err != nil {
		s.Fatal("Failed to configure btpeer1 as a keyboard device: ", err)
	}
	if _, err := fv.BTS.DiscoverDevice(ctx, &bts.DiscoverDeviceRequest{
		Device: keyboardDevice.BTSDevice(),
	}); err != nil {
		s.Fatalf("DUT failed to discover btpeer1 as %s: %v", keyboardDevice.String(), err)
	}

	// Discover btpeer2 as a mouse.
	mouseDevice, err := bluetooth.NewEmulatedBTPeerDevice(ctx, fv.BTPeers[1], &bluetooth.EmulatedBTPeerDeviceConfig{
		DeviceType: cbt.DeviceTypeMouse,
	})
	if err != nil {
		s.Fatal("Failed to configure btpeer2 as a mouse device: ", err)
	}
	if _, err := fv.BTS.DiscoverDevice(ctx, &bts.DiscoverDeviceRequest{
		Device: mouseDevice.BTSDevice(),
	}); err != nil {
		s.Fatalf("DUT failed to discover btpeer1 as %s: %v", mouseDevice.String(), err)
	}

	// Confirm that btpeer1 is also still discoverable as a keyboard, since both
	// peers should be usable at the same time.
	if _, err := fv.BTS.DiscoverDevice(ctx, &bts.DiscoverDeviceRequest{
		Device: keyboardDevice.BTSDevice(),
	}); err != nil {
		s.Fatalf("DUT failed to still discover btpeer1 as %s: %v", keyboardDevice.String(), err)
	}
}
