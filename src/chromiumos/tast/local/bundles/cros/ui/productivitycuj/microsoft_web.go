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

	"github.com/mafredri/cdp/protocol/target"

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
	// officeURL indicates the link URL of "Microsoft Office Home".
	officeURL = "https://www.office.com/"

	// myFiles indicates the "My files" item name in the navigation bar.
	myFiles = "My files"
	// recent indicates the "Recent" item label in the navigation bar.
	recent = "Recent"

	// word indicates the label of the new document.
	word = "Word document"
	// powerpoint indicates the label of the new presentation.
	powerpoint = "PowerPoint presentation"
	// excel indicates the label of the new spreadsheet.
	excel = "Excel workbook"
)

// MicrosoftWebOffice implements the ProductivityApp interface.
type MicrosoftWebOffice struct {
	cr         *chrome.Chrome
	tconn      *chrome.TestConn
	ui         *uiauto.Context
	kb         *input.KeyboardEventWriter
	uiHdl      cuj.UIActionHandler
	tabletMode bool
	username   string
	password   string
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
	return uiauto.Combine("open a new document",
		app.openBlankDocument(word),
		app.reloadWordDocument,
		// Make sure paragraph exists before typing. This is especially necessary on low-end DUTs.
		app.ui.WithTimeout(longerUIWaitTime).WaitUntilExists(paragraph),
		app.kb.TypeAction(docText),
	)(ctx)
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
	return uiauto.Combine("create a new presentation",
		app.openBlankDocument(powerpoint),
		// Make sure title exists before typing. This is especially necessary on low-end DUTs.
		app.ui.WithTimeout(longerUIWaitTime).WaitUntilExists(title),
		app.uiHdl.Click(title),
		app.kb.TypeAction(titleText),
		app.uiHdl.Click(subtitle),
		app.kb.TypeAction(subtitleText),
		app.kb.AccelAction("Enter"),
	)(ctx)
}

// CreateSpreadsheet creates a new spreadsheet from microsoft web app and returns sheet name.
func (app *MicrosoftWebOffice) CreateSpreadsheet(ctx context.Context) (string, error) {
	return "", nil
}

