// Copyright 2021 The ChromiumOS Authors
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

	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	// myFiles indicates the "My files" item name in the navigation bar.
	myFiles = "My files"
	// recent indicates the "Recent" item label in the navigation bar.
	recent = "Recent"

	// oneDriveTab indicates the suffix of the tab name.
	oneDriveTab = "OneDrive"
	// myFiles indicates the tab name of the "My files - OneDrive".
	myFilesTab = "My files - OneDrive"
	// recentTab indicates the tab name of the "Recent - OneDrive".
	recentTab = "Recent - OneDrive"
	// wordTab indicates the tab name of the "Microsoft Word".
	wordTab = "Microsoft Word Online"
	// powerpointTab indicates the tab name of the "Microsoft PowerPoint".
	powerpointTab = "Microsoft PowerPoint Online"
	// excelTab indicates the tab name of the "Microsoft Excel".
	excelTab = "Microsoft Excel Online"

	// word indicates the label of the new document.
	word = "Word document"
	// powerpoint indicates the label of the new presentation.
	powerpoint = "PowerPoint presentation"
	// excel indicates the label of the new spreadsheet.
	excel = "Excel workbook"
)

var (
	homeTabPanel = nodewith.Name("Home").Role(role.TabPanel)
	excelWebArea = nodewith.Name("Excel").Role(role.RootWebArea)
	canvas       = nodewith.Role(role.Canvas).Ancestor(excelWebArea).First()
)

// MicrosoftWebOffice implements the ProductivityApp interface.
type MicrosoftWebOffice struct {
	br              *browser.Browser
	tconn           *chrome.TestConn
	ui              *uiauto.Context
	kb              *input.KeyboardEventWriter
	uiHdl           cuj.UIActionHandler
	tabletMode      bool
	username        string
	password        string
	documentCreated bool
	slideCreated    bool
	sheetCreated    bool
	isLacros        bool
}

// CreateDocument creates a new document from microsoft web app.
func (app *MicrosoftWebOffice) CreateDocument(ctx context.Context) error {
	conn, err := app.openOneDrive(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open OneDrive")
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	wordWebArea := nodewith.Name("Word").Role(role.RootWebArea)
	paragraph := nodewith.Role(role.Paragraph).Ancestor(wordWebArea).Editable()
	if err := uiauto.Combine("open a new document",
		app.openBlankDocument(word),
		// Make sure paragraph exists before typing. This is especially necessary on low-end DUTs.
		app.ui.WithTimeout(longerUIWaitTime).WaitUntilExists(paragraph),
		app.kb.TypeAction(docText),
		app.maybeCloseOneDriveTab(myFilesTab),
	)(ctx); err != nil {
		return err
	}
	// Since the file will only be saved automatically after the file is edited, mark the file created successfully here.
	app.documentCreated = true
	return nil
}

// CreateSlides creates a new presentation from microsoft web app.
func (app *MicrosoftWebOffice) CreateSlides(ctx context.Context) error {
	conn, err := app.openOneDrive(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open OneDrive")
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	title := nodewith.Name("Click to add title").First()
	subtitle := nodewith.Name("Click to add subtitle").Role(role.StaticText)
	if err := uiauto.Combine("create a new presentation",
		app.openBlankDocument(powerpoint),
		// Make sure title exists before typing. This is especially necessary on low-end DUTs.
		app.ui.WithTimeout(longerUIWaitTime).WaitUntilExists(title),
		app.uiHdl.Click(title),
		app.kb.TypeAction(titleText),
		app.uiHdl.Click(subtitle),
		app.kb.TypeAction(subtitleText),
		app.kb.AccelAction("Enter"),
		app.maybeCloseOneDriveTab(myFilesTab),
	)(ctx); err != nil {
		return err
	}
	// Since the file will only be saved automatically after the file is edited, mark the file created successfully here.
	app.slideCreated = true
	return nil
}

// CreateSpreadsheet creates a new spreadsheet from the microsoft web app.
// and copy the content from the public shared document to return the sheet name.
// Since MS Office documents cannot directly copy view-only documents,
// we can only copy the contents of worksheets from public shared documents.
func (app *MicrosoftWebOffice) CreateSpreadsheet(ctx context.Context, cr *chrome.Chrome, sampleSheetURL, outDir string) (fileName string, err error) {
	var bTconn *browser.TestConn
	var connExcel, connOneDrive *browser.Conn

	bTconn, err = app.br.TestAPIConn(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to create test API connection")
	}
	closeTabsFunc := browser.CloseAllTabs
	if app.isLacros {
		// For lacros-Chrome, it should leave a new tab to keep the Chrome process alive.
		closeTabsFunc = browser.ReplaceAllTabsWithSingleNewTab
	}
	connExcel, err = app.br.NewConn(ctx, sampleSheetURL)
	if err != nil {
		return "", errors.Wrapf(err, "failed to open URL: %s", sampleSheetURL)
	}

	defer func() {
		faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, func() bool { return err != nil }, cr, "ui_tree")
		connExcel.Close()
		closeTabsFunc(ctx, bTconn)
	}()

	if err = webutil.WaitForQuiescence(ctx, connExcel, longerUIWaitTime); err != nil {
		return "", errors.Wrap(err, "failed to wait for sample sheet page to finish loading")
	}

	// 1. If the account is already logged in, the "We are updating our terms" dialog may pop up after navigating to the sample sheet.
	// 2. If the account is not logged in, a dialog box may pop up after logging in.
	// So check the dialog before and after login.
	if err := uiauto.Combine("wait for sample sheet content appears",
		app.skipUpdatingTermsDialog(),
		app.checkSignIn,
		app.ui.WaitUntilExists(canvas),
	)(ctx); err != nil {
		return "", errors.Wrap(err, "failed to wait sample sheet content appears")
	}

	connOneDrive, err = app.openOneDrive(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to open OneDrive")
	}
	defer connOneDrive.Close()

	if err = uiauto.NamedCombine("remove if the same file name exists",
		app.clickNavigationItem(myFiles),
		app.switchToListView,
		app.sortByModified,
		app.removeSheet(sheetName),
		app.clickNavigationItem(myFiles),
	)(ctx); err != nil {
		return "", errors.Wrap(err, "failed to check if the file already exists")
	}

	copyFromExistingSheet := func(ctx context.Context) error {
		checkCopiedData := func(ctx context.Context) error {
			data, err := getClipboardText(ctx, app.tconn)
			if err != nil {
				return err
			}
			if lines := strings.Fields(data); len(lines) != 100 && !strings.HasPrefix(data, "1") {
				return errors.New("incorrect pasted content")
			}
			return nil
		}
		selectAll := uiauto.Combine("select all",
			app.kb.AccelAction("Ctrl+A"),
			uiauto.Sleep(dataWaitTime), // Given time to select all data.
			app.kb.AccelAction("Ctrl+C"),
			uiauto.Sleep(dataWaitTime), // Given time to copy data.
		)
		copyAll := uiauto.NamedCombine("copy all data",
			app.selectBox("A1"),
			selectAll,
			uiauto.IfFailThen(checkCopiedData, uiauto.Combine("select range with Go To",
				app.selectRangeWithGoTo,
				app.kb.TypeAction("A1"),
				app.kb.AccelAction("Enter"),
				selectAll,
			)),
		)
		return uiauto.Combine("copy from existing spreadsheet",
			app.openBlankDocument(excel),
			app.uiHdl.SwitchToChromeTabByName(excelTab),
			app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(canvas),
			copyAll,
		)(ctx)
	}

	pasteIntoNewSheet := uiauto.Combine("paste into newly created spreadsheet",
		app.uiHdl.SwitchToChromeTabByName("Book"),
		app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(canvas),
		app.selectBox("A1"),
		uiauto.Sleep(dataWaitTime), // Given time to select box.
		app.kb.AccelAction("Ctrl+V"),
		uiauto.Sleep(dataWaitTime), // Given time to paste data.
		app.selectBox("H1"),
		app.kb.TypeAction(sheetText),
		app.kb.AccelAction("Enter"),
	)

	if err = uiauto.Combine("create the example spreadsheet",
		uiauto.NamedAction("copy from existing spreadsheet", copyFromExistingSheet),
		uiauto.NamedAction("paste into newly created spreadsheet", pasteIntoNewSheet),
		uiauto.NamedAction("rename the spreadsheet", app.renameDocument(sheetName)),
	)(ctx); err != nil {
		return "", err
	}

	// Since the file will only be saved automatically after the file is edited, mark the file created successfully here.
	app.sheetCreated = true
	return sheetName, nil
}

// skipUpdatingTermsDialog skips the "We are updating our terms" dialog.
// The dialog might pop up in the following situations:
// 1. After navigating to OneDrive or Microsoft Excel.
// 2. After the sign-in process, before the stay sign-in dialog pops up.
func (app *MicrosoftWebOffice) skipUpdatingTermsDialog() uiauto.Action {
	updatingRootWebArea := nodewith.Name("We're updating our terms").Role(role.RootWebArea).Focusable()
	skipTermsButton := nodewith.Name("Next").Role(role.Button).Ancestor(updatingRootWebArea)
	return uiauto.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(skipTermsButton), app.uiHdl.Click(skipTermsButton))
}

