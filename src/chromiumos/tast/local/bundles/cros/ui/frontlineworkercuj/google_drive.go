// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontlineworkercuj

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
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
func (g *GoogleDrive) Launch(ctx context.Context) (time.Duration, error) {
	return 0, nil
}

// OpenSpreadSheet opens the spreadsheet with pivot table.
func (g *GoogleDrive) OpenSpreadSheet(ctx context.Context, sheetName string) error {
	return nil
}