// OpenSpreadsheet opens an existing spreadsheet from microsoft web app.
func (app *MicrosoftWebOffice) OpenSpreadsheet(ctx context.Context, fileName string) error {
	testing.ContextLog(ctx, "Opening an existing spreadsheet: ", fileName)

	conn, err := app.openOneDrive(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	return app.searchSampleSheet(ctx)
}

// MoveDataFromDocToSheet moves data from document to spreadsheet.
func (app *MicrosoftWebOffice) MoveDataFromDocToSheet(ctx context.Context) error {
	return nil
}

// MoveDataFromSheetToDoc moves data from spreadsheet to document.
func (app *MicrosoftWebOffice) MoveDataFromSheetToDoc(ctx context.Context) error {
	return nil
}

// ScrollPage scrolls the document and spreadsheet.
func (app *MicrosoftWebOffice) ScrollPage(ctx context.Context) error {
	return nil
}

// SwitchToOfflineMode switches to offline mode.
func (app *MicrosoftWebOffice) SwitchToOfflineMode(ctx context.Context) error {
	return nil
}

// UpdateCells updates one of the independent cells and propagate values to dependent cells.
func (app *MicrosoftWebOffice) UpdateCells(ctx context.Context) error {
	return nil
}

// VoiceToTextTesting uses the "Dictation" function to achieve voice-to-text (VTT) and directly input text into office documents.
func (app *MicrosoftWebOffice) VoiceToTextTesting(ctx context.Context, expectedText string, playAudio action.Action) error {
	return nil
}

// End cleans up case. Removes the document and slide which we created in the test case and close all tabs.
func (app *MicrosoftWebOffice) End(ctx context.Context) error {
	return nil
}

// checkSignIn checks if it is logged in, if not, try to log in.
func (app *MicrosoftWebOffice) checkSignIn(ctx context.Context) error {
	testing.ContextLog(ctx, "Signing in to microsoft account")

	// If the account manager exists, it means it has been logged in. Skip the login procedure.
	accountManager := nodewith.NameContaining("Account manager for").Role(role.Button)
	if err := app.ui.WithTimeout(longerUIWaitTime).WaitUntilExists(accountManager)(ctx); err != nil {
		testing.ContextLog(ctx, "Clicking the sign in link")

		msWebArea := nodewith.NameContaining("Microsoft Office").Role(role.RootWebArea)
		signInLink := nodewith.NameContaining("Sign in").Role(role.Link).Ancestor(msWebArea).First()
		if err := app.ui.IfSuccessThen(
			app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(signInLink),
			app.uiHdl.Click(signInLink),
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

// fillInAccount enters the account information in the account text box if it hasn't been filled in yet.
func (app *MicrosoftWebOffice) fillInAccount(expected string) action.Action {
	return func(ctx context.Context) error {
		accountField := nodewith.NameContaining("Enter your email").Role(role.TextField)
		node, err := app.ui.Info(ctx, accountField)
		if err != nil {
			return err
		}
		if node.Value == "" {
			return app.kb.TypeAction(app.username)(ctx)
		}
		if node.Value != expected {
			// If it has filled in the wrong account information, select all first and then enter the information.
			testing.ContextLog(ctx, "Incorrect account information")
			return uiauto.Combine("select all and enter account",
				app.kb.AccelAction("Ctrl+A"),
				app.kb.TypeAction(app.username),
			)(ctx)
		}
		// If it has filled in the correct account information, skip entering it again.
		testing.ContextLog(ctx, "Account information is correct")
		return nil
	}
}

// signIn signs in to Microsoft Office account.
func (app *MicrosoftWebOffice) signIn(ctx context.Context) error {
	testing.ContextLog(ctx, "Signing in to Microsoft Office")

	nextButton := nodewith.Name("Next").Role(role.Button)

	enterAccount := func(ctx context.Context) error {
		testing.ContextLog(ctx, "Entering the account")
		return uiauto.Combine("enter the account",
			app.fillInAccount(app.username),
			app.uiHdl.Click(nextButton),
		)(ctx)
	}

	passwordField := nodewith.Name("Enter the password for " + app.username).Role(role.TextField)
	signInButton := nodewith.Name("Sign in").Role(role.Button)
	staySignInHeading := nodewith.Name("Stay signed in?").Role(role.Heading)
	yesButton := nodewith.Name("Yes").Role(role.Button)
	closeButton := nodewith.Name("Close first run experience").Role(role.Button)

	enterPassword := func(ctx context.Context) error {
		testing.ContextLog(ctx, "Entering the password")
		return uiauto.Combine("enter the password",
			app.ui.DoubleClick(passwordField),
			app.kb.TypeAction(app.password),
			app.uiHdl.Click(signInButton),
			app.ui.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(staySignInHeading), app.uiHdl.Click(yesButton)),
			app.ui.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(closeButton), app.uiHdl.Click(closeButton)),
		)(ctx)
	}

	accountList := nodewith.Name("Pick an account").Role(role.List)
	accountButton := nodewith.NameContaining(app.username).Role(role.Button).Ancestor(accountList)
	// If we have logged in before, sometimes it will show a "Pick an account" list.
	if err := app.ui.IfSuccessThen(
		app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(accountButton),
		app.uiHdl.Click(accountButton),
	)(ctx); err != nil {
		return err
	}

	accountField := nodewith.NameContaining("Enter your email").Role(role.TextField)
	// If we select the account option in the "Pick an account" list, there is no need to fill in the account field.
	if err := app.ui.IfSuccessThen(
		app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(accountField),
		enterAccount,
	)(ctx); err != nil {
		return err
	}

	// Sometimes it will login directly without entering password.
	return app.ui.IfSuccessThen(
		app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(passwordField),
		enterPassword,
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
		testing.ContextLogf(ctx, "Checking if the %q node exists", info.name)

		node := info.node
		if err := app.ui.WaitUntilExists(node)(ctx); err != nil {
			continue
		}

		return app.ui.Retry(retryTimes, func(ctx context.Context) error {
			if err := app.uiHdl.ClickUntil(node, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilGone(node))(ctx); err != nil {
				return err
			}
			// Sometimes it just disappears for a while and then reappears.
			if err := app.ui.WaitUntilExists(node)(ctx); err != nil {
				return nil
			}
			return errors.New("the website needs to be reloaded")
		})(ctx)
	}

	return nil
}

// reload reloads the page if the display is different from what we expected.
// If the tab navigates to the "My Files" page after reloading, then we need to re-operate the operation.
// After clicking the "Go to OneDrive" or "Go to My OneDrive" button, it will create another new tab called "My files - OneDrive".
// Therefore, it needs to be closed after re-operation. Otherwise, the number of current tabs will be affected and subsequent operations will fail.
func (app *MicrosoftWebOffice) reload(finder *nodewith.Finder, action func(ctx context.Context) error) action.Action {
	return func(ctx context.Context) error {
		oneDriveWebArea := nodewith.Name("My files - OneDrive").Role(role.RootWebArea)
		oneDriveTab := nodewith.Name("My files - OneDrive").Role(role.Tab).ClassName("Tab").First()
		closeTab := nodewith.Name("Close").Role(role.Button).ClassName("TabCloseButton").Ancestor(oneDriveTab)
		if err := app.ui.WaitUntilExists(finder)(ctx); err != nil {
			return uiauto.Combine("reload and reoperate the action",
				app.reloadPage,
				app.ui.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(oneDriveWebArea), action),
				app.ui.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(closeTab), app.uiHdl.Click(closeTab)),
			)(ctx)
		}
		return nil
	}
}

