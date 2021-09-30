// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package productivitycuj

import (
	"context"
	"regexp"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	// docsURL indicates the homepage URL of Google Docs.
	docsURL = "https://docs.google.com/document"
	// slidesURL indicates the homepage URL of Google Slides.
	slidesURL = "http://docs.google.com/slides"
	// sheetsURL indicates the homepage URL of Google Sheets.
	sheetsURL = "http://docs.google.com/spreadsheets"
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
	testing.ContextLog(ctx, "Opening a new document")

	conn, err := app.cr.NewConn(ctx, docsURL)
	if err != nil {
		return errors.Wrapf(err, "failed to open URL: %s", docsURL)
	}
	app.tabs = append(app.tabs, conn)

	return uiauto.Combine("open a new document",
		app.openBlankDocument,
		app.kb.TypeAction(docText),
	)(ctx)
}

// CreateSlides creates a new presentation from GDocs.
func (app *GoogleDocs) CreateSlides(ctx context.Context) error {
	testing.ContextLog(ctx, "Opening a new presentation")

	conn, err := app.cr.NewConn(ctx, slidesURL)
	if err != nil {
		return errors.Wrapf(err, "failed to open URL: %s", slidesURL)
	}
	app.tabs = append(app.tabs, conn)

	slidesWebArea := nodewith.NameContaining("Google Slides").Role(role.RootWebArea)
	title := nodewith.Name("title").Role(role.StaticText).Ancestor(slidesWebArea)
	subtitle := nodewith.Name("subtitle").Role(role.StaticText).Ancestor(slidesWebArea)
	return uiauto.Combine("open a new presentation",
		app.openBlankDocument,
		app.uiHdl.Click(title),
		app.kb.TypeAction(titleText),
		app.uiHdl.Click(subtitle),
		app.kb.TypeAction(subtitleText),
	)(ctx)
}

// CreateSpreadsheet creates a new spreadsheet and fill default data.
func (app *GoogleDocs) CreateSpreadsheet(ctx context.Context) (string, error) {
	return "", nil
}

// OpenSpreadsheet creates a new document from GDocs.
func (app *GoogleDocs) OpenSpreadsheet(ctx context.Context, filename string) error {
	testing.ContextLog(ctx, "Opening an existing spreadsheet: ", filename)

	conn, err := app.cr.NewConn(ctx, sheetsURL)
	if err != nil {
		return errors.Wrapf(err, "failed to open URL: %s", sheetsURL)
	}
	app.tabs = append(app.tabs, conn)

	section := nodewith.NameRegex(regexp.MustCompile("^(Today|Yesterday|Previous (7|30) days|Earlier).*")).Role(role.ListBox).First()
	fileOption := nodewith.NameContaining(sheetName).Role(role.ListBoxOption).Ancestor(section).First()
	return uiauto.Combine("search file from recently opened",
		app.uiHdl.Click(fileOption),
		app.validateEditMode,
	)(ctx)
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
func (app *GoogleDocs) VoiceToTextTesting(ctx context.Context, expectedText string, playAudio action.Action) error {
	return nil
}

// End cleans up GDocs resources. Remove the document and slide which we created in the test case and close all tabs.
func (app *GoogleDocs) End(ctx context.Context) error {
	return nil
}

// closeWelcomeDialog closes the "Welcome to Google Docs/Slides/Sheets" dialog.
func (app *GoogleDocs) closeWelcomeDialog(ctx context.Context) error {
	welcomeDialog := nodewith.NameRegex(regexp.MustCompile("^Welcome to Google (Docs|Slides|Sheets)$")).Role(role.Dialog)
	closeButton := nodewith.Name("Close").Ancestor(welcomeDialog)
	return app.ui.IfSuccessThen(
		app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(welcomeDialog),
		app.uiHdl.Click(closeButton),
	)(ctx)
}

// validateEditMode checks if the share button exists to confirm whether to enter the edit mode.
func (app *GoogleDocs) validateEditMode(ctx context.Context) error {
	shareButton := nodewith.Name("Share. Private to only me. ").Role(role.Button)
	// Make sure share button exists to ensure to enter the edit mode. This is especially necessary on low-end DUTs.
	return app.ui.WithTimeout(longerUIWaitTime).WaitUntilExists(shareButton)(ctx)
}

// openBlankDocument opens a blank document for the specified service.
func (app *GoogleDocs) openBlankDocument(ctx context.Context) error {
	testing.ContextLog(ctx, "Opening a blank document")

	blankOption := nodewith.Name("Blank").Role(role.ListBoxOption)
	return uiauto.Combine("open a blank document",
		app.closeWelcomeDialog,
		app.uiHdl.Click(blankOption),
		app.validateEditMode,
	)(ctx)
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
