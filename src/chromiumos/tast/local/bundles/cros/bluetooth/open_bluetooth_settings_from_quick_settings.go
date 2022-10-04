// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/testing"
)

// bluetoothSubPageURL is the URL of the Bluetooth sub-page within the OS Settings.
const bluetoothSubPageURL = "chrome://os-settings/bluetoothDevices"

func init() {
	testing.AddTest(&testing.Test{
		Func:         OpenBluetoothSettingsFromQuickSettings,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that clicking the Settings button on the detailed Bluetooth page within the Quick Settings navigates to the Bluetooth Settings",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:    "floss_disabled",
			Fixture: "bluetoothEnabledWithBlueZ",
		}},
	})
}

// OpenBluetoothSettingsFromQuickSettings tests that a user can successfully
// navigate through to the Bluetooth sub-page within the OS Settings from the
// Settings button in the Bluetooth detailed view within the Quick Settings.
func OpenBluetoothSettingsFromQuickSettings(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*bluetooth.ChromeLoggedInWithBluetoothEnabled).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	if err := quicksettings.NavigateToBluetoothDetailedView(ctx, tconn); err != nil {
		s.Fatal("Failed to navigate to the detailed Bluetooth view: ", err)
	}

	if err := uiauto.New(tconn).LeftClick(quicksettings.BluetoothDetailedViewSettingsButton)(ctx); err != nil {
		s.Fatal("Failed to click the Bluetooth Settings button: ", err)
	}

	// Check if the Bluetooth sub-page within the OS Settings was opened.
	matcher := chrome.MatchTargetURL(bluetoothSubPageURL)
	conn, err := cr.NewConnForTarget(ctx, matcher)
	if err != nil {
		s.Fatal("Failed to open the Bluetooth settings: ", err)
	}
	defer conn.Close()
}
