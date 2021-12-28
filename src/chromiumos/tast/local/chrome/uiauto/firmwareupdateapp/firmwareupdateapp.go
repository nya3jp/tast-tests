// Copyright 2021 The Chromium OS Authors. All rights reserved.
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

// FirmwareUpdateRootNode returns the root ui node of Firmware Update app.
func FirmwareUpdateRootNode(ctx context.Context, tconn *chrome.TestConn) (*nodewith.Finder, error) {
	ui := uiauto.New(tconn)
	err := ui.WithTimeout(20 * time.Second).WaitUntilExists(appRootNodeParams)(ctx)
	return appRootNodeParams, err
}

// Launch firmware update via default method and finder and error.
func Launch(ctx context.Context, tconn *chrome.TestConn) (*nodewith.Finder, error) {
	err := apps.Launch(ctx, tconn, apps.FirmwareUpdate.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch firmware update app")
	}

	err = ash.WaitForApp(ctx, tconn, apps.FirmwareUpdate.ID, time.Minute)
	if err != nil {
		return nil, errors.Wrap(err, "firmware update  app did not appear in shelf after launch")
	}

	dxRootnode, err := FirmwareUpdateRootNode(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find firmware update  app")
	}
	return dxRootnode, nil
}
