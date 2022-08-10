// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ossettings

import (
	"context"

	"chromiumos/tast/local/bluetooth/bluez"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// osBluetoothSettingsButton is the arrow button on the OS Settings that a user
// can click to navigate to the Bluetooth Settings.
var osBluetoothSettingsButton = nodewith.HasClass("subpage-arrow").NameContaining("Bluetooth").Role(role.Button)

// OsSettingsBluetoothToggleButton is the Bluetooth toggle on the OS Settings page.
var OsSettingsBluetoothToggleButton = nodewith.NameContaining("Bluetooth").Role(role.ToggleButton)

// BluetoothPairNewDeviceButton is the "pair new device" button within the OS Settings and Bluetooth Settings.
var BluetoothPairNewDeviceButton = nodewith.NameContaining("Pair new device").Role(role.Button)

// BluetoothPairNewDeviceModal is the modal that is opened when the "pair new
// device" button within either the OS Settings or Bluetooth Settings is pressed.
var BluetoothPairNewDeviceModal = nodewith.NameContaining("Pair new device").Role(role.Heading)

// NavigateToBluetoothSettingsPage will navigate to the Bluetooth sub-page
// within the OS Settings by clicking the sub-page button. This is safe to call
// when the OS Settings are already open.
func NavigateToBluetoothSettingsPage(ctx context.Context, tconn *chrome.TestConn) (*OSSettings, error) {
	app, err := Launch(ctx, tconn)
	if err != nil {
		return app, err
	}

	if err := bluez.Enable(ctx); err != nil {
		return app, err
	}

	ui := uiauto.New(tconn)

	if err := uiauto.Combine("Focus and click the Bluetooth Settings button",
		ui.FocusAndWait(osBluetoothSettingsButton),
		ui.LeftClick(osBluetoothSettingsButton),
	)(ctx); err != nil {
		return app, err
	}

	return app, nil
}