// OpenSpreadsheet opens an existing spreadsheet from microsoft web app.
func (app *MicrosoftWebOffice) OpenSpreadsheet(ctx context.Context, fileName string) (err error) {
	testing.ContextLog(ctx, "Opening an existing spreadsheet: ", fileName)
	conn, err := app.openOneDrive(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open OneDrive")
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	return uiauto.NamedCombine("search the sample spreadsheet",
		app.searchSampleSheet,
		app.maybeCloseOneDriveTab(myFilesTab),
	)(ctx)
}

// MoveDataFromDocToSheet moves data from document to spreadsheet.
func (app *MicrosoftWebOffice) MoveDataFromDocToSheet(ctx context.Context) error {
	wordWebArea := nodewith.Name("Word").Role(role.RootWebArea)
	paragraph := nodewith.Role(role.GenericContainer).Ancestor(wordWebArea).HasClass("EditingSurfaceBody").Focusable()
	if err := uiauto.NamedCombine("switch to Microsoft Word cut selected text from the document",
		app.uiHdl.SwitchToChromeTabByName(wordTab),
		uiauto.IfFailThen(app.ui.DoDefault(paragraph), app.ui.DoDefault(paragraph.Editable())),
		app.kb.AccelAction("Ctrl+A"),
		app.kb.AccelAction("Ctrl+C"),
		uiauto.Sleep(dataWaitTime), // Given time to select all data.
	)(ctx); err != nil {
		return err
	}

	return uiauto.NamedCombine("switch to Microsoft Excel and paste the content into a cell of the spreadsheet",
		app.uiHdl.SwitchToChromeTabByName(excelTab),
		app.ui.WaitUntilExists(canvas),
		app.selectBox("H3"),
		app.kb.AccelAction("Ctrl+V"),
	)(ctx)
}

// MoveDataFromSheetToDoc moves data from spreadsheet to document.
func (app *MicrosoftWebOffice) MoveDataFromSheetToDoc(ctx context.Context) error {
	if err := uiauto.Combine("cut selected text from cell",
		app.selectBox("H1"),
		app.kb.AccelAction("Ctrl+C"),
	)(ctx); err != nil {
		return err
	}

	wordWebArea := nodewith.Name("Word").Role(role.RootWebArea)
	paragraph := nodewith.Role(role.Paragraph).Ancestor(wordWebArea).Editable()
	return uiauto.NamedCombine("switch to Microsoft Word and paste the content",
		app.uiHdl.SwitchToChromeTabByName(wordTab),
		app.ui.WaitUntilExists(paragraph),
		app.kb.AccelAction("Ctrl+V"),
	)(ctx)
}

// ScrollPage scrolls the document and spreadsheet.
func (app *MicrosoftWebOffice) ScrollPage(ctx context.Context) error {
	testing.ContextLog(ctx, "Scrolling the document and spreadsheet")
	for _, tabName := range []string{wordTab, excelTab} {
		if err := scrollTabPageByName(ctx, app.uiHdl, tabName); err != nil {
			return err
		}
	}
	return nil
}

// SwitchToOfflineMode switches to offline mode.
func (app *MicrosoftWebOffice) SwitchToOfflineMode(ctx context.Context) error {
	return nil
}

// UpdateCells updates one of the independent cells and propagate values to dependent cells.
func (app *MicrosoftWebOffice) UpdateCells(ctx context.Context) error {
	if err := app.editBoxValue(ctx, "A3", "100"); err != nil {
		return errors.Wrap(err, "failed to edit the value of the cell")
	}

	val, err := app.getBoxValue(ctx, "B1")
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

// VoiceToTextTesting uses the "Dictation" function to achieve voice-to-text (VTT) and directly input text into office documents.
func (app *MicrosoftWebOffice) VoiceToTextTesting(ctx context.Context, expectedText string, playAudio action.Action) error {

	// allowPermission allows microphone permission if requested by the browser.
	allowPermission := func(ctx context.Context) error {
		alertDialog := nodewith.NameContaining("Use your microphone").ClassName("RootView").Role(role.AlertDialog).First()
		allowButton := nodewith.Name("Allow").Role(role.Button).Ancestor(alertDialog)
		if err := app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(allowButton)(ctx); err != nil {
			testing.ContextLog(ctx, "No action to grant microphone permission")
			return nil
		}
		return app.uiHdl.ClickUntil(allowButton, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilGone(alertDialog))(ctx)
	}

	wordWebArea := nodewith.Name("Word").Role(role.RootWebArea)
	paragraph := nodewith.Role(role.Paragraph).HasClass("Paragraph").Ancestor(wordWebArea).First()

	// checkDictationResult checks if the document contains the expected dictation results.
	checkDictationResult := func(ctx context.Context) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			if err := uiauto.Combine("copy the content of the document to the clipboard",
				app.closeHelpPanel,
				app.uiHdl.Click(paragraph),
				app.kb.AccelAction("Ctrl+A"),
				uiauto.Sleep(dataWaitTime), // Wait for all text to be selected.
				app.kb.AccelAction("Ctrl+C"),
				uiauto.Sleep(dataWaitTime), // Wait for all text to be copied.
			)(ctx); err != nil {
				return err
			}

			clipData, err := getClipboardText(ctx, app.tconn)
			if err != nil {
				return err
			}
			clipData = strings.ReplaceAll(clipData, ",", "")
			clipData = strings.ReplaceAll(clipData, ".", "")
			ignoreCaseData := strings.TrimSuffix(strings.ToLower(clipData), "\n")
			if !strings.Contains(ignoreCaseData, strings.ToLower(expectedText)) {
				return errors.Errorf("failed to validate input value ignoring case: got: %s; want: %s", clipData, expectedText)
			}
			return nil
		}, &testing.PollOptions{Interval: time.Second, Timeout: 15 * time.Second})
	}

	// dictate operates the dictation process.
	dictate := uiauto.Combine("play an audio file and check dictation results",
		uiauto.NamedAction("turn on the dictation", app.turnOnDictation),
		allowPermission,
		uiauto.NamedAction("check if dictation is on", app.checkDictation),
		uiauto.NamedAction("play the audio", playAudio),
		uiauto.NamedAction("check if the result is as expected", checkDictationResult),
	)

	dictationToolbar := nodewith.Name("Dictation toolbar").Role(role.Toolbar)
	stopDictationButton := nodewith.Name("Stop Dictation").Role(role.Button).Ancestor(dictationToolbar).First()
	return uiauto.NamedCombine("turn on the voice typing",
		app.uiHdl.SwitchToChromeTabByName(wordTab), // Switch to Microsoft Word.
		app.ui.WaitUntilExists(wordWebArea),
		app.ui.Retry(retryTimes, dictate),
		uiauto.IfSuccessThen(app.ui.Exists(stopDictationButton), app.uiHdl.Click(stopDictationButton)),
	)(ctx)
}

