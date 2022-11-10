// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PairNewDeviceFromOSSettings,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that the pairing dialog can be opened from the OS Settings",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Attr:         []string{"group:bluetooth"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:      "floss_disabled",
			Fixture:   "bluetoothEnabledWithBlueZ",
			ExtraAttr: []string{"bluetooth_flaky"},
		}, {
			Name:      "floss_enabled",
			Fixture:   "bluetoothEnabledWithFloss",
			ExtraAttr: []string{"bluetooth_floss"},
		}},
	})
}

// PairNewDeviceFromOSSettings tests that a user can successfully open the
// pairing dialog from the "Pair new device" button on the OS Settings page.
func PairNewDeviceFromOSSettings(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(bluetooth.HasTconn).Tconn()

	app, err := ossettings.Launch(ctx, tconn)
	defer app.Close(ctx)

	if err != nil {
		s.Fatal("Failed to launch OS Settings: ", err)
	}

	bt := s.FixtValue().(bluetooth.HasBluetoothImpl).BluetoothImpl()

	if err := bt.Enable(ctx); err != nil {
		s.Fatal("Failed to enable Bluetooth: ", err)
	}

	ui := uiauto.New(tconn)

	if err := uiauto.Combine("Open the \"Pair new device\" dialog",
		ui.LeftClick(ossettings.BluetoothPairNewDeviceButton),
		ui.WaitUntilExists(ossettings.BluetoothPairNewDeviceModal),
	)(ctx); err != nil {
		s.Fatal("Failed to open the \"Pair new device\" dialog: ", err)
	}
}
