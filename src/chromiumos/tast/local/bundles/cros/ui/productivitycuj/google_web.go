// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package productivitycuj

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

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

	// docsTab indicates the tab name of the "Google Docs".
	docsTab = "Google Docs"
	// slidesTab indicates the tab name of the "Google Slides".
	slidesTab = "Google Slides"
	// sheetsTab indicates the tab name of the "Google Sheets".
	sheetsTab = "Google Sheets"
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
	conn, err := app.cr.NewConn(ctx, sheetsURL)
	if err != nil {
		return "", errors.Wrapf(err, "failed to open URL: %s", sheetsURL)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	// If the file already exists, check the dependent cells first to prevent continuous failure due to incorrect content.
	checkComputeChain := func(ctx context.Context) error {
		for i := 1; i <= rangeOfCells; i++ {
			idx := strconv.Itoa(i)
			cell := fmt.Sprintf("A%d", i)
			value, err := app.getCellValue(ctx, cell)
			if err != nil {
				return err
			}
			if value != idx {
				if err := app.editCellValue(ctx, cell, idx); err != nil {
					return err
				}
			}
		}
		return nil
	}

	section := nodewith.NameRegex(regexp.MustCompile("^(Today|Yesterday|Previous (7|30) days|Earlier).*")).Role(role.ListBox).First()
	fileOption := nodewith.NameContaining(sheetName).Role(role.ListBoxOption).Ancestor(section).First()

	// Check if the sample file exists. If not, create a blank one.
	if err := app.ui.WaitUntilExists(fileOption)(ctx); err != nil {
		return sheetName, app.createSampleSheet(ctx)
	}

	testing.ContextLog(ctx, "Opening an existing spreadsheet")
	return sheetName, uiauto.Combine("open an existing spreadsheet",
		app.uiHdl.ClickUntil(fileOption, app.ui.WithTimeout(time.Second).WaitUntilGone(fileOption)),
		app.validateEditMode,
		uiauto.NamedAction("check dependent compute chain in a spreadsheet", checkComputeChain),
	)(ctx)
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
	testing.ContextLog(ctx, "Moving data from document to spreadsheet")

	// tabIndexMap define the index of the corresponding service tab.
	tabIndexMap := map[string]int{
		docsTab:   0,
		sheetsTab: 2,
	}

	content := nodewith.Name("Document content").Role(role.TextField).Editable()

	if err := uiauto.Combine("cut selected text from the document",
		app.uiHdl.SwitchToChromeTabByIndex(tabIndexMap[docsTab]),
		app.uiHdl.Click(content),
		app.kb.AccelAction("Ctrl+A"),
		app.kb.AccelAction("Ctrl+X"),
	)(ctx); err != nil {
		return err
	}

	if err := uiauto.Combine("switch to Google Sheets and jump to the target cell",
		app.uiHdl.SwitchToChromeTabByIndex(tabIndexMap[sheetsTab]),
		app.maybeCloseEditHistoryDialog,
		app.selectCell("H3"),
	)(ctx); err != nil {
		return err
	}

	return uiauto.Combine("paste content into the cell",
		app.kb.AccelAction("Ctrl+V"),
		app.kb.AccelAction("Enter"),
	)(ctx)
}

// MoveDataFromSheetToDoc moves data from spreadsheet to document.
func (app *GoogleDocs) MoveDataFromSheetToDoc(ctx context.Context) error {
	testing.ContextLog(ctx, "Moving data from document to spreadsheet")

	if err := uiauto.Combine("cut selected text from cell",
		app.selectCell("H1"),
		app.kb.AccelAction("Ctrl+X"),
	)(ctx); err != nil {
		return err
	}

	content := nodewith.Name("Document content").Role(role.TextField).Editable()
	return uiauto.Combine("switch to Google Docs and paste the content",
		app.uiHdl.SwitchToChromeTabByIndex(0),
		app.ui.WaitUntilExists(content),
		app.kb.AccelAction("Ctrl+V"),
	)(ctx)
}

// ScrollPage scrolls the document and spreadsheet.
func (app *GoogleDocs) ScrollPage(ctx context.Context) error {
	testing.ContextLog(ctx, "Scrolling the document and spreadsheet")

	for _, tabIdx := range []int{0, 2} {
		if err := scrollTabPage(ctx, app.uiHdl, tabIdx); err != nil {
			return err
		}
	}

	return nil
}

// SwitchToOfflineMode switches to offline mode.
func (app *GoogleDocs) SwitchToOfflineMode(ctx context.Context) error {
	testing.ContextLog(ctx, "Switching to offline mode")

	menuBar := nodewith.Name("Menu bar").Role(role.Banner)
	fileItem := nodewith.Name("File").Role(role.MenuItem).Ancestor(menuBar)
	offlineCheckBox := nodewith.NameContaining("Make available offline k").Role(role.MenuItemCheckBox)
	complentary := nodewith.Role(role.Complementary)
	onlineMode := nodewith.Name("Document no longer available offline").Role(role.StaticText).Ancestor(complentary)
	turnOnOfflineDialog := nodewith.Name("Turn on offline for all files?").Role(role.Dialog)
	turnOnOfflineButton := nodewith.Name("Turn on").Role(role.Button).Ancestor(turnOnOfflineDialog)
	return uiauto.Combine("switch to offline mode",
		app.uiHdl.Click(fileItem),
		app.uiHdl.Click(offlineCheckBox),
		// If the last click makes the mode change to online mode, click again to switch back to offline mode.
		app.ui.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(onlineMode), app.uiHdl.Click(offlineCheckBox)),
		app.ui.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(turnOnOfflineDialog), app.uiHdl.Click(turnOnOfflineButton)),
	)(ctx)
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