// Cleanup cleans up the resources used by running the Microsoft Web Office testing.
// It removes the document and slide which we created in the test case and close all tabs after completing the test.
// This function should be called as deferred function after the app is created.
func (app *MicrosoftWebOffice) Cleanup(ctx context.Context, sheetName string) error {
	testing.ContextLog(ctx, "Cleaning up the tabs and files")

	conn, err := app.openOneDrive(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	if err := app.clickNavigationItem(myFiles)(ctx); err != nil {
		return err
	}

	// Because we did not rename the document and the slide, the file names will be named as the default values.
	// documentCreated and slideCreated are variables that indicate whether the file has been successfully created.
	filesRemovedMap := map[string]bool{
		"Document":     app.documentCreated,
		"Presentation": app.slideCreated,
		sheetName:      app.sheetCreated,
	}

	for file, isCreated := range filesRemovedMap {
		if isCreated {
			if err := app.removeDocument(file)(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to removing: ", file)
			}
		}
	}

	return nil
}

// SetBrowser sets browser to chrome or lacros.
func (app *MicrosoftWebOffice) SetBrowser(br *browser.Browser) {
	app.br = br
}

// removeSheet checks the existence of the sheet and remove it if it exists.
func (app *MicrosoftWebOffice) removeSheet(sheetName string) uiauto.Action {
	return func(ctx context.Context) error {
		sheetFileRow := nodewith.NameContaining(sheetName).Role(role.Row)
		nodes, err := app.ui.NodesInfo(ctx, sheetFileRow)
		if err != nil {
			return err
		}
		for _, node := range nodes {
			testing.ContextLogf(ctx, "Removing %s: ", node.Name)
			if err := app.removeDocument(sheetName)(ctx); err != nil {
				return err
			}
		}
		return nil
	}
}

// maybeCloseOneDriveTab closes the specified tab if it exists.
// Sometimes the tab name is just "OneDrive". Therefore, if the specified tab cannot be found, try to search for it.
func (app *MicrosoftWebOffice) maybeCloseOneDriveTab(tabName string) action.Action {
	return func(ctx context.Context) error {
		// Get a TestConn to active browser.
		bTconn, err := app.br.TestAPIConn(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to create test API connection")
		}
		tabs, err := browser.CurrentTabs(ctx, bTconn)
		if err != nil {
			return err
		}
		tabID := 0
		found := false
		for _, tab := range tabs {
			if tab.Title == tabName || tab.Title == oneDriveTab {
				tabID = tab.ID
				found = true
				break
			}
		}
		if !found {
			testing.ContextLogf(ctx, "Cannot find the tab name containing %q or %q", tabName, oneDriveTab)
			return nil
		}
		return browser.CloseTabsByID(ctx, bTconn, []int{tabID})
	}
}

// checkSignIn checks if it is signed in, if not, try to sign in.
func (app *MicrosoftWebOffice) checkSignIn(ctx context.Context) error {
	testing.ContextLog(ctx, "Check if already signed in")

	// If the account manager exists, it means it has been logged in. Skip the login procedure.
	accountManager := nodewith.NameContaining("Account manager for").Role(role.Button)
	if err := app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(accountManager)(ctx); err != nil {
		msWebArea := nodewith.NameContaining("Microsoft Office").Role(role.RootWebArea)
		signInLink := nodewith.NameContaining("Sign in").Role(role.Link).Ancestor(msWebArea).First()
		securityHeading := nodewith.Name("Is your security info still accurate?").Role(role.Heading)
		looksGoodButton := nodewith.Name("Looks good!").Role(role.Button)
		if err := uiauto.NamedCombine("click the sign in link",
			uiauto.IfSuccessThen(
				app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(signInLink),
				app.ui.DoDefault(signInLink),
			),
			uiauto.IfSuccessThen(
				app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(securityHeading),
				app.uiHdl.Click(looksGoodButton),
			),
		)(ctx); err != nil {
			return err
		}

		accountLocked := nodewith.Name("Your account has been locked").Role(role.StaticText)
		// If the message exists, it means the account has been locked. We can only recover it manually.
		if err := app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(accountLocked)(ctx); err != nil {
			return app.signIn(ctx)
		}

		return errors.New("failed to sign in to microsoft office, your account has been locked")
	}

	testing.ContextLog(ctx, "Account has been logged in")
	return nil
}

// signIn signs in to Microsoft Office account.
func (app *MicrosoftWebOffice) signIn(ctx context.Context) error {
	testing.ContextLog(ctx, "Signing in to Microsoft Office")

	accountField := nodewith.NameContaining("Enter your email").Role(role.TextField)
	enterAccount := uiauto.NamedCombine("enter the account",
		app.ui.DoDefaultUntil(accountField, app.ui.Exists(accountField.Focused())),
		app.kb.AccelAction("Ctrl+A"),
		app.kb.TypeAction(app.username),
		app.kb.AccelAction("Enter"),
	)

	passwordField := nodewith.Name("Enter the password for " + app.username).Role(role.TextField)
	enterPassword := uiauto.NamedCombine("enter the password",
		app.ui.DoDefaultUntil(passwordField, app.ui.Exists(passwordField.Focused())),
		app.kb.AccelAction("Ctrl+A"), // Prevent the field from already being populated.
		app.kb.TypeAction(app.password),
		app.kb.AccelAction("Enter"),
	)

	accountList := nodewith.Name("Pick an account").Role(role.List)
	accountButton := nodewith.NameContaining(app.username).Role(role.Button).Ancestor(accountList)
	// If we have logged in before, sometimes it will show a "Pick an account" list.
	if err := uiauto.IfSuccessThen(
		app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(accountButton),
		app.uiHdl.Click(accountButton),
	)(ctx); err != nil {
		return err
	}

	// If we select the account option in the "Pick an account" list, there is no need to fill in the account field.
	if err := uiauto.IfSuccessThen(
		app.ui.WaitUntilExists(accountField),
		enterAccount,
	)(ctx); err != nil {
		return err
	}

	// Check and skip the dialog at the end of sign in action.
	savePasswordWindow := nodewith.Name("Save password?").Role(role.Window)
	closeSavePasswordWindow := nodewith.Name("Close").Role(role.Button).Ancestor(savePasswordWindow)
	msAccountWebArea := nodewith.Name("Microsoft account").Role(role.RootWebArea)
	staySignInHeading := nodewith.Name("Stay signed in?").Role(role.Heading).Ancestor(msAccountWebArea)
	staySignInYesButton := nodewith.Name("Yes").Role(role.Button).Ancestor(msAccountWebArea).Focusable()
	closeButton := nodewith.Name("Close first run experience").Role(role.Button)

	// Sometimes it will login directly without entering password.
	return uiauto.Combine("enter password and skip dialog",
		uiauto.IfSuccessThen(app.ui.WaitUntilExists(passwordField), enterPassword),
		uiauto.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(closeSavePasswordWindow), app.uiHdl.Click(closeSavePasswordWindow)),
		app.skipUpdatingTermsDialog(),
		uiauto.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(staySignInHeading), uiauto.NamedAction("click stay sign in", app.uiHdl.Click(staySignInYesButton))),
		uiauto.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(closeButton), app.uiHdl.Click(closeButton)),
	)(ctx)
}