// openOneDrive navigates to OneDrive web page from Microsoft Office Home.
func (app *MicrosoftWebOffice) openOneDrive(ctx context.Context) (*chrome.Conn, error) {
	testing.ContextLog(ctx, "Navigating to OneDrive")

	conn, err := app.cr.NewConn(ctx, officeURL)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open URL: %s", officeURL)
	}

	msWebArea := nodewith.Name("Microsoft Office Home").Role(role.RootWebArea)
	appLauncher := nodewith.Name("App launcher").Ancestor(msWebArea)
	appLauncherOpened := nodewith.Name("App launcher opened").Ancestor(msWebArea).First()
	oneDriveLink := nodewith.Name("OneDrive").Role(role.Link).Ancestor(appLauncherOpened)
	if err := uiauto.Combine("navigate to OneDrive",
		app.checkSignIn,
		app.uiHdl.ClickUntil(appLauncher, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(appLauncherOpened)),
		app.uiHdl.Click(oneDriveLink),
	)(ctx); err != nil {
		return nil, err
	}

	myFiles := nodewith.Name("My files").Role(role.Heading).First()
	if err := app.reload(myFiles, func(ctx context.Context) error { return nil })(ctx); err != nil {
		return nil, err
	}

	alertDialog := nodewith.Role(role.AlertDialog).First()
	closeDialog := nodewith.Name("Close dialog").Role(role.Button).Ancestor(alertDialog)
	if err := app.ui.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(closeDialog), app.uiHdl.Click(closeDialog))(ctx); err != nil {
		return nil, errors.Wrap(err, `failed to close the "Let's get you started" dialog`)
	}

	gotItButton := nodewith.Name("Got it").Role(role.Button)
	if err := app.ui.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(gotItButton), app.uiHdl.Click(gotItButton))(ctx); err != nil {
		return nil, err
	}

	return conn, nil
}

// openNewFile open a new document for the specified service.
func (app *MicrosoftWebOffice) openNewFile(service string) action.Action {
	return func(ctx context.Context) error {
		testing.ContextLog(ctx, "Opening a new ", service)

		oneDriveWebArea := nodewith.Name("My files - OneDrive").Role(role.RootWebArea)
		newItem := nodewith.Name("New").Role(role.MenuItem).Ancestor(oneDriveWebArea)
		newItemMenu := nodewith.Role(role.Menu).Ancestor(newItem)
		serviceItem := nodewith.Name(service).Role(role.MenuItem).Ancestor(oneDriveWebArea)
		return uiauto.Combine("create a new file",
			app.uiHdl.ClickUntil(newItem, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(newItemMenu)),
			app.uiHdl.ClickUntil(serviceItem, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilGone(oneDriveWebArea)),
		)(ctx)
	}
}

// openBlankDocument opens a blank document with specified service.
// When we try to open a blank document on the corresponding service page, it will jump to the Microsoft Office App to request permission.
// Therefore, we try to open a blank document from OneDrive to avoid this situation.
func (app *MicrosoftWebOffice) openBlankDocument(service string) action.Action {
	return func(ctx context.Context) error {
		if err := app.openNewFile(service)(ctx); err != nil {
			return err
		}

		paragraph := nodewith.Role(role.Paragraph).Editable()
		title := nodewith.Name("Click to add title").First()
		excelWebArea := nodewith.Name("Excel").Role(role.RootWebArea)
		canvas := nodewith.Role(role.Canvas).Ancestor(excelWebArea).First()

		// element defines the node to specify whether it navigates to the corresponding service page correctly.
		element := map[string]*nodewith.Finder{
			word:       paragraph,
			powerpoint: title,
			excel:      canvas,
		}

		return app.reload(element[service], app.openNewFile(service))(ctx)
	}
}