// maybeCloseWelcomeDialog closes the "Welcome to Google Docs/Slides/Sheets" dialog if it exists.
func (app *GoogleDocs) maybeCloseWelcomeDialog(ctx context.Context) error {
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
	blankOption := nodewith.Name("Blank").Role(role.ListBoxOption)
	return uiauto.Combine("open a blank document",
		app.maybeCloseWelcomeDialog,
		app.uiHdl.Click(blankOption),
		app.validateEditMode,
	)(ctx)
}

// selectCell selects the specified cell using the name box.
func (app *GoogleDocs) selectCell(cell string) action.Action {
	nameBox := nodewith.Name("Name box (Ctrl + J)").Role(role.GenericContainer)
	nameField := nodewith.Role(role.TextField).FinalAncestor(nameBox)
	nameFieldFocused := nameField.Focused()
	return uiauto.NamedAction(fmt.Sprintf("to select cell %q", cell),
		uiauto.Combine("click the name box and name a range",
			app.uiHdl.ClickUntil(nameField, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(nameFieldFocused)),
			app.kb.TypeAction(cell),
			app.kb.AccelAction("Enter"),
			// Given time to jump to the specific cell and select it.
			// And because we cannot be sure whether the target cell is focused, we have to wait a short time.
			app.ui.Sleep(500*time.Millisecond),
		),
	)
}

// getCellValue gets the value of the specified cell.
func (app *GoogleDocs) getCellValue(ctx context.Context, cell string) (clipData string, err error) {
	if err := app.selectCell(cell)(ctx); err != nil {
		return "", err
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Due to the unstable network, there might be no data in the clipboard after the copy operation.
		// Therefore, we also need to retry the copy operation.
		if err := app.kb.AccelAction("Ctrl+C")(ctx); err != nil {
			return err
		}
		clipData, err = getClipboardText(ctx, app.tconn)
		if err != nil {
			return err
		}
		if clipData == "Retrieving data. Wait a few seconds and try to cut or copy again." {
			return errors.New("clipboard data is not yet ready")
		}
		return nil
	}, &testing.PollOptions{Timeout: 2 * time.Minute}); err != nil {
		return "", err
	}

	testing.ContextLogf(ctx, "Getting cell %q value: %s", cell, clipData)

	return clipData, nil
}

// editCellValue edits the cell to the specified value.
func (app *GoogleDocs) editCellValue(ctx context.Context, cell, value string) error {
	testing.ContextLogf(ctx, "Writing cell %q value: %s", cell, value)

	return uiauto.Combine(fmt.Sprintf("write cell %q value", cell),
		app.selectCell(cell),
		app.kb.TypeAction(value),
		app.kb.AccelAction("Enter"),
	)(ctx)
}

// createSampleSheet creates a sample spreadsheet.
func (app *GoogleDocs) createSampleSheet(ctx context.Context) error {
	if err := app.openBlankDocument(ctx); err != nil {
		return errors.Wrap(err, "failed to open a blank document")
	}

	testing.ContextLogf(ctx, "Writing cell(A1:A%d) values", rangeOfCells)
	for i := 1; i <= rangeOfCells; i++ {
		idx := strconv.Itoa(i)
		cell := fmt.Sprintf("A%d", i)
		if err := app.editCellValue(ctx, cell, idx); err != nil {
			return errors.Wrapf(err, "failed to edit cell %q", cell)
		}
	}

	formula := fmt.Sprintf("=SUM(A1:A%d)", rangeOfCells)
	testing.ContextLog(ctx, "Writing cell(B1) value")
	if err := app.editCellValue(ctx, "B1", formula); err != nil {
		return errors.Wrap(err, `failed to edit cell "B1"`)
	}

	testing.ContextLog(ctx, "Writing cell(H1) value")
	if err := app.editCellValue(ctx, "H1", "Copy to document"); err != nil {
		return errors.Wrap(err, `failed to edit cell "H1"`)
	}

	menuBar := nodewith.Name("Menu bar").Role(role.Banner)
	fileItem := nodewith.Name("File").Role(role.MenuItem).Ancestor(menuBar)
	renameItem := nodewith.Name("Rename r").Role(role.MenuItem)
	renameField := nodewith.Name("Rename").Role(role.TextField).Editable().Focused()

	testing.ContextLog(ctx, "Renaming spreadsheet")
	return uiauto.Combine("rename then save document",
		app.ui.WaitUntilExists(menuBar),
		// Click "File" and then "Rename".
		app.uiHdl.Click(fileItem),
		app.uiHdl.Click(renameItem),
		app.ui.WaitUntilExists(renameField),
		app.kb.TypeAction(sheetName),
		app.kb.AccelAction("Enter"),
		app.ui.Sleep(2*time.Second), // Wait Google Sheets to save the changes.
	)(ctx)
}

// maybeCloseEditHistoryDialog closes the "See edit history of a cell" dialog if it exists.
func (app *GoogleDocs) maybeCloseEditHistoryDialog(ctx context.Context) error {
	dialog := nodewith.Name("See edit history of a cell").Role(role.Dialog)
	button := nodewith.Name("GOT IT").Role(role.Button).Ancestor(dialog)
	return app.ui.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(dialog), app.uiHdl.Click(button))(ctx)
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
