// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PairNewDeviceFromBluetoothSettings,
		Desc: "Checks that the pairing dialog can be opened from the Bluetooth Settings sub-page",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInWithBluetoothEnabled",
	})
}

// PairNewDeviceFromBluetoothSettings tests that a user can successfully open
// the pairing dialog from the "Pair new device" button on the Bluetooth
// Settings sub-page.
func PairNewDeviceFromBluetoothSettings(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	app, err := ossettings.NavigateToBluetoothSettingsPage(ctx, tconn)
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