// reloadPage reloads the website if it down.
// There might be five situations and there is no guarantee that the page will be restored after one click.
// 1. "This page isn’t working" means that the Microsoft website returns an HTTP status code of 500, sometimes with a "Reload" button.
// 2. The heading "Something went wrong" pops up with a "Go to OneDrive" button.
// 3. The image "Something went wrong" pops up with a "Go to my OneDrive" button.
// 4. The heading "This item might not exist or is no longer available" pops up with a "Go to OneDrive" button.
// 5. The link "Microsoft OneDrive" pops up.
func (app *MicrosoftWebOffice) reloadPage(ctx context.Context) error {
	testing.ContextLog(ctx, "Checking if the website needs to be reloaded")

	reloadButton := nodewith.Name("Reload").Role(role.Button).ClassName("blue-button text-button")
	goToOneDrive := nodewith.Name("Go to OneDrive").Role(role.Button).First()
	goToMyOneDrive := nodewith.Name("Go to my OneDrive").Role(role.Button).First()
	oneDriveLink := nodewith.Name("Microsoft OneDrive").Role(role.Link).First()

	// dialogsInfo holds the information of dialogs that will encountered and needs to be handled during testing.
	// The order of slices starts with the most frequent occurrence.
	dialogsInfo := []dialogInfo{
		{
			name: "Reload",
			node: reloadButton,
		},
		{
			name: "Go to OneDrive",
			node: goToOneDrive,
		},
		{
			name: "Go to my OneDrive",
			node: goToMyOneDrive,
		},
		{
			name: "Microsoft OneDrive",
			node: oneDriveLink,
		},
	}

	for _, info := range dialogsInfo {
		name, node := info.name, info.node

		testing.ContextLogf(ctx, "Checking if the %q node exists", name)
		if err := app.ui.WaitUntilExists(node)(ctx); err != nil {
			continue
		}

		return app.ui.Retry(retryTimes, func(ctx context.Context) error {
			if err := app.uiHdl.ClickUntil(node, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilGone(node))(ctx); err != nil {
				return err
			}
			// Sometimes it just disappears for a while and then reappears.
			// Make sure that the node does not appear at all.
			return app.ui.EnsureGoneFor(node, 15*time.Second)(ctx)
		})(ctx)
	}

	return nil
}

// reload reloads the page if the display is different from what we expected.
// If the tab navigates to the "My Files" page after reloading, then we need to re-operate the operation.
// After clicking the "Go to OneDrive" or "Go to My OneDrive" button, it will create another new tab called "My files - OneDrive".
// Therefore, it needs to be closed after re-operation. Otherwise, the number of current tabs will be affected and subsequent operations will fail.
func (app *MicrosoftWebOffice) reload(finder *nodewith.Finder, action action.Action) action.Action {
	return func(ctx context.Context) error {
		oneDriveWebArea := nodewith.Name("My files - OneDrive").Role(role.RootWebArea)
		if err := app.ui.WithTimeout(longerUIWaitTime).WaitUntilExists(finder)(ctx); err != nil {
			return uiauto.Combine("reload and reoperate the action",
				app.reloadPage,
				uiauto.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(oneDriveWebArea), action),
				app.maybeCloseOneDriveTab(myFilesTab),
			)(ctx)
		}
		return nil
	}
}