// reloadWordDocument reloads the page because it pops up "Couldn't save automatically" on Microsoft Word.
func (app *MicrosoftWebOffice) reloadWordDocument(ctx context.Context) error {
	reloadButtonName := "Reload Couldn't save automatically To avoid losing your changes, please copy them before you reload this document."
	reloadButton := nodewith.Name(reloadButtonName).Role(role.Button)
	if err := app.ui.WaitUntilExists(reloadButton)(ctx); err != nil {
		return nil
	}

	testing.ContextLog(ctx, `Reloading the page because it pops up "Couldn't save automatically"`)

	wordWebArea := nodewith.Name("Word").Role(role.RootWebArea)
	paragraph := nodewith.Role(role.Paragraph).Ancestor(wordWebArea).First()
	paragraphEditable := nodewith.Role(role.Paragraph).Ancestor(wordWebArea).Editable()
	return uiauto.Combine("copy content before reloading the page",
		app.uiHdl.Click(paragraph),
		app.kb.AccelAction("Ctrl+A"),
		app.kb.AccelAction("Ctrl+C"),
		app.uiHdl.Click(reloadButton),
		app.uiHdl.Click(paragraphEditable),
		app.kb.AccelAction("Ctrl+V"),
	)(ctx)
}

// clickNavigationItem clicks the specified item in the navigation bar.
func (app *MicrosoftWebOffice) clickNavigationItem(itemName string) action.Action {
	navigation := nodewith.Role(role.Navigation)
	navigationOffScreen := navigation.Offscreen()
	appMenu := nodewith.NameContaining("App menu").Role(role.MenuItem)
	navigationItem := nodewith.NameContaining(itemName).Role(role.MenuItem).Ancestor(navigation)
	return uiauto.Combine("click navigation bar item",
		app.ui.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(navigationOffScreen), app.uiHdl.Click(appMenu)),
		app.uiHdl.Click(navigationItem),
	)
}

// switchToListView switches the view option to list view.
func (app *MicrosoftWebOffice) switchToListView(ctx context.Context) error {
	testing.ContextLog(ctx, "Switching to list view")

	details := nodewith.Name("Details").Role(role.MenuItem)
	viewOptions := nodewith.NameContaining("View options").Role(role.MenuItem)
	viewOptionsExpanded := viewOptions.Expanded()
	listView := nodewith.NameContaining("List").Role(role.MenuItemCheckBox).Ancestor(viewOptions)
	if err := app.ui.Retry(retryTimes, func(ctx context.Context) error {
		return uiauto.Combine("switch to list view",
			// Sometimes "Details" will be displayed later, causing the position of the "View Options" button to change.
			app.ui.WaitUntilExists(details),
			app.uiHdl.ClickUntil(viewOptions, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(viewOptionsExpanded)),
			// After the page loads, OneDrive will reload and display again.
			// Therefore, the expanded list of "View Options" might disappear and cause the "List" option to not be found.
			app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(listView),
			app.uiHdl.Click(listView),
		)(ctx)
	})(ctx); err != nil {
		return err
	}

	// Wait for the list to be reordered. Otherwise, we might encounter an error while operating the list at the same time.
	return testing.Sleep(ctx, 2*time.Second)
}

