// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package printmanagementapp contains common functions used in the app.
package printmanagementapp

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
)

var printManagementRootNodeParams = ui.FindParams{
	Name: apps.PrintManagement.Name,
	Role: ui.RoleTypeWindow,
}

var printManagementClearHistoryButton = ui.FindParams{
	Name: "Clear all history",
	Role: ui.RoleTypeButton,
}

// PrintManagementRootNode returns the root ui node of Diagnotsics app.
func PrintManagementRootNode(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
	return ui.FindWithTimeout(ctx, tconn, printManagementRootNodeParams, 20*time.Second)
}

// WaitForApp waits for the app to be shown and rendered.
func WaitForApp(ctx context.Context, tconn *chrome.TestConn) error {
	dxRootnode, err := PrintManagementRootNode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find print management app")
	}
	defer dxRootnode.Release(ctx)

	// Find the session log button to verify app is rendering.
	if _, err := dxRootnode.DescendantWithTimeout(ctx, printManagementClearHistoryButton, 20*time.Second); err != nil {
		return errors.Wrap(err, "failed to render print management app")
	}
	return nil
}
