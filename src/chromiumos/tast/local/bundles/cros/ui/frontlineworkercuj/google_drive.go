// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontlineworkercuj

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
)

const (
	// googleDrivePackageName indicates the package name of the "Google Drive".
	googleDrivePackageName = "com.google.android.apps.docs"
	// googleDriveAppName indicates the app name of the "Google Drive".
	googleDriveAppName = "Google Drive"
	// driveTab indicates the tab name of the "Google Drive".
	driveTab = "Google Drive"

	// longerUIWaitTime indicates the time to wait for some UI elements that need more time to appear.
	longerUIWaitTime = time.Minute
)

// GoogleDrive holds the information used to do Google Drive testing.
type GoogleDrive struct {
	tconn *chrome.TestConn
	ui    *uiauto.Context
	a     *arc.ARC
	d     *ui.Device
	kb    *input.KeyboardEventWriter
}

// NewGoogleDrive returns the the manager of GoogleDrive, caller will able to control GoogleDrive app through this object.
func NewGoogleDrive(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, kb *input.KeyboardEventWriter) (*GoogleDrive, error) {
	return &GoogleDrive{
		tconn: tconn,
		ui:    ui,
		kb:    kb,
	}, nil
}

// Launch launches the specified app.
func (g *GoogleDrive) Launch(ctx context.Context, tconn *chrome.TestConn) (time.Duration, error) {
	startTime := time.Now()
	// Google Drive App has been installed by Fixture.
	if err := apps.Launch(ctx, tconn, apps.Drive.ID); err != nil {
		return -1, errors.Wrapf(err, "failed to launch %s app", googleDriveAppName)
	}
	return time.Since(startTime), nil
}

// OpenSpreadSheet opens the spreadsheet with pivot table.
func (g *GoogleDrive) OpenSpreadSheet(ctx context.Context, sheetName string) error {
	gotIt := nodewith.Name("Got it").Role(role.Button).Focusable()
	myDrive := nodewith.Name("My Drive - Google Drive").Role(role.RootWebArea)
	sheetOption := nodewith.NameContaining(sheetName).Role(role.ListBoxOption).First()
	googleSheets := nodewith.NameContaining(sheetName).Role(role.RootWebArea)
	return uiauto.Combine("open the spreadsheet with pivot table",
		uiauto.IfSuccessThen(g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(gotIt), g.ui.LeftClick(gotIt)),
		g.ui.WithTimeout(longerUIWaitTime).WaitUntilExists(myDrive),
		g.ui.DoubleClick(sheetOption),
		g.ui.WaitUntilExists(googleSheets),
	)(ctx)
}