// searchSampleSheet searches for the existence of the sample spreadsheet.
func (app *MicrosoftWebOffice) searchSampleSheet(ctx context.Context) error {
	testing.ContextLog(ctx, "Searching sample spreadsheet")

	// Check if the sample file exists via searching box.
	searchFromBox := func(ctx context.Context) error {
		testing.ContextLog(ctx, "Searching through the Searching box")
		search := nodewith.Role(role.Search)
		searchBox := nodewith.Role(role.ListBox).Ancestor(search)
		suggestedFiles := nodewith.Name("Suggested files").Role(role.Group).Ancestor(searchBox)
		fileOption := fmt.Sprintf("Excel file result: .xlsx %v.xlsx ,", sheetName)
		fileResult := nodewith.Name(fileOption).Role(role.ListBoxOption).Ancestor(suggestedFiles).First()
		return uiauto.Combine("search file via searching box",
			app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(search),
			app.uiHdl.Click(search),
			app.kb.TypeAction(sheetName),
			app.uiHdl.ClickUntil(fileResult, app.ui.Gone(fileResult)),
		)(ctx)
	}

	// Check if the sample file in the list of recently opened files.
	searchFromRecent := func(ctx context.Context) error {
		testing.ContextLog(ctx, "Searching from Recent")
		row := nodewith.NameContaining(sheetName).Role(role.Row).First()
		link := nodewith.NameContaining(sheetName).Role(role.Link).Ancestor(row)
		return uiauto.Combine("search file from recently opened",
			app.clickNavigationItem(recent),
			app.switchToListView,
			app.uiHdl.ClickUntil(link, app.ui.Gone(link)),
		)(ctx)
	}

	// Sometimes the search box node does not exist, try searching from "Recent".
	if err := searchFromBox(ctx); err != nil {
		return searchFromRecent(ctx)
	}

	return nil
}

// openFindAndSelect opens "Find & Select".
func (app *MicrosoftWebOffice) openFindAndSelect(ctx context.Context) error {
	testing.ContextLog(ctx, `Opening "Find & Select"`)

	findAndSelectButton := nodewith.Name("Find & Select").Role(role.PopUpButton)
	moreOptions := nodewith.Name("More Options").Role(role.Group)
	moreOptionsMenu := nodewith.Name("More Options").Role(role.Menu)
	editing := nodewith.Name("Editing").Role(role.Group).Ancestor(moreOptionsMenu)
	findAndSelectItem := nodewith.Name("Find & Select").Role(role.MenuItem).Ancestor(editing)

	// There might be two situations.
	// 1. TabPanel shows "Find & Select" directly.
	// 2. First click on "More Options" then you can find "Find & Select".
	found, err := app.ui.IsNodeFound(ctx, findAndSelectButton)
	if err != nil {
		return err
	}
	if found {
		return app.uiHdl.Click(findAndSelectButton)(ctx)
	}

	return uiauto.Combine("click more options",
		app.uiHdl.Click(moreOptions),
		app.uiHdl.Click(findAndSelectItem),
	)(ctx)
}

// selectRange selects the range by clicking on the "Name Box" or opening "Go to" box since the tapping response is different with clicking.
func (app *MicrosoftWebOffice) selectRange(ctx context.Context) error {
	testing.ContextLog(ctx, `Selecting "Range"`)

	// In the clamshell mode, the "Name Box" can be focused with just click.
	if !app.tabletMode {
		testing.ContextLog(ctx, `Selecting range by focus on "Name Box"`)
		nameBox := nodewith.Name("Name Box").Role(role.TextFieldWithComboBox).Editable()
		nameBoxFocused := nameBox.Focused()
		return app.uiHdl.ClickUntil(nameBox, app.ui.Exists(nameBoxFocused))(ctx)
	}

	rangeText := nodewith.Name("Range:").Role(role.TextField).Editable()
	rangeTextFocused := rangeText.Focused()
	// Pressing Ctrl+G will open the "Go To" box.
	// Sometimes key events are typed but the UI does not respond. Retry to alert dialog does appear.
	if err := app.ui.WithInterval(time.Second).RetryUntil(app.kb.AccelAction("Ctrl+G"), app.ui.Exists(rangeText))(ctx); err != nil {
		testing.ContextLog(ctx, "Opening with panel")

		home := nodewith.Name("Home").Role(role.Tab)
		homeTabPanel := nodewith.Name("Home").Role(role.TabPanel)
		goTo := nodewith.Name("Go to").Role(role.MenuItem)
		if err := uiauto.Combine(`open "Go To" with panel`,
			app.uiHdl.ClickUntil(home, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(homeTabPanel)),
			app.openFindAndSelect,
			app.ui.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(goTo), app.uiHdl.Click(goTo)),
		)(ctx); err != nil {
			return err
		}
	}

	return app.uiHdl.ClickUntil(rangeText, app.ui.Exists(rangeTextFocused))(ctx)
}

