// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/testing"
)

// BluetoothPairingDialogURL is the URL of the Bluetooth pairing dialog.
const BluetoothPairingDialogURL = "chrome://bluetooth-pairing/"

// BluetoothSubPageURL is the URL of the Bluetooth sub-page within the OS Settings.
const BluetoothSubPageURL = "chrome://os-settings/bluetoothDevices"

// FeaturePodIconButton is the icon child of the Bluetooth feature pod button.
var FeaturePodIconButton = nodewith.ClassName("FeaturePodIconButton").NameContaining("Bluetooth")

// FeaturePodLabelButton is the label child of the Bluetooth feature pod button.
var FeaturePodLabelButton = nodewith.ClassName("FeaturePodLabelButton").NameContaining("Bluetooth")

// DetailedView is the detailed Bluetooth view within the Quick Settings.
var DetailedView = nodewith.ClassName("BluetoothDetailedViewImpl")

// DetailedViewPairNewDeviceButton is the "Pair new device" button child within the detailed Bluetooth view.
var DetailedViewPairNewDeviceButton = nodewith.ClassName("TopShortcutButton").NameContaining("Pair new device").Ancestor(DetailedView)

// DetailedViewSettingsButton is the Settings button child within the detailed Bluetooth view.
var DetailedViewSettingsButton = nodewith.ClassName("TopShortcutButton").NameContaining("Bluetooth settings").Ancestor(DetailedView)

// DetailedViewToggleButton is the Bluetooth toggle child within the detailed Bluetooth view.
var DetailedViewToggleButton = nodewith.ClassName("ToggleButton").NameContaining("Bluetooth").Ancestor(DetailedView)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "chromeLoggedInWithBluetoothRevamp",
		Desc: "Logs into a user session with the BluetoothRevamp feature flag enabled",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.EnableFeatures("BluetoothRevamp")}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
}

// ShowDetailedView will show the detailed Bluetooth view within the Quick
// Settings. This is safe to call when the Quick Settings are already open.
func ShowDetailedView(ctx context.Context, tconn *chrome.TestConn) error {
	if err := quicksettings.Hide(ctx, tconn); err != nil {
		return err
	}

	if err := quicksettings.ShowWithRetry(ctx, tconn, 5*time.Second); err != nil {
		return err
	}

	ui := uiauto.New(tconn)

	if err := uiauto.Combine("Click the Bluetooth feature pod label",
		ui.WaitUntilExists(FeaturePodLabelButton),
		ui.LeftClick(FeaturePodLabelButton),
		ui.WaitUntilExists(DetailedView),
	)(ctx); err != nil {
		return err
	}
	return nil
}
