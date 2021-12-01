// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ossettings

import (
	"context"

	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// osBluetoothSettingsButton is the arrow button on the OS Settings that a user
// can click to navigate to the Bluetooth Settings.
var osBluetoothSettingsButton = nodewith.HasClass("subpage-arrow").NameContaining("Bluetooth").Role(role.Button)

// NavigateToBluetoothSettingsPage will navigate to the Bluetooth sub-page
// within the OS Settings by clicking the sub-page button. This is safe to call
// when the OS Settings are already open.
func NavigateToBluetoothSettingsPage(ctx context.Context, tconn *chrome.TestConn) (*OSSettings, error) {
	app, err := Launch(ctx, tconn)
	if err != nil {
		return app, err
	}

	if err := bluetooth.Enable(ctx); err != nil {
		return app, err
	}

	if err := uiauto.New(tconn).LeftClick(osBluetoothSettingsButton)(ctx); err != nil {
		return app, err
	}

	return app, nil
}
