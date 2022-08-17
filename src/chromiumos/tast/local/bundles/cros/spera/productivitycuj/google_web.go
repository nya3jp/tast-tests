// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
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

var (
	menuBarBanner = nodewith.Name("Menu bar").Role(role.Banner)
	fileItem      = nodewith.Name("File").Role(role.MenuItem).Ancestor(menuBarBanner)
	fileExpanded  = fileItem.Expanded()
)

// GoogleDocs implements the ProductivityApp interface.
type GoogleDocs struct {
	br         *browser.Browser
	tconn      *chrome.TestConn
	ui         *uiauto.Context
	kb         *input.KeyboardEventWriter
	uiHdl      cuj.UIActionHandler
	tabletMode bool
}

// CreateDocument creates a new document from GDocs.
func (app *GoogleDocs) CreateDocument(ctx context.Context) error {
	conn, err := app.br.NewConn(ctx, cuj.NewGoogleDocsURL)
	if err != nil {
		return errors.Wrapf(err, "failed to open URL: %s", cuj.GoogleDocsURL)
	}
	if err := webutil.WaitForQuiescence(ctx, conn, longerUIWaitTime); err != nil {
		return errors.Wrap(err, "failed to wait for page to finish loading")
	}
	if err := cuj.MaximizeBrowserWindow(ctx, app.tconn, app.tabletMode, docsTab); err != nil {
		return errors.Wrap(err, "failed to maximize the Google Docs page")
	}
	docWebArea := nodewith.NameContaining(docsTab).Role(role.RootWebArea).First()
	canvas := nodewith.Role(role.Canvas).Ancestor(docWebArea).First()
	return uiauto.Combine("type word to document",
		app.maybeCloseWelcomeDialog,
		app.ui.WaitUntilExists(canvas),
		app.kb.TypeAction(docText),
	)(ctx)
}

// CreateSlides creates a new presentation from GDocs.
func (app *GoogleDocs) CreateSlides(ctx context.Context) error {
	conn, err := app.br.NewConn(ctx, cuj.NewGoogleSlidesURL)
	if err != nil {
		return errors.Wrapf(err, "failed to open URL: %s", cuj.GoogleSlidesURL)
	}
	if err := webutil.WaitForQuiescence(ctx, conn, longerUIWaitTime); err != nil {
		return errors.Wrap(err, "failed to wait for page to finish loading")
	}
	slidesWebArea := nodewith.NameContaining("Google Slides").Role(role.RootWebArea)
	title := nodewith.Name("title").Role(role.StaticText).Ancestor(slidesWebArea)
	subtitle := nodewith.Name("subtitle").Role(role.StaticText).Ancestor(slidesWebArea)
	return uiauto.Combine("open a new presentation",
		app.uiHdl.Click(title),
		app.kb.TypeAction(titleText),
		app.uiHdl.Click(subtitle),
		app.kb.TypeAction(subtitleText),
	)(ctx)
}

