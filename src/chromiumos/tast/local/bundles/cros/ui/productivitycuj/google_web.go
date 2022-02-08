// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package productivitycuj

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
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
}

// CreateDocument creates a new document from GDocs.
func (app *GoogleDocs) CreateDocument(ctx context.Context) error {
	_, err := app.cr.NewConn(ctx, cuj.GoogleDocsURL)
	if err != nil {
		return errors.Wrapf(err, "failed to open URL: %s", cuj.GoogleDocsURL)
	}

	return uiauto.Combine("open a new document",
		app.openBlankDocument,
		app.kb.TypeAction(docText),
	)(ctx)
}

// CreateSlides creates a new presentation from GDocs.
func (app *GoogleDocs) CreateSlides(ctx context.Context) error {
	_, err := app.cr.NewConn(ctx, cuj.GoogleSlidesURL)
	if err != nil {
		return errors.Wrapf(err, "failed to open URL: %s", cuj.GoogleSlidesURL)
	}

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
func (app *GoogleDocs) CreateSpreadsheet(ctx context.Context, sampleSheetURL string) (string, error) {
	conn, err := app.cr.NewConn(ctx, sampleSheetURL+"/copy")
	if err != nil {
		return "", errors.Wrapf(err, "failed to open URL: %s", sampleSheetURL)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	copyButton := nodewith.Name("Make a copy").Role(role.Button)
	if err := app.uiHdl.Click(copyButton)(ctx); err != nil {
		return "", errors.Wrap(err, "failed to open the copied data spreadsheet")
	}

	if err := app.renameFile(sheetName)(ctx); err != nil {
		return "", err
	}
	return sheetName, nil
}

// OpenSpreadsheet creates a new document from GDocs.
func (app *GoogleDocs) OpenSpreadsheet(ctx context.Context, filename string) error {
	testing.ContextLog(ctx, "Opening an existing spreadsheet: ", filename)

	_, err := app.cr.NewConn(ctx, cuj.GoogleSheetsURL)
	if err != nil {
		return errors.Wrapf(err, "failed to open URL: %s", cuj.GoogleSheetsURL)
	}

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
	return nil
}

// UpdateCells updates one of the independent cells and propagate values to dependent cells.
func (app *GoogleDocs) UpdateCells(ctx context.Context) error {
	if err := app.maybeCloseEditHistoryDialog(ctx); err != nil {
		return errors.Wrap(err, `failed to close "See edit history of a cell" dialog`)
	}

	if err := app.editCellValue(ctx, "A3", "100"); err != nil {
		return errors.Wrap(err, "failed to edit the value of the cell")
	}

	val, err := app.getCellValue(ctx, "B1")
	if err != nil {
		return errors.Wrap(err, "failed to get the value of the cell")
	}

	sum, err := strconv.Atoi(val)
	if err != nil {
		return errors.Wrap(err, "failed to convert type to integer")
	}

	if expectedSum := calculateSum(3, 100); sum != expectedSum {
		return errors.Errorf("failed to validate the sum %d rows: got: %v; want: %v", rangeOfCells, sum, expectedSum)
	}
	return nil
}

// VoiceToTextTesting uses the "Dictate" function to achieve voice-to-text (VTT) and directly input text into office documents.
func (app *GoogleDocs) VoiceToTextTesting(ctx context.Context, expectedText string, playAudio action.Action) error {
	testing.ContextLog(ctx, "Using voice to text (VTT) to enter text directly to document")

	// allowPermission allows microphone if browser asks for the permission.
	allowPermission := func(ctx context.Context) error {
		alertDialog := nodewith.NameContaining("Use your microphone").ClassName("RootView").Role(role.AlertDialog).First()
		allowButton := nodewith.Name("Allow").Role(role.Button).Ancestor(alertDialog)
		if err := app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(allowButton)(ctx); err != nil {
			testing.ContextLog(ctx, "No action to grant microphone permission")
			return nil
		}
		return app.uiHdl.ClickUntil(allowButton, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilGone(alertDialog))(ctx)
	}

	checkDictationResult := func(ctx context.Context) error {
		testing.ContextLog(ctx, "Check if the result is as expected")

		return testing.Poll(ctx, func(ctx context.Context) error {
			if err := uiauto.Combine("copy the content of the document to the clipboard",
				app.kb.AccelAction("Ctrl+A"),
				app.ui.Sleep(500*time.Millisecond), // Wait for all text to be selected.
				app.kb.AccelAction("Ctrl+C"),
			)(ctx); err != nil {
				return err
			}

			clipData, err := getClipboardText(ctx, app.tconn)
			if err != nil {
				return err
			}
			ignoreCaseData := strings.TrimSuffix(strings.ToLower(clipData), "\n")
			if !strings.Contains(ignoreCaseData, strings.ToLower(expectedText)) {
				return errors.Errorf("failed to validate input value ignoring case: got: %s; want: %s", clipData, expectedText)
			}
			return nil
		}, &testing.PollOptions{Interval: time.Second, Timeout: 15 * time.Second})
	}

	docsWebArea := nodewith.NameContaining("Google Docs").Role(role.RootWebArea)
	menuBar := nodewith.Role(role.MenuBar).Ancestor(docsWebArea)
	tools := nodewith.Name("Tools").Role(role.MenuItem).FinalAncestor(menuBar)
	toolsExpanded := tools.Expanded()
	voiceTypingItem := nodewith.Name("Voice typing v Ctrl+Shift+S").Role(role.MenuItem)
	voiceTypingDialog := nodewith.Name("Voice typing").Role(role.Dialog)
	dictationButton := nodewith.Name("Start dictation").Role(role.ToggleButton).FinalAncestor(voiceTypingDialog)

	// Click "Tools" and then "Voice typing". A microphone box appears.
	// Click the microphone box when ready to speak.
	if err := uiauto.Combine("turn on the voice typing",
		app.uiHdl.SwitchToChromeTabByIndex(0), // Switch to Google Docs.
		app.closeDialogs,
		app.uiHdl.ClickUntil(tools, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(toolsExpanded)),
		app.ui.WaitUntilExists(voiceTypingItem), // Sometimes the "Voice typing" menu item can be found but UI has not showed up yet.
		app.uiHdl.Click(voiceTypingItem),
		app.uiHdl.ClickUntil(dictationButton, app.checkDictionButton),
		allowPermission,
	)(ctx); err != nil {
		return err
	}

	return uiauto.Combine("play an audio file and check dictation results",
		app.uiHdl.ClickUntil(dictationButton, app.checkDictionButton), // Make sure that the dictation does not stop by waiting a long time.
		playAudio,
		checkDictationResult,
		app.uiHdl.Click(dictationButton), // Click again to stop voice typing.
	)(ctx)
}

// Cleanup cleans up the resources used by running the GDocs testing.
// It removes the document and slide which we created in the test case and close all tabs after completing the test.
// This function should be called as deferred function after the app is created.
func (app *GoogleDocs) Cleanup(ctx context.Context) error {
	// tabIndexMap define the index of the corresponding service tab.
	tabIndexMap := map[string]int{
		docsTab:   0,
		slidesTab: 1,
		sheetsTab: 2,
	}

	menuBar := nodewith.Name("Menu bar").Role(role.Banner)
	file := nodewith.Name("File").Role(role.MenuItem).Ancestor(menuBar)
	moveToTrash := nodewith.NameContaining("Move to trash t").Role(role.MenuItem)
	dialog := nodewith.Name("File moved to trash").Role(role.Dialog)
	homeScreen := nodewith.NameRegex(regexp.MustCompile("^Go to (Docs|Slides|Sheets) home screen")).Role(role.Button).Ancestor(dialog)

	for k, v := range tabIndexMap {
		testing.ContextLogf(ctx, "Switching to %q", k)
		if err := uiauto.Combine("remove the file",
			app.uiHdl.SwitchToChromeTabByIndex(v),
			app.closeDialogs,
			app.uiHdl.Click(file),
			app.uiHdl.Click(moveToTrash),
			app.uiHdl.Click(homeScreen),
		)(ctx); err != nil {
			return err
		}
	}
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
	blankOption := nodewith.NameContaining("Blank").Role(role.ListBoxOption)
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

// renameFile renames the name of the spreadsheet.
func (app *GoogleDocs) renameFile(sheetName string) uiauto.Action {
	menuBar := nodewith.Name("Menu bar").Role(role.Banner)
	fileItem := nodewith.Name("File").Role(role.MenuItem).Ancestor(menuBar)
	fileMenu := nodewith.Role(role.Menu)
	renameItem := nodewith.Name("Rename r").Role(role.MenuItem).Ancestor(fileMenu)
	renameField := nodewith.Name("Rename").Role(role.TextField).Editable().Focused()
	return uiauto.Combine("rename the file",
		app.ui.WaitUntilExists(menuBar),
		app.uiHdl.ClickUntil(fileItem, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(fileMenu)),
		app.uiHdl.ClickUntil(renameItem, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(renameField)),
		app.kb.TypeAction(sheetName),
		app.kb.AccelAction("Enter"),
		app.ui.Sleep(2*time.Second), // Wait Google Sheets to save the changes.
	)
}

// maybeCloseEditHistoryDialog closes the "See edit history of a cell" dialog if it exists.
func (app *GoogleDocs) maybeCloseEditHistoryDialog(ctx context.Context) error {
	dialog := nodewith.Name("See edit history of a cell").Role(role.Dialog)
	button := nodewith.Name("GOT IT").Role(role.Button).Ancestor(dialog)
	return app.ui.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(dialog), app.uiHdl.Click(button))(ctx)
}

// closeDialogs closes dialogs if the display is different from what we expected.
func (app *GoogleDocs) closeDialogs(ctx context.Context) error {
	testing.ContextLog(ctx, "Checking if any dialogs need to be closed")

	unableToLoadDialog := nodewith.Name("Unable to load file").Role(role.Dialog)
	dialogButton := nodewith.Name("Reload").Ancestor(unableToLoadDialog).First()
	reloadContainer := nodewith.Name("Reload to allow offline editing. Reload").Role(role.GenericContainer)
	reloadButton := nodewith.Name("Reload").Role(role.Button).FinalAncestor(reloadContainer)

	// dialogsInfo holds the information of dialogs that will be encountered and needs to be handled during testing.
	// The order of slices starts with the most frequent occurrence.
	dialogsInfo := []dialogInfo{
		{
			name:   "Unable to load file",
			dialog: unableToLoadDialog,
			node:   dialogButton,
		},
		{
			name:   "Reload to allow offline editing. Reload",
			dialog: reloadContainer,
			node:   reloadButton,
		},
	}

	for _, info := range dialogsInfo {
		name, dialog, button := info.name, info.dialog, info.node

		testing.ContextLogf(ctx, "Checking if the %q dialog exists", name)
		if err := app.ui.WaitUntilExists(dialog)(ctx); err != nil {
			continue
		}
		return app.uiHdl.ClickUntil(button, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilGone(button))(ctx)
	}

	return nil
}

// checkDictionButton checks if the "Start dictation" button is checked.
func (app *GoogleDocs) checkDictionButton(ctx context.Context) error {
	voiceTypingDialog := nodewith.Name("Voice typing").Role(role.Dialog)
	dictationButton := nodewith.Name("Start dictation").Role(role.ToggleButton).FinalAncestor(voiceTypingDialog)
	return testing.Poll(ctx, func(ctx context.Context) error {
		button, err := app.ui.Info(ctx, dictationButton)
		if err != nil {
			return err
		}
		if button.Checked != checked.True {
			return errors.New("button not checked yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second})
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
