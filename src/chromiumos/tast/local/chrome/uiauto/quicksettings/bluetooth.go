// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package quicksettings

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
)

// bluetoothDetailedView is the detailed Bluetooth view within the Quick
// Settings.
var bluetoothDetailedView = nodewith.ClassName("BluetoothDetailedViewImpl")

// bluetoothFeaturePodLabelButton is the label child of the Bluetooth feature pod button.
var bluetoothFeaturePodLabelButton = nodewith.ClassName("FeaturePodLabelButton").NameContaining("Bluetooth")

// BluetoothDetailedViewPairNewDeviceButton is the "Pair new device" button
// child within the detailed Bluetooth view.
var BluetoothDetailedViewPairNewDeviceButton = nodewith.ClassName("IconButton").NameContaining("Pair new device").Ancestor(bluetoothDetailedView)

// BluetoothDetailedViewSettingsButton is the Settings button child within the
// detailed Bluetooth view.
var BluetoothDetailedViewSettingsButton = nodewith.ClassName("IconButton").NameContaining("Bluetooth settings").Ancestor(bluetoothDetailedView)

// BluetoothDetailedViewToggleButton is the Bluetooth toggle child within the
// detailed Bluetooth view.
var BluetoothDetailedViewToggleButton = nodewith.ClassName("ToggleButton").NameContaining("Bluetooth").Ancestor(bluetoothDetailedView)

// NavigateToBluetoothDetailedView will navigate to the detailed Bluetooth view
// within the Quick Settings. This is safe to call even when the Quick Settings
// are already open.
func NavigateToBluetoothDetailedView(ctx context.Context, tconn *chrome.TestConn) error {
	if err := Expand(ctx, tconn); err != nil {
		return err
	}

	ui := uiauto.New(tconn)

	if err := uiauto.Combine("Click the Bluetooth feature pod label",
		ui.LeftClick(bluetoothFeaturePodLabelButton),
		ui.WaitUntilExists(bluetoothDetailedView),
	)(ctx); err != nil {
		return err
	}
	return nil
}
