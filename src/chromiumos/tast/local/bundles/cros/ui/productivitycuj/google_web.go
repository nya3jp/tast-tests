// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package productivitycuj

import (
	"context"

	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
)

// GoogleDocs implements the ProductivityApp interface.
type GoogleDocs struct {
	cr    *chrome.Chrome
	tconn *chrome.TestConn
	ui    *uiauto.Context
	kb    *input.KeyboardEventWriter
	uiHdl cuj.UIActionHandler
	tabs  []*chrome.Conn
}

// CreateDocument creates a new document from GDocs.
func (app *GoogleDocs) CreateDocument(ctx context.Context) error {
	return nil
}

// CreateSlides creates a new presentation from GDocs.
func (app *GoogleDocs) CreateSlides(ctx context.Context) error {
	return nil
}

// CreateSpreadsheet creates a new spreadsheet and fill default data.
func (app *GoogleDocs) CreateSpreadsheet(ctx context.Context) (string, error) {
	return "", nil
}

// OpenSpreadsheet creates a new document from GDocs.
func (app *GoogleDocs) OpenSpreadsheet(ctx context.Context, filename string) error {
	return nil
}

// MoveDataFromDocToSheet moves data from document to spreadsheet.
func (app *GoogleDocs) MoveDataFromDocToSheet(ctx context.Context) error {
	return nil
}

// MoveDataFromSheetToDoc moves data from spreadsheet to document.
func (app *GoogleDocs) MoveDataFromSheetToDoc(ctx context.Context) error {
	return nil
}

// ScrollPage scrolls the document and spreadsheet.
func (app *GoogleDocs) ScrollPage(ctx context.Context) error {
	return nil
}

// SwitchToOfflineMode switches to offline mode.
func (app *GoogleDocs) SwitchToOfflineMode(ctx context.Context) error {
	return nil
}

// UpdateCells updates one of the independent cells and propagate values to dependent cells.
func (app *GoogleDocs) UpdateCells(ctx context.Context) error {
	return nil
}

// VoiceToTextTesting uses the "Dictate" function to achieve voice-to-text (VTT) and directly input text into office documents.
func (app *GoogleDocs) VoiceToTextTesting(ctx context.Context, expectedText string, playAudio func(context.Context) error) error {
	return nil
}

// End cleans up GDocs resources.
// Remove the document and slide which we created in the test case and close all tabs.
func (app *GoogleDocs) End(ctx context.Context) error {
	return nil
}

// NewGoogleDocs creates GoogleDocs instance which implements ProductivityApp interface.
func NewGoogleDocs(cr *chrome.Chrome, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, uiHdl cuj.UIActionHandler) *GoogleDocs {
	return &GoogleDocs{
		cr:    cr,
		tconn: tconn,
		ui:    uiauto.New(tconn),
		kb:    kb,
		uiHdl: uiHdl,
	}
}

var _ ProductivityApp = (*GoogleDocs)(nil)
