// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

// bluetoothSettingsBluetoothToggleButton is the Bluetooth toggle within the Bluetooth Settings page.
var bluetoothSettingsBluetoothToggleButton = nodewith.NameContaining("Bluetooth").Role(role.ToggleButton)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ToggleBluetoothFromBluetoothSettings,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that Bluetooth can be enabled and disabled from the Bluetooth Settings sub-page",
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

// ToggleBluetoothFromBluetoothSettings tests that a user can successfully
// toggle the Bluetooth state using the Bluetooth toggle within the
// Bluetooth Settings sub-page.
func ToggleBluetoothFromBluetoothSettings(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*bluetooth.ChromeLoggedInWithBluetoothEnabled).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	bt := s.FixtValue().(*bluetooth.ChromeLoggedInWithBluetoothEnabled).Impl

	app, err := ossettings.NavigateToBluetoothSettingsPage(ctx, tconn, bt)
	defer app.Close(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	if err != nil {
		s.Fatal("Failed to show the Bluetooth Settings sub-page: ", err)
	}

	ui := uiauto.New(tconn)

	if err := ui.WaitUntilExists(bluetoothSettingsBluetoothToggleButton)(ctx); err != nil {
		s.Fatal("Failed to find the Bluetooth toggle: ", err)
	}

	state := false
	const iterations = 20
	for i := 0; i < iterations; i++ {
		s.Logf("Toggling Bluetooth (iteration %d of %d)", i+1, iterations)

		if err := ui.LeftClick(bluetoothSettingsBluetoothToggleButton)(ctx); err != nil {
			s.Fatal("Failed to click the Bluetooth toggle: ", err)
		}
		if err := bt.PollForAdapterState(ctx, state); err != nil {
			s.Fatal("Failed to toggle Bluetooth state: ", err)
		}
		state = !state
	}
}
