// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package firmwareupdateapp contains drivers for controlling the ui of
// firmware update SWA.
package firmwareupdateapp

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

var appRootNodeParams = nodewith.Name(apps.FirmwareUpdate.Name).Role(role.Window)

// AppHeader exported to find application header.
var AppHeader = nodewith.ClassName("firmware-header-font").Role(role.Heading).Ancestor(appRootNodeParams).First()

// FirmwareUpdateRootNode returns the root ui node of Firmware Update
// application.
// An error is returned if root node cannot be found in UI.
func FirmwareUpdateRootNode(ctx context.Context, tconn *chrome.TestConn) (*nodewith.Finder, error) {
	ui := uiauto.New(tconn)
	err := ui.WithTimeout(20 * time.Second).WaitUntilExists(appRootNodeParams)(ctx)
	return appRootNodeParams, err
}

// Launch opens firmware update then returns results of attempting to access
// the application root node.
// An error is returned if the application fails to launch.
func Launch(ctx context.Context, tconn *chrome.TestConn) (*nodewith.Finder, error) {
	err := apps.Launch(ctx, tconn, apps.FirmwareUpdate.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch firmware update app")
	}

	err = ash.WaitForApp(ctx, tconn, apps.FirmwareUpdate.ID, time.Minute)
	if err != nil {
		return nil, errors.Wrap(err, "firmware update app did not appear in shelf after launch")
	}

	appRootNode, err := FirmwareUpdateRootNode(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find firmware update app")
	}
	return appRootNode, nil
}