// openOneDrive navigates to OneDrive web page from Microsoft Office Home.
func (app *MicrosoftWebOffice) openOneDrive(ctx context.Context) (*chrome.Conn, error) {
	conn, err := app.br.NewConn(ctx, cuj.MicrosoftOfficeURL)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open URL: %s", cuj.MicrosoftOfficeURL)
	}
	if err := webutil.WaitForQuiescence(ctx, conn, longerUIWaitTime); err != nil {
		return nil, errors.Wrap(err, "failed to wait for microsoft page to finish loading")
	}
	if err := cuj.MaximizeBrowserWindow(ctx, app.tconn, app.tabletMode, "Chrome"); err != nil {
		return nil, errors.Wrap(err, "failed to maximize the microsoft page")
	}
	appLauncher := nodewith.Name("App launcher").Role(role.PopUpButton).Collapsed()
	appLauncherOpened := nodewith.Name("App launcher opened").Role(role.GenericContainer)
	closeAppLauncher := nodewith.Name("Close the app launcher").Role(role.Button).Ancestor(appLauncherOpened)
	oneDriveLink := nodewith.Name("OneDrive").Role(role.Link).Ancestor(appLauncherOpened)
	goToOffice := nodewith.Name("Go to Office").Role(role.Link)
	securityHeading := nodewith.Name("Is your security info still accurate?").Role(role.Heading)
	looksGoodButton := nodewith.Name("Looks good!").Role(role.Button)
	navigateToOneDrive := uiauto.Retry(retryTimes, func(ctx context.Context) (err error) {
		if err = uiauto.NamedCombine("navigate to OneDrive",
			uiauto.IfSuccessThen(app.ui.Exists(securityHeading), app.uiHdl.Click(looksGoodButton)),
			app.ui.DoDefault(appLauncher),
			app.ui.WaitUntilExists(closeAppLauncher),
			app.ui.FocusAndWait(oneDriveLink),
			app.ui.DoDefault(oneDriveLink),
			app.skipUpdatingTermsDialog(),
		)(ctx); err == nil {
			return nil
		}
		return uiauto.Combine(`click the "Office" link`,
			app.uiHdl.Click(goToOffice),
			app.ui.WithTimeout(defaultUIWaitTime).WaitUntilGone(goToOffice),
		)(ctx)
	})

	myFiles := nodewith.Name("My files").Role(role.Heading).First()
	alertDialog := nodewith.Role(role.AlertDialog).First()
	closeDialog := nodewith.Name("Close dialog").Role(role.Button).Ancestor(alertDialog)
	gotItButton := nodewith.Name("Got it").Role(role.Button)

	if err := uiauto.Combine("check if already signed in and navigate to OneDrive",
		app.checkSignIn,
		navigateToOneDrive,
		app.reload(myFiles, func(ctx context.Context) error { return nil }),
		uiauto.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(closeDialog), app.uiHdl.Click(closeDialog)),
		uiauto.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(gotItButton), app.uiHdl.Click(gotItButton)),
	)(ctx); err != nil {
		return nil, err
	}

	return conn, nil
}

// openNewFile opens a new document for the specified service.
func (app *MicrosoftWebOffice) openNewFile(service string) action.Action {
	oneDriveWebArea := nodewith.Name("My files - OneDrive").Role(role.RootWebArea)
	newItem := nodewith.NameStartingWith("New").Role(role.MenuItem).Ancestor(oneDriveWebArea)
	newItemMenu := nodewith.Role(role.Menu).Ancestor(newItem)
	serviceItem := nodewith.NameContaining(service).Role(role.MenuItem).Ancestor(oneDriveWebArea)
	return uiauto.NamedCombine("open a new "+service,
		// Make sure "New" exists before creating a new file. This is especially necessary on low-end DUTs.
		app.ui.WithTimeout(longerUIWaitTime).WaitUntilExists(newItem),
		app.uiHdl.ClickUntil(newItem, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(newItemMenu)),
		app.uiHdl.ClickUntil(serviceItem, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilGone(oneDriveWebArea)),
	)
}

// openBlankDocument opens a blank document with specified service.
// When we try to open a blank document on the corresponding service page, it will jump to the Microsoft Office App to request permission.
// Therefore, we try to open a blank document from OneDrive to avoid this situation.
func (app *MicrosoftWebOffice) openBlankDocument(service string) action.Action {
	return func(ctx context.Context) error {
		noThanksButton := nodewith.Name("No thanks").Role(role.Button).Focusable()
		closeDialogButton := nodewith.Name("Close dialog").Role(role.Button).Focusable()
		if err := uiauto.Combine("close dialogs",
			// Skip an alert dialog "Get the most out of your OneDrive" when it pops up.
			uiauto.IfSuccessThen(
				app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(noThanksButton),
				app.uiHdl.Click(noThanksButton),
			),
			// Skip "Let's get you started" when the dialog pops up.
			uiauto.IfSuccessThen(
				app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(closeDialogButton),
				app.uiHdl.Click(closeDialogButton),
			),
		)(ctx); err != nil {
			return err
		}

		if err := app.openNewFile(service)(ctx); err != nil {
			return err
		}

		paragraph := nodewith.Role(role.Paragraph).Editable()
		title := nodewith.Name("Click to add title").First()

		// element defines the node to specify whether it navigates to the corresponding service page correctly.
		element := map[string]*nodewith.Finder{
			word:       paragraph,
			powerpoint: title,
			excel:      canvas,
		}

		return app.reload(element[service], app.openNewFile(service))(ctx)
	}
}

// clickNavigationItem clicks the specified item in the navigation bar.
func (app *MicrosoftWebOffice) clickNavigationItem(itemName string) action.Action {
	return func(ctx context.Context) error {
		window := nodewith.NameContaining(myFilesTab).Role(role.Window).First()
		maximizeButton := nodewith.Name("Maximize").Role(role.Button).HasClass("FrameCaptionButton").Ancestor(window)
		// Maximize the browser window so that the navigation bar appears on the screen.
		if err := uiauto.IfSuccessThen(app.ui.Exists(maximizeButton), app.uiHdl.Click(maximizeButton))(ctx); err != nil {
			return err
		}

		navigation := nodewith.Role(role.Navigation)
		navigationList := nodewith.Role(role.List).Ancestor(navigation)
		itemLink := nodewith.NameContaining(itemName).Role(role.Link).Ancestor(navigationList)

		if err := app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(itemLink)(ctx); err == nil {
			testing.ContextLogf(ctx, "Directly enter to %q from the navigation list", itemName)
			return app.ui.DoDefault(itemLink)(ctx)
		}

		menu := nodewith.Role(role.Menu).Ancestor(navigation)
		menuItem := nodewith.NameContaining(itemName).Role(role.MenuItem).Ancestor(menu).Visited()

		if err := app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(menuItem)(ctx); err == nil {
			testing.ContextLogf(ctx, "Directly enter to %q from the menu", itemName)
			return app.uiHdl.Click(menuItem)(ctx)
		}

		commandBar := nodewith.NameStartingWith("Command bar").Role(role.MenuBar).Horizontal()
		appMenu := nodewith.NameContaining("App menu").Role(role.MenuItem).Ancestor(commandBar)
		navigationMenu := nodewith.Role(role.Menu).Ancestor(navigation)
		itemLink = nodewith.NameContaining(itemName).Role(role.MenuItem).Ancestor(navigationMenu).Visited()

		return uiauto.NamedCombine(fmt.Sprintf("click on the %q item in the navigation bar", itemName),
			app.ui.DoDefaultUntil(appMenu, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(navigationMenu)),
			app.uiHdl.Click(itemLink),
		)(ctx)
	}
}

