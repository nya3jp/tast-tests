// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontlineworkercuj

import (
	"context"

	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
)

// GoogleSheets holds the information used to do Google Sheets testing.
type GoogleSheets struct {
	cr           *chrome.Chrome
	tconn        *chrome.TestConn
	ui           *uiauto.Context
	uiHdl        cuj.UIActionHandler
	kb           *input.KeyboardEventWriter
	conn         *chrome.Conn
	account      string
	password     string
	sheetCreated bool
}

// NewGoogleSheets returns the the manager of Google Sheets, caller will able to control Google Sheets app through this object.
func NewGoogleSheets(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, ui *uiauto.Context,
	uiHdl cuj.UIActionHandler, kb *input.KeyboardEventWriter, account, password string) *GoogleSheets {
	return &GoogleSheets{
		cr:       cr,
		tconn:    tconn,
		ui:       ui,
		uiHdl:    uiHdl,
		kb:       kb,
		account:  account,
		password: password,
	}
}

// CopySpreadSheet copies the sample spreadsheet.
func (g *GoogleSheets) CopySpreadSheet(ctx context.Context, sampleSheetURL string) (sheetName string, err error) {
	return sheetName, nil
}

// CreatePivotTable creates the pivot table.
func (g *GoogleSheets) CreatePivotTable(ctx context.Context) error {
	return nil
}

// EditPivotTable opens the pivot table editor and add settings.
func (g *GoogleSheets) EditPivotTable(ctx context.Context) error {
	return nil
}

// ValidatePivotTable validates that the values in the pivot table meet our expectations.
func (g *GoogleSheets) ValidatePivotTable(ctx context.Context) error {
	return nil
}

// login logs in to the browser.
// Since we are now using a guest session, we need to log in to the browser.
func (g *GoogleSheets) login(ctx context.Context) error {
	return nil
}

// maybeCloseWelcomeDialog closes the "Welcome to Google Sheets" dialog if it exists.
func (g *GoogleSheets) maybeCloseWelcomeDialog(ctx context.Context) error {
	return nil
}

// validateEditMode checks if the share button exists to confirm whether to enter the edit mode.
func (g *GoogleSheets) validateEditMode(ctx context.Context) error {
	return nil
}

// openBlankDocument opens a blank document for the specified service.
func (g *GoogleSheets) openBlankDocument(ctx context.Context) error {
	return nil
}

// renameFile renames the name of the spreadsheet.
func (g *GoogleSheets) renameFile(ctx context.Context, sheetName string) error {
	return nil
}

// removeFile removes the spreadsheet.
func (g *GoogleSheets) removeFile(ctx context.Context, sheetName *string) error {
	return nil
}

// getClipboardText gets the clipboard text data.
func getClipboardText(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	return "", nil
}

// getCellValue gets the value of the specified cell.
func (g *GoogleSheets) getCellValue(ctx context.Context, cell string) (clipData string, err error) {
	return clipData, nil
}

// editCellValue edits the cell to the specified value.
func (g *GoogleSheets) editCellValue(ctx context.Context, cell, value string) error {
	return nil
}

// fillInPivotRange enters the pivot table data range in the range text box if it hasn't been filled in yet.
func (g *GoogleSheets) fillInPivotRange(ctx context.Context) error {
	return nil
}

// Close closes the Google Sheets tab.
func (g *GoogleSheets) Close(ctx context.Context) {
	return
}
