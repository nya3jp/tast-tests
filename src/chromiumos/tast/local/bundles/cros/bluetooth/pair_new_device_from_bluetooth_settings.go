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
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PairNewDeviceFromBluetoothSettings,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that the pairing dialog can be opened from the Bluetooth Settings sub-page",
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

// PairNewDeviceFromBluetoothSettings tests that a user can successfully open
// the pairing dialog from the "Pair new device" button on the Bluetooth
// Settings sub-page.
func PairNewDeviceFromBluetoothSettings(ctx context.Context, s *testing.State) {
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

	if err := uiauto.Combine("Open the \"Pair new device\" dialog",
		ui.LeftClick(ossettings.BluetoothPairNewDeviceButton),
		ui.WaitUntilExists(ossettings.BluetoothPairNewDeviceModal),
	)(ctx); err != nil {
		s.Fatal("Failed to open the \"Pair new device\" dialog: ", err)
	}
}