// switchToListView switches the view option to list view.
func (app *MicrosoftWebOffice) switchToListView(ctx context.Context) error {
	details := nodewith.NameRegex(regexp.MustCompile(`([Dd]etails|Info).*`)).Role(role.MenuItem).First()
	viewOptions := nodewith.NameRegex(regexp.MustCompile(`[Vv]iew options`)).Role(role.MenuItem)
	viewOptionsExpanded := viewOptions.Expanded()
	listView := nodewith.NameContaining("List").Role(role.MenuItemCheckBox).Ancestor(viewOptions)

	checkListViewEnabled := func(ctx context.Context) error {
		if node, err := app.ui.Info(ctx, listView); err != nil {
			return err
		} else if node.Checked == checked.False {
			return errors.New("list view options has not been enabled")
		}
		return nil
	}

	// If the setting is already "list view", skip the switch view operation.
	if err := checkListViewEnabled(ctx); err != nil {
		return app.ui.Retry(retryTimes, uiauto.Combine("switch to list view",
			// Sometimes "Details" will be displayed later, causing the position of the "View Options" button to change.
			app.ui.WaitUntilExists(details),
			app.ui.RetryUntil(app.ui.DoDefault(viewOptions), app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(viewOptionsExpanded)),
			// After the page loads, OneDrive will reload and display again.
			// Therefore, the expanded list of "View Options" might disappear and cause the "List" option to not be found.
			app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(listView),
			app.uiHdl.Click(listView),
		))(ctx)
	}

	// Give the list some time to finish loading and ordering. Otherwise, we may encounter errors in the next operation.
	return testing.Sleep(ctx, 2*time.Second)
}

// sortByModified sorts by date modified in descending order.
func (app *MicrosoftWebOffice) sortByModified(ctx context.Context) error {
	sort := nodewith.NameContaining("Sort").Role(role.MenuItem).First()
	sortExpanded := sort.Expanded()
	modified := nodewith.Name("Modified").Role(role.MenuItemCheckBox)
	descending := nodewith.Name("Descending").Role(role.MenuItemCheckBox)
	sortMenuExpand := app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(sortExpanded)

	setCheckBoxChecked := func(nodeFinder *nodewith.Finder) uiauto.Action {
		return func(ctx context.Context) error {
			if err := uiauto.IfFailThen(app.ui.Exists(sortExpanded), app.uiHdl.Click(sort))(ctx); err != nil {
				return err
			}
			node, err := app.ui.Info(ctx, nodeFinder)
			if err != nil {
				return err
			}
			if node.Checked == checked.True {
				return nil
			}
			return app.uiHdl.Click(nodeFinder)(ctx)
		}
	}

	return uiauto.Combine("sort by date modified in descending order",
		setCheckBoxChecked(modified),
		setCheckBoxChecked(descending),
		uiauto.IfFailThen(sortMenuExpand, app.uiHdl.Click(sort)),
		// Give the list some time to finish loading and ordering. Otherwise, we may encounter errors in the next operation.
		uiauto.Sleep(2*time.Second),
	)(ctx)
}

// searchSampleSheet searches for the existence of the sample spreadsheet.
func (app *MicrosoftWebOffice) searchSampleSheet(ctx context.Context) error {

	// Check if the sample file exists via searching box.
	searchFromBox := func(ctx context.Context) error {
		goToOneDrive := nodewith.Name("Go to OneDrive").Role(role.Button).First()
		searchBox := nodewith.Name("Search box. Suggestions appear as you type.").Role(role.TextFieldWithComboBox)
		suggestedFiles := nodewith.Name("Suggested files").Role(role.Group)
		fileOption := fmt.Sprintf("Excel file result: .xlsx %v.xlsx ,", sheetName)
		fileResult := nodewith.Name(fileOption).Role(role.ListBoxOption).Ancestor(suggestedFiles).First()
		return uiauto.NamedCombine("search through the box",
			uiauto.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(goToOneDrive), app.uiHdl.Click(goToOneDrive)),
			app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(searchBox),
			app.uiHdl.Click(searchBox),
			app.kb.TypeAction(sheetName),
			app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(fileResult),
			app.uiHdl.ClickUntil(fileResult, app.ui.Gone(fileResult)),
			app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(canvas),
		)(ctx)
	}

	// Check if the sample file in the list of "My files".
	searchFromMyFiles := func(ctx context.Context) error {
		row := nodewith.NameContaining(sheetName).Role(role.Row).First()
		link := nodewith.NameContaining(sheetName).Role(role.Link).Ancestor(row)
		return uiauto.NamedCombine("search file from my files",
			app.clickNavigationItem(myFiles),
			app.switchToListView,
			app.sortByModified,
			app.ui.DoDefaultUntil(link, app.ui.Gone(link)),
			app.ui.WithTimeout(longerUIWaitTime).WaitUntilExists(canvas), // The excel page may takes long time to load.
			app.maybeCloseOneDriveTab(myFilesTab),
		)(ctx)
	}

	// If the spreadsheet doesn't appear in "My files", try searching from the search box.
	if err := searchFromMyFiles(ctx); err != nil {
		return searchFromBox(ctx)
	}

	return nil
}

// openFindAndSelect opens "Find & Select".
func (app *MicrosoftWebOffice) openFindAndSelect(ctx context.Context) error {
	findAndSelectButton := nodewith.Name("Find & Select").Role(role.PopUpButton)
	moreOptionsButton := nodewith.NameContaining("More Options").Role(role.PopUpButton).Ancestor(homeTabPanel)
	moreOptionsMenu := nodewith.NameContaining("More Options").Role(role.Menu).Ancestor(homeTabPanel)
	findAndSelectItem := nodewith.Name("Find & Select").Role(role.MenuItem).Ancestor(moreOptionsMenu)

	// There might be two situations.
	// 1. TabPanel shows "Find & Select" directly.
	// 2. First click on "More Options" or "Editing" then you can find "Find & Select".
	found, err := app.ui.IsNodeFound(ctx, findAndSelectButton)
	if err != nil {
		return err
	}
	if found {
		return uiauto.NamedAction("click Find & Select", app.uiHdl.Click(findAndSelectButton))(ctx)
	}

	return uiauto.NamedCombine("click more options",
		app.uiHdl.Click(moreOptionsButton),
		app.uiHdl.Click(findAndSelectItem),
	)(ctx)
}

// selectRangeWithNameBox selects the range by clicking on the "Name Box".
func (app *MicrosoftWebOffice) selectRangeWithNameBox(ctx context.Context) error {
	testing.ContextLog(ctx, `Selecting range by focus on "Name Box"`)

	// In the clamshell mode, the "Name Box" can be focused with just click.
	nameBox := nodewith.NameContaining("Name Box").Role(role.TextFieldWithComboBox).Editable()
	return app.ui.DoDefaultUntil(nameBox, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(nameBox.Focused()))(ctx)
}

