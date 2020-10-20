// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package scanapp contains functions used to interact with the Scan SWA.
package scanapp

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
)

var scanAppRootNodeParams = ui.FindParams{
	Name: apps.Scan.Name,
	Role: ui.RoleTypeWindow,
}

var scanButton = ui.FindParams{
	Name: "Scan",
	Role: ui.RoleTypeButton,
}

// RootNode returns the root UI node of the Scan SWA.
func RootNode(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
	return ui.FindWithTimeout(ctx, tconn, scanAppRootNodeParams, 20*time.Second)
}

// WaitForApp waits for the Scan SWA to be shown and rendered.
func WaitForApp(ctx context.Context, tconn *chrome.TestConn) error {
	appRootNode, err := RootNode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find Scan app")
	}

	// Find the scan button to verify the app is rendering.
	if _, err := appRootNode.DescendantWithTimeout(ctx, scanButton, 20*time.Second); err != nil {
		return errors.Wrap(err, "failed to render Scan app")
	}

	return nil
}
