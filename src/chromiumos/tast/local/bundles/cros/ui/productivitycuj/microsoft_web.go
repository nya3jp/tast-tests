// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package productivitycuj

import (
	"context"
	"fmt"
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
	// officeURL indicates the link URL of "Microsoft Office Home".
	officeURL = "https://www.office.com/"

	// myFiles indicates the "My files" item name in the navigation bar.
	myFiles = "My files"
	// recent indicates the "Recent" item label in the navigation bar.
	recent = "Recent"
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
	return nil
}

// CreateSlides creates a new presentation from microsoft web app.
func (app *MicrosoftWebOffice) CreateSlides(ctx context.Context) error {
	return nil
}

// CreateSpreadsheet creates a new spreadsheet from microsoft web app and returns the sheet name.
func (app *MicrosoftWebOffice) CreateSpreadsheet(ctx context.Context) (string, error) {
	return "", nil
}

// OpenSpreadsheet opens an existing spreadsheet from microsoft web app.
func (app *MicrosoftWebOffice) OpenSpreadsheet(ctx context.Context, fileName string) error {
	testing.ContextLog(ctx, "Opening an existing spreadsheet: ", fileName)

	conn, err := app.openOneDrive(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open OneDrive")
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