// selectRangeWithGoTo selects the range by opening "Go to" box since the tapping response is different with clicking.
func (app *MicrosoftWebOffice) selectRangeWithGoTo(ctx context.Context) error {
	rangeText := nodewith.Name("Range:").Role(role.TextField).Editable()
	rangeTextFocused := rangeText.Focused()
	// Pressing Ctrl+G will open the "Go To" box.
	// Sometimes key events are typed but the UI does not respond. Retry to alert dialog does appear.
	if err := app.ui.WithInterval(time.Second).RetryUntil(app.kb.AccelAction("Ctrl+G"),
		app.ui.WithTimeout(3*time.Second).WaitUntilExists(rangeText))(ctx); err != nil {
		testing.ContextLog(ctx, "Opening with panel due to ", err.Error())

		home := nodewith.Name("Home").Role(role.Tab)
		goToMenuItem := nodewith.Name("Go to").Role(role.MenuItem)
		goToDialog := nodewith.Name("Go to").Role(role.Dialog)
		okButton := nodewith.Name("OK").Role(role.Button).Ancestor(goToDialog)
		if err := uiauto.NamedCombine(`open "Go To" with panel`,
			app.uiHdl.ClickUntil(home, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(homeTabPanel)),
			app.openFindAndSelect,
			uiauto.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(goToMenuItem), app.uiHdl.Click(goToMenuItem)),
			uiauto.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(goToDialog), app.uiHdl.Click(okButton)),
		)(ctx); err != nil {
			return err
		}
	}

	return app.uiHdl.ClickUntil(rangeText, app.ui.Exists(rangeTextFocused))(ctx)
}

// selectBox selects the specified cell using the name box.
func (app *MicrosoftWebOffice) selectBox(box string) action.Action {
	return uiauto.NamedCombine(fmt.Sprintf("select box %q", box),
		app.selectRangeWithNameBox,
		app.kb.AccelAction("Ctrl+A"), // Make sure to clear the content and re-input.
		app.kb.TypeAction(box),
		app.kb.AccelAction("Enter"),
	)
}

// getBoxValue gets the value of the specified box.
func (app *MicrosoftWebOffice) getBoxValue(ctx context.Context, box string) (clipData string, err error) {
	if err := app.selectBox(box)(ctx); err != nil {
		return "", err
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Due to the unstable network, there might be no data in the clipboard after the copy operation.
		// Therefore, we also need to retry the copy operation.
		if err := app.kb.AccelAction("Ctrl+C")(ctx); err != nil {
			return err
		}
		// Given time to copy data.
		testing.Sleep(ctx, time.Second)
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

	testing.ContextLogf(ctx, "Getting box %q value: %s", box, clipData)

	return clipData, nil
}

// editBoxValue edits the cell to the specified value.
func (app *MicrosoftWebOffice) editBoxValue(ctx context.Context, box, value string) error {
	return uiauto.NamedCombine(fmt.Sprintf("write box %q value", box),
		app.selectBox(box),
		app.kb.TypeAction(value),
		app.kb.AccelAction("Enter"),
	)(ctx)
}

// checkFormula checks if the formula is correct.
func (app *MicrosoftWebOffice) checkFormula(ctx context.Context, box, value string) error {
	if err := app.selectBox(box)(ctx); err != nil {
		return err
	}

	formulaBar := nodewith.Name("formula bar").Role(role.TextField).Editable()
	formulaBarText := nodewith.Role(role.StaticText).FinalAncestor(formulaBar)
	node, err := app.ui.Info(ctx, formulaBarText)
	if err != nil {
		return err
	}
	if node.Name != value {
		return errors.Errorf("failed to verify the formula name, got %s, want %s", node.Name, value)
	}

	return nil
}

// closeHelpPanel closes the "Help" panel if it exists.
func (app *MicrosoftWebOffice) closeHelpPanel(ctx context.Context) error {
	helpPanel := nodewith.Name("Help").Role(role.TabPanel)
	close := nodewith.Name("Close").Role(role.Button).Ancestor(helpPanel)
	return uiauto.IfSuccessThen(
		app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(helpPanel),
		app.uiHdl.Click(close),
	)(ctx)
}

// checkDictation checks whether the dictation is turned on by checking the "Start Dictation" button exists.
func (app *MicrosoftWebOffice) checkDictation(ctx context.Context) error {
	dictationToolbar := nodewith.Name("Dictation toolbar").Role(role.Toolbar)
	startDictationButton := nodewith.Name("Start Dictation").Role(role.Button).Ancestor(dictationToolbar)

	if err := app.ui.WaitUntilExists(dictationToolbar)(ctx); err != nil {
		return err
	}

	return uiauto.IfSuccessThen(
		app.ui.WaitUntilExists(startDictationButton),
		app.uiHdl.Click(startDictationButton),
	)(ctx)
}

// turnOnDictationFromMoreOptions turns on the dictation function through "More Options".
func (app *MicrosoftWebOffice) turnOnDictationFromMoreOptions(ctx context.Context) error {
	moreOptions := nodewith.Name("More Options").Role(role.PopUpButton).Ancestor(homeTabPanel).First()
	dictationButton := nodewith.Name("Dictate").Role(role.Button).Ancestor(moreOptions).First()
	dictationCheckBox := nodewith.Name("Dictate").Role(role.MenuItemCheckBox).First()
	dictationToolbar := nodewith.Name("Dictation toolbar").Role(role.Toolbar)
	return uiauto.NamedCombine(`turn on the dictation through "More Options"`,
		app.uiHdl.ClickUntil(moreOptions, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(dictationButton)),
		app.uiHdl.ClickUntil(dictationButton, app.ui.Gone(dictationButton)),
		uiauto.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(dictationCheckBox), app.uiHdl.Click(dictationCheckBox)),
		app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(dictationToolbar),
	)(ctx)
}

// turnOnDictationFromPanel turns on the dictation function via the button in the "Home" panel.
func (app *MicrosoftWebOffice) turnOnDictationFromPanel(ctx context.Context) error {
	dictationToggleButton := nodewith.Name("Dictate").Role(role.ToggleButton).Ancestor(homeTabPanel).First()
	dictationCheckBox := nodewith.Name("Dictate").Role(role.MenuItemCheckBox).First()
	dictationToolbar := nodewith.Name("Dictation toolbar").Role(role.Toolbar)
	return uiauto.NamedCombine("turn on the dictation through the panel",
		app.uiHdl.ClickUntil(dictationToggleButton, uiauto.Combine("check if the dictation works",
			uiauto.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(dictationCheckBox), app.uiHdl.Click(dictationCheckBox)),
			app.ui.WaitUntilExists(dictationToolbar)),
		),
	)(ctx)
}