// CreateSpreadsheet creates a new spreadsheet by copying from sample spreadsheet.
func (app *GoogleDocs) CreateSpreadsheet(ctx context.Context, cr *chrome.Chrome, sampleSheetURL, outDir string) (string, error) {
	conn, err := app.br.NewConn(ctx, sampleSheetURL+"/copy")
	if err != nil {
		return "", errors.Wrapf(err, "failed to open URL: %s", sampleSheetURL)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)
	if err := webutil.WaitForQuiescence(ctx, conn, longerUIWaitTime); err != nil {
		return "", errors.Wrap(err, "failed to wait for page to finish loading")
	}
	copyButton := nodewith.Name("Make a copy").Role(role.Button)
	if err := app.ui.DoDefault(copyButton)(ctx); err != nil {
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
	conn, err := app.br.NewConn(ctx, cuj.GoogleSheetsURL)
	if err != nil {
		return errors.Wrapf(err, "failed to open URL: %s", cuj.GoogleSheetsURL)
	}
	if err := webutil.WaitForQuiescence(ctx, conn, longerUIWaitTime); err != nil {
		return errors.Wrap(err, "failed to wait for page to finish loading")
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
	content := nodewith.Name("Document content").Role(role.TextField).Editable()

	if err := uiauto.Combine("cut selected text from the document",
		app.uiHdl.SwitchToChromeTabByName(docsTab),
		app.uiHdl.Click(content),
		app.kb.AccelAction("Ctrl+A"),
		app.kb.AccelAction("Ctrl+X"),
	)(ctx); err != nil {
		return err
	}

	if err := uiauto.Combine("switch to Google Sheets and jump to the target cell",
		app.uiHdl.SwitchToChromeTabByName(sheetsTab),
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
		app.uiHdl.SwitchToChromeTabByName(docsTab),
		app.ui.WaitUntilExists(content),
		app.kb.AccelAction("Ctrl+V"),
	)(ctx)
}

// ScrollPage scrolls the document and spreadsheet.
func (app *GoogleDocs) ScrollPage(ctx context.Context) error {
	testing.ContextLog(ctx, "Scrolling the document and spreadsheet")
	for _, tabName := range []string{docsTab, sheetsTab} {
		if err := scrollTabPageByName(ctx, app.uiHdl, tabName); err != nil {
			return err
		}
	}
	return nil
}

// SwitchToOfflineMode switches to offline mode and switches back to online mode.
func (app *GoogleDocs) SwitchToOfflineMode(ctx context.Context) error {
	expandFileMenu := uiauto.IfSuccessThen(
		app.ui.WithTimeout(defaultUIWaitTime).WaitUntilGone(fileExpanded),
		app.uiHdl.ClickUntil(fileItem, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(fileExpanded)),
	)

	removeOffline := nodewith.Role(role.MenuItemCheckBox).Name("Remove offline access k")
	checkOffline := uiauto.NamedCombine("check whether offline mode is available",
		expandFileMenu,
		app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(removeOffline),
	)

	makeOfflineModeAvailable := uiauto.NamedCombine("make offline mode available",
		expandFileMenu,
		app.kb.TypeAction("k"),
		checkOffline,
		uiauto.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(fileExpanded), app.uiHdl.Click(fileItem)),
	)

	return uiauto.IfFailThen(checkOffline, makeOfflineModeAvailable)(ctx)
}

// UpdateCells updates one of the independent cells and propagate values to dependent cells.
func (app *GoogleDocs) UpdateCells(ctx context.Context) error {
	if err := app.maybeCloseEditHistoryDialog(ctx); err != nil {
		return errors.Wrap(err, `failed to close "See edit history of a cell" dialog`)
	}

	if err := app.editCellValue(ctx, "A3", "100"); err != nil {
		return errors.Wrap(err, "failed to edit the value of the cell")
	}

	var sum int
	if err := uiauto.Retry(retryTimes, func(ctx context.Context) error {
		val, err := app.getCellValue(ctx, "B1")
		if err != nil {
			return errors.Wrap(err, "failed to get the value of the cell")
		}

		sum, err = strconv.Atoi(val)
		if err != nil {
			return errors.Wrap(err, "failed to convert type to integer")
		}
		return nil
	})(ctx); err != nil {
		return err
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
		return app.ui.DoDefaultUntil(allowButton, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilGone(alertDialog))(ctx)
	}

	checkDictationResult := func(ctx context.Context) error {
		testing.ContextLog(ctx, "Check if the result is as expected")

		return testing.Poll(ctx, func(ctx context.Context) error {
			if err := uiauto.Combine("copy the content of the document to the clipboard",
				app.kb.AccelAction("Ctrl+A"),
				uiauto.Sleep(500*time.Millisecond), // Wait for all text to be selected.
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

	pasteTableContainer := nodewith.Name("Paste table").Role(role.GenericContainer)
	cancelButton := nodewith.Name("Cancel").Role(role.Button).Ancestor(pasteTableContainer)
	tools := nodewith.Name("Tools").Role(role.MenuItem).Ancestor(menuBarBanner)
	toolsExpanded := tools.Expanded()
	voiceTypingItem := nodewith.Name("Voice typing v Ctrl+Shift+S").Role(role.MenuItem)
	voiceTypingDialog := nodewith.Name("Voice typing").Role(role.Dialog)
	dictationButton := nodewith.Name("Start dictation").Role(role.ToggleButton).FinalAncestor(voiceTypingDialog)

	// Click "Tools" and then "Voice typing". A microphone box appears.
	// Click the microphone box when ready to speak.
	if err := uiauto.Combine("turn on the voice typing",
		app.uiHdl.SwitchToChromeTabByName(docsTab),
		app.closeDialogs,
		uiauto.IfSuccessThen(
			app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(cancelButton),
			uiauto.NamedAction("click the cancel button", app.uiHdl.Click(cancelButton)),
		),
		app.ui.DoDefaultUntil(tools, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(toolsExpanded)),
		app.ui.WaitUntilExists(voiceTypingItem), // Sometimes the "Voice typing" menu item can be found but UI has not showed up yet.
		app.ui.DoDefault(voiceTypingItem),
		app.ui.DoDefaultUntil(dictationButton, app.checkDictionButton),
		allowPermission,
	)(ctx); err != nil {
		return err
	}

	return uiauto.Combine("play an audio file and check dictation results",
		// Make sure that the dictation does not stop by waiting a long time.
		uiauto.IfFailThen(app.checkDictionButton,
			app.ui.DoDefaultUntil(dictationButton, app.checkDictionButton)),
		playAudio,
		checkDictationResult,
		app.uiHdl.Click(dictationButton), // Click again to stop voice typing.
	)(ctx)
}

// Cleanup cleans up the resources used by running the GDocs testing.
// It removes the document and slide which we created in the test case and close all tabs after completing the test.
// This function should be called as deferred function after the app is created.
func (app *GoogleDocs) Cleanup(ctx context.Context, sheetName string) error {
	moveToTrash := nodewith.NameContaining("Move to trash t").Role(role.MenuItem)
	dialog := nodewith.Name("File moved to trash").Role(role.Dialog)
	homeScreen := nodewith.NameRegex(regexp.MustCompile("^Go to (Docs|Slides|Sheets) home screen")).Role(role.Button).Ancestor(dialog)
	for _, tabName := range []string{docsTab, slidesTab, sheetsTab} {
		if err := uiauto.NamedCombine("remove the "+tabName,
			app.uiHdl.SwitchToChromeTabByName(tabName),
			app.closeDialogs,
			app.uiHdl.Click(fileItem),
			app.uiHdl.Click(moveToTrash),
			app.uiHdl.Click(homeScreen),
		)(ctx); err != nil {
			return err
		}
	}
	return nil
}

// SetBrowser sets browser to chrome or lacros.
func (app *GoogleDocs) SetBrowser(br *browser.Browser) {
	app.br = br
}

// maybeCloseWelcomeDialog closes the "Welcome to Google Docs/Slides/Sheets" dialog if it exists.
func (app *GoogleDocs) maybeCloseWelcomeDialog(ctx context.Context) error {
	welcomeDialog := nodewith.NameRegex(regexp.MustCompile("^Welcome to Google (Docs|Slides|Sheets)$")).Role(role.Dialog)
	closeButton := nodewith.Name("Close").Ancestor(welcomeDialog)
	return uiauto.IfSuccessThen(
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

// selectCell selects the specified cell using the name box.
func (app *GoogleDocs) selectCell(cell string) action.Action {
	nameBox := nodewith.Name("Name box (Ctrl + J)").Role(role.GenericContainer)
	nameField := nodewith.Role(role.TextField).FinalAncestor(nameBox)
	nameFieldFocused := nameField.Focused()
	nameFieldText := nodewith.Name(cell).Role(role.StaticText).Ancestor(nameField).Editable()
	return uiauto.NamedCombine(fmt.Sprintf("to select cell %q", cell),
		app.ui.DoDefaultUntil(nameField, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(nameFieldFocused)),
		app.kb.AccelAction("Ctrl+A"),
		app.kb.TypeAction(cell),
		app.kb.AccelAction("Enter"),
		app.ui.WaitUntilExists(nameFieldText),
		// Given time to jump to the specific cell and select it.
		// And because we cannot be sure whether the target cell is focused, we have to wait a short time.
		uiauto.Sleep(500*time.Millisecond),
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
	workingText := nodewith.Name("Working…").Role(role.StaticText)
	return uiauto.NamedCombine(fmt.Sprintf("write cell %q value: %v", cell, value),
		app.selectCell(cell),
		app.kb.TypeAction(value),
		app.kb.AccelAction("Enter"),
		uiauto.IfSuccessThen(
			app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(workingText),
			app.ui.WaitUntilGone(workingText),
		),
	)(ctx)
}

// renameFile renames the name of the spreadsheet.
func (app *GoogleDocs) renameFile(sheetName string) uiauto.Action {
	renameItem := nodewith.Name("Rename r").Role(role.MenuItem)
	renameField := nodewith.Name("Rename").Role(role.TextField).Editable().Focused()

	inputFileName := func(ctx context.Context) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			if err := uiauto.Combine("input file name",
				app.kb.AccelAction("Ctrl+A"),
				app.kb.TypeAction(sheetName),
			)(ctx); err != nil {
				return err
			}
			node, err := app.ui.Info(ctx, renameField)
			if err != nil {
				return err
			}
			if node.Value != sheetName {
				return errors.New("file name is incorrect")
			}
			return nil
		}, &testing.PollOptions{Timeout: time.Minute})
	}

	return uiauto.Combine("rename the file",
		app.validateEditMode,
		app.ui.Retry(retryTimes, uiauto.Combine(`select "Rename" from the "File" menu`,
			uiauto.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(fileItem),
				app.ui.DoDefaultUntil(fileItem, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(fileExpanded))),
			uiauto.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(renameItem),
				app.ui.DoDefaultUntil(renameItem, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(renameField))),
		)),
		uiauto.NamedAction("input the file name", inputFileName),
		app.kb.AccelAction("Enter"),
		uiauto.Sleep(2*time.Second), // Wait Google Sheets to save the changes.
	)
}

// maybeCloseEditHistoryDialog closes the "See edit history of a cell" dialog if it exists.
func (app *GoogleDocs) maybeCloseEditHistoryDialog(ctx context.Context) error {
	dialog := nodewith.Name("See edit history of a cell").Role(role.Dialog)
	button := nodewith.Name("GOT IT").Role(role.Button).Ancestor(dialog)
	return uiauto.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(dialog), app.uiHdl.Click(button))(ctx)
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
	startTime := time.Now()
	voiceTypingDialog := nodewith.Name("Voice typing").Role(role.Dialog)
	dictationButton := nodewith.Name("Start dictation").Role(role.ToggleButton).FinalAncestor(voiceTypingDialog)
	return testing.Poll(ctx, func(ctx context.Context) error {
		button, err := app.ui.WithTimeout(time.Second).Info(ctx, dictationButton)
		if err != nil {
			return err
		}
		if button.Checked != checked.True {
			return errors.New("button not checked yet")
		}

		testing.ContextLogf(ctx, "Takes %v seconds to verify the dictation button is checked", time.Since(startTime).Seconds())
		return nil
	}, &testing.PollOptions{Timeout: defaultUIWaitTime})
}

// NewGoogleDocs creates GoogleDocs instance which implements ProductivityApp interface.
func NewGoogleDocs(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, uiHdl cuj.UIActionHandler, tabletMode bool) *GoogleDocs {
	return &GoogleDocs{
		tconn:      tconn,
		ui:         uiauto.New(tconn),
		kb:         kb,
		uiHdl:      uiHdl,
		tabletMode: tabletMode,
	}
}

var _ ProductivityApp = (*GoogleDocs)(nil)
