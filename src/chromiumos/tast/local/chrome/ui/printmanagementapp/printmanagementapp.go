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
	"chromiumos/tast/local/chrome/ash"
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

// PrintManagementHistoryLabel export is used to find the label for the History
// section.
var PrintManagementHistoryLabel = ui.FindParams{
	Name: "History",
	Role: ui.RoleTypeStaticText,
}

// PrintManagementPrintJobEntry export is used to find all print job entries.
var PrintManagementPrintJobEntry = ui.FindParams{
	ClassName: "list-item flex-center",
}

// Launch Print Management app via default method and return root node.
func Launch(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
	err := apps.Launch(ctx, tconn, apps.PrintManagement.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch Print Management app")
	}

	err = ash.WaitForApp(ctx, tconn, apps.PrintManagement.ID)
	if err != nil {
		return nil, errors.Wrap(err, "Print Management app did not appear in shelf after launch")
	}

	rootnode, err := ui.FindWithTimeout(ctx, tconn, printManagementRootNodeParams, 20*time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find Print Management app")
	}
	return rootnode, nil
}