// turnOnDictation turns on the dictation function.
func (app *MicrosoftWebOffice) turnOnDictation(ctx context.Context) error {
	testing.ContextLog(ctx, "Turning on dictation")

	openDocument := func(ctx context.Context) error {
		// Since we did not specify the name of the "Word" document, we need to extract the document name from the tab name.
		docTab := nodewith.NameRegex(regexp.MustCompile(".*.docx - Microsoft Word Online")).Role(role.Tab).First()
		node, err := app.ui.Info(ctx, docTab)
		if err != nil {
			return err
		}
		docsName := strings.Replace(node.Name, " - Microsoft Word Online", "", -1)
		testing.ContextLog(ctx, "Getting the document name from the tab: ", docsName)

		row := nodewith.NameContaining(docsName).Role(role.Row).First()
		link := nodewith.NameContaining(docsName).Role(role.Link).Ancestor(row)
		return uiauto.Combine("reopen the document",
			app.clickNavigationItem(myFiles),
			app.uiHdl.Click(link),
		)(ctx)
	}

	reoperate := func(ctx context.Context, turnOnAction action.Action) error {
		testing.ContextLog(ctx, "Reloading the page and reoperating the function to turn on the dictation function")

		reload := nodewith.Name("Reload").ClassName("ReloadButton")
		paragraph := nodewith.Role(role.Paragraph).Editable()
		if err := turnOnAction(ctx); err != nil {
			if err := uiauto.Combine("reload the page",
				app.uiHdl.Click(reload),
				// After reloading the webpage, it might encounter "This page isn’t working".
				app.reload(paragraph, openDocument),
				app.ui.WithTimeout(longerUIWaitTime).WaitUntilGone(homeTabPanel),
			)(ctx); err != nil {
				return err
			}
		}

		dictationToolbar := nodewith.Name("Dictation toolbar").Role(role.Toolbar)
		// Sometimes the "Dictation toolbar" will be displayed directly, so we don't need to click the "Dictation" button again.
		if err := app.ui.WaitUntilExists(dictationToolbar)(ctx); err != nil {
			if strings.Contains(err.Error(), nodewith.ErrNotFound) {
				return turnOnAction(ctx)
			}
			return err
		}

		return nil
	}

	dictateButton := nodewith.Name("Dictate").Role(role.ToggleButton).Ancestor(homeTabPanel).First()
	// If the "Dictation" button is not displayed on the screen, we need to find it through "More Options".
	// Otherwise, we can click it directly on the panel.
	if err := app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(dictateButton)(ctx); err != nil {
		if strings.Contains(err.Error(), nodewith.ErrNotFound) {
			if err := reoperate(ctx, app.turnOnDictationFromMoreOptions); err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		if err := reoperate(ctx, app.turnOnDictationFromPanel); err != nil {
			return err
		}
	}

	featureBrokenContainer := nodewith.Name("We couldn't connect to the catalog server for this feature.").Role(role.GenericContainer)
	retryButton := nodewith.Name("RETRY").Role(role.Button).Ancestor(featureBrokenContainer)
	return uiauto.Combine("turn on the dictation",
		uiauto.IfSuccessThen(app.ui.Exists(featureBrokenContainer), app.uiHdl.Click(retryButton)),
		app.checkDictation,
	)(ctx)
}

// closeTab closes the tab with the title of the specified name.
func (app *MicrosoftWebOffice) closeTab(title string) action.Action {
	return func(ctx context.Context) error {
		matcher := func(t *target.Info) bool {
			return strings.Contains(t.Title, title) && t.Type == "page"
		}

		conn, err := app.br.NewConnForTarget(ctx, matcher)
		if err != nil {
			return err
		}
		conn.CloseTarget(ctx)
		conn.Close()

		return nil
	}
}

// renameDocument renames the document with the specified file name.
func (app *MicrosoftWebOffice) renameDocument(fileName string) uiauto.Action {
	return func(ctx context.Context) error {
		renameButton := nodewith.NameContaining("Saved to OneDrive").Role(role.Button)
		fileNameTextField := nodewith.NameContaining("File Name").Role(role.TextField)
		checkFileName := func(ctx context.Context) error {
			if err := app.uiHdl.Click(renameButton)(ctx); err != nil {
				return err
			}
			node, err := app.ui.Info(ctx, fileNameTextField)
			if err != nil {
				return err
			}
			if node.Value != fileName {
				return errors.Errorf("file name is incorrect: got: %s; want: %s", node.Value, fileName)
			}
			return nil
		}
		return app.ui.Retry(retryTimes, uiauto.Combine("rename the document: "+fileName,
			uiauto.IfSuccessThen(app.ui.Gone(fileNameTextField),
				app.uiHdl.ClickUntil(renameButton, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(fileNameTextField))),
			app.uiHdl.ClickUntil(fileNameTextField, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(fileNameTextField.Focused())),
			app.kb.AccelAction("Ctrl+A"),
			uiauto.Sleep(dataWaitTime), // Given time to select all data.
			app.kb.TypeAction(sheetName),
			app.kb.AccelAction("Enter"),
			checkFileName,
		))(ctx)
	}
}

// removeDocument removes the document with the specified file name.
func (app *MicrosoftWebOffice) removeDocument(fileName string) uiauto.Action {
	row := nodewith.NameContaining(fileName).Role(role.Row).First()
	checkBox := nodewith.Role(role.CheckBox).Ancestor(row)
	complementary := nodewith.Role(role.Complementary).First()
	commandBar := nodewith.NameContaining("Command bar").Role(role.MenuBar).Ancestor(complementary)
	deleteItem := nodewith.Name("Delete").Role(role.MenuItem).Ancestor(commandBar)
	deleteDialog := nodewith.Name("Delete?").Role(role.Dialog)
	deleteButton := nodewith.Name("Delete").Role(role.Button).Ancestor(deleteDialog)
	myFilesRootWebArea := nodewith.Name(myFilesTab).Role(role.RootWebArea)
	deleteAlert := nodewith.Role(role.Alert).Ancestor(myFilesRootWebArea)
	deleteAlertButton := nodewith.Name("Delete").Role(role.Button).Ancestor(deleteAlert)
	return uiauto.NamedCombine("remove the document: "+fileName,
		app.switchToListView,
		app.ui.DoDefaultUntil(checkBox, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(commandBar)),
		app.uiHdl.Click(deleteItem),
		uiauto.IfSuccessThen(
			app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(deleteDialog),
			app.uiHdl.Click(deleteButton),
		),
		// If the file is still open, the website will confirm the deletion again.
		uiauto.IfSuccessThen(
			app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(deleteAlertButton),
			app.uiHdl.Click(deleteAlertButton),
		),
	)
}

// NewMicrosoftWebOffice creates MicrosoftWebOffice instance which implements ProductivityApp interface.
func NewMicrosoftWebOffice(tconn *chrome.TestConn, uiHdl cuj.UIActionHandler, kb *input.KeyboardEventWriter, tabletMode, isLacros bool, username, password string) *MicrosoftWebOffice {
	return &MicrosoftWebOffice{
		tconn:      tconn,
		ui:         uiauto.New(tconn),
		uiHdl:      uiHdl,
		kb:         kb,
		tabletMode: tabletMode,
		isLacros:   isLacros,
		username:   username,
		password:   password,
	}
}

var _ ProductivityApp = (*MicrosoftWebOffice)(nil)
