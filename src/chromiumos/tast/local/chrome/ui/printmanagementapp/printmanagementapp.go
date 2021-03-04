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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// PrintManagementApp represents an instance of the Print Management app.
type PrintManagementApp struct {
	ui    *uiauto.Context
	tconn *chrome.TestConn
}

// printManagementHistoryLabel is used to find the label for the History
// section.
var printManagementHistoryLabel = nodewith.Name("History").Role(role.StaticText)

// printManagementPrintJobEntry is used to find all print job entries.
var printManagementPrintJobEntry = nodewith.ClassName("list-item flex-center")

// Launch Print Management app via default method.
func Launch(ctx context.Context, tconn *chrome.TestConn) (*PrintManagementApp, error) {
	if err := apps.Launch(ctx, tconn, apps.PrintManagement.ID); err != nil {
		return nil, errors.Wrap(err, "failed to launch Print Management app")
	}

	if err := ash.WaitForApp(ctx, tconn, apps.PrintManagement.ID); err != nil {
		return nil, errors.Wrap(err, "Print Management app did not appear in shelf after launch")
	}

	return &PrintManagementApp{ui: uiauto.New(tconn), tconn: tconn}, nil
}

// VerifyHistoryLabel returns an action that verifies the History section of the
// Print Management app is visible.
func (p *PrintManagementApp) VerifyHistoryLabel() uiauto.Action {
	return p.ui.WithTimeout(20 * time.Second).WaitUntilExists(printManagementHistoryLabel)
}

// VerifyPrintJob returns an action that verifies at least one print job is
// visible in the Print Management app.
func (p *PrintManagementApp) VerifyPrintJob() uiauto.Action {
	return p.ui.WithTimeout(20 * time.Second).WaitUntilExists(printManagementPrintJobEntry)
}