// selectBox selects the specified cell using the name box.
func (app *MicrosoftWebOffice) selectBox(box string) action.Action {
	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Selecting box(%v)", box)

		ok := nodewith.Name("OK").Role(role.Button).First()
		return uiauto.Combine(fmt.Sprintf("select box (%s) value", box),
			app.selectRange,
			app.kb.AccelAction("Ctrl+A"), // Make sure to clear the content and re-input.
			app.kb.TypeAction(box),
			app.kb.AccelAction("Enter"),
			app.ui.IfSuccessThen(app.ui.Exists(ok), app.ui.WithTimeout(defaultUIWaitTime).WaitUntilGone(ok)),
		)(ctx)
	}
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

	testing.ContextLogf(ctx, "Getting box (%s) value: %s", box, clipData)

	return clipData, nil
}

// editBoxValue edits the cell to the specified value.
func (app *MicrosoftWebOffice) editBoxValue(ctx context.Context, box, value string) error {
	testing.ContextLogf(ctx, "Writing box (%s) value: %s", box, value)

	return uiauto.Combine(fmt.Sprintf("write box (%s) value", box),
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
		return errors.New("incorrect formula")
	}

	return nil
}

// createSampleSheet creates a sample spreadsheet.
func (app *MicrosoftWebOffice) createSampleSheet(ctx context.Context) error {
	testing.ContextLog(ctx, "Creating a new spreadsheet")

	myFilesWebArea := nodewith.Name("My files - OneDrive").Role(role.RootWebArea)
	excelWebArea := nodewith.Name("Excel").Role(role.RootWebArea)
	canvas := nodewith.Role(role.Canvas).Ancestor(excelWebArea).First()
	savedButton := nodewith.NameContaining("Saved to OneDrive").Role(role.Button)
	fileNameBox := nodewith.Name("File Name").Role(role.TextField)

	if err := uiauto.Combine("open a blank spreadsheet",
		// Make sure we are in "My Files" so that "New" can be found.
		app.ui.IfSuccessThen(app.ui.Gone(myFilesWebArea), app.clickNavigationItem(myFiles)),
		app.openBlankDocument(excel),
		// Make sure canvas exists before typing. This is especially necessary on low-end DUTs.
		app.ui.WithTimeout(longerUIWaitTime).WaitUntilExists(canvas),
	)(ctx); err != nil {
		return err
	}

	testing.ContextLogf(ctx, "Writing cell(A1:A%d) values", rangeOfCells)
	for i := 1; i <= rangeOfCells; i++ {
		idx := strconv.Itoa(i)
		cell := fmt.Sprintf("A%d", i)
		if err := app.editBoxValue(ctx, cell, idx); err != nil {
			return err
		}
	}

	formula := fmt.Sprintf("=SUM(A1:A%d)", rangeOfCells)
	testing.ContextLog(ctx, "Writing cell(B1) value")
	if err := app.editBoxValue(ctx, "B1", formula); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Writing cell(H1) value")
	if err := app.editBoxValue(ctx, "H1", "Copy to document"); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Renaming spreadsheet")
	return uiauto.Combine("rename file",
		app.uiHdl.Click(savedButton),
		app.ui.DoubleClick(fileNameBox),
		app.kb.AccelAction("Ctrl+A"),
		app.kb.TypeAction(sheetName),
		app.kb.AccelAction("Enter"),
		app.ui.WaitUntilExists(savedButton),
	)(ctx)
}

// closeTab closes the tab with the title of the specified name.
func (app *MicrosoftWebOffice) closeTab(title string) action.Action {
	return func(ctx context.Context) error {
		matcher := func(t *target.Info) bool {
			return strings.Contains(t.Title, title) && t.Type == "page"
		}

		conn, err := app.cr.NewConnForTarget(ctx, matcher)
		if err != nil {
			return err
		}
		conn.CloseTarget(ctx)
		conn.Close()

		return nil
	}
}

// closeHelpPanel closes the "Help" panel if it exists.
func (app *MicrosoftWebOffice) closeHelpPanel(ctx context.Context) error {
	helpPanel := nodewith.Name("Help").Role(role.TabPanel)
	close := nodewith.Name("Close").Role(role.Button).Ancestor(helpPanel)
	return app.ui.IfSuccessThen(
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

	return app.ui.IfSuccessThen(
		app.ui.WaitUntilExists(startDictationButton),
		app.uiHdl.Click(startDictationButton),
	)(ctx)
}

// turnOnDictationFromMoreOptions turns on the dictation function through "More Options".
func (app *MicrosoftWebOffice) turnOnDictationFromMoreOptions(ctx context.Context) error {
	testing.ContextLog(ctx, `Turning on the dictation through "More options"`)

	homeTabPanel := nodewith.Name("Home").Role(role.TabPanel)
	moreOptions := nodewith.Name("More Options").Role(role.PopUpButton).Ancestor(homeTabPanel).First()
	dictationButton := nodewith.Name("Dictate").Role(role.Button).Ancestor(moreOptions).First()
	dictationCheckBox := nodewith.Name("Dictate").Role(role.MenuItemCheckBox).First()
	dictationToolbar := nodewith.Name("Dictation toolbar").Role(role.Toolbar)
	return uiauto.Combine(`turn on the dictation through "More Options"`,
		app.uiHdl.ClickUntil(moreOptions, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(dictationButton)),
		app.uiHdl.ClickUntil(dictationButton, app.ui.Gone(dictationButton)),
		app.ui.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(dictationCheckBox), app.uiHdl.Click(dictationCheckBox)),
		app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(dictationToolbar),
	)(ctx)
}

// turnOnDictationFromPanel turns on the dictation function via the button in the "Home" panel.
func (app *MicrosoftWebOffice) turnOnDictationFromPanel(ctx context.Context) error {
	testing.ContextLog(ctx, "Turning on the dictation through the panel")

	homeTabPanel := nodewith.Name("Home").Role(role.TabPanel)
	dictationToggleButton := nodewith.Name("Dictate").Role(role.ToggleButton).Ancestor(homeTabPanel).First()
	dictationCheckBox := nodewith.Name("Dictate").Role(role.MenuItemCheckBox).First()
	dictationToolbar := nodewith.Name("Dictation toolbar").Role(role.Toolbar)
	return uiauto.Combine("turn on the dictation through the panel",
		app.uiHdl.ClickUntil(dictationToggleButton, func(ctx context.Context) error {
			return uiauto.Combine("check if the dictation works",
				app.ui.IfSuccessThen(app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(dictationCheckBox), app.uiHdl.Click(dictationCheckBox)),
				app.ui.WaitUntilExists(dictationToolbar),
			)(ctx)
		}),
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
			app.clickNavigationItem(recent),
			app.uiHdl.Click(link),
		)(ctx)
	}

	homeTabPanel := nodewith.Name("Home").Role(role.TabPanel)

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
			if err := turnOnAction(ctx); err != nil {
				return err
			}
		}

		return nil
	}

	dictateButton := nodewith.Name("Dictate").Role(role.ToggleButton).Ancestor(homeTabPanel).First()
	if err := app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(dictateButton)(ctx); err != nil {
		if err := reoperate(ctx, app.turnOnDictationFromMoreOptions); err != nil {
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
		app.ui.IfSuccessThen(app.ui.Exists(featureBrokenContainer), app.uiHdl.Click(retryButton)),
		app.checkDictation,
	)(ctx)
}

// removeDocument removes the document with the specified file name.
func (app *MicrosoftWebOffice) removeDocument(ctx context.Context, fileName string) error {
	testing.ContextLog(ctx, "Removing an existing document: ", fileName)

	row := nodewith.NameContaining(fileName).Role(role.Row).First()
	checkBox := nodewith.NameContaining("Checkbox").Role(role.CheckBox).Ancestor(row)
	complementary := nodewith.Role(role.Complementary).First()
	commandBar := nodewith.NameContaining("Command bar").Role(role.MenuBar).Ancestor(complementary)
	remove := nodewith.Name("Remove").Role(role.MenuItem).Ancestor(commandBar)
	return uiauto.Combine("remove the file",
		app.switchToListView,
		app.uiHdl.ClickUntil(checkBox, app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(commandBar)),
		app.uiHdl.Click(remove),
	)(ctx)
}

// NewMicrosoftWebOffice creates MicrosoftWebOffice instance which implements ProductivityApp interface.
func NewMicrosoftWebOffice(cr *chrome.Chrome, tconn *chrome.TestConn, uiHdl cuj.UIActionHandler, kb *input.KeyboardEventWriter, tabletMode bool, username, password string) *MicrosoftWebOffice {
	return &MicrosoftWebOffice{
		cr:         cr,
		tconn:      tconn,
		ui:         uiauto.New(tconn),
		uiHdl:      uiHdl,
		kb:         kb,
		tabletMode: tabletMode,
		username:   username,
		password:   password,
	}
}

var _ ProductivityApp = (*MicrosoftWebOffice)(nil)
