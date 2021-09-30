// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package productivitycuj

import (
	"context"

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
	return nil
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
func (app *MicrosoftWebOffice) VoiceToTextTesting(ctx context.Context, expectedText string, playAudio func(ctx context.Context) error) error {
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

		return errors.New("failed to sign in microsoft office, your account has been locked")
	}

	testing.ContextLog(ctx, "Account has been logged in")
	return nil
}

// checkIfAccountFilled checks if the account field is automatically filled in.
func (app *MicrosoftWebOffice) checkIfAccountFilled(expected string) action.Action {
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
			testing.ContextLog(ctx, "Filled in the wrong account information")
			return uiauto.Combine("fill in the account",
				app.kb.AccelAction("Ctrl+A"),
				app.kb.TypeAction(app.username),
			)(ctx)
		}
		// If it has filled in the correct account information, skip entering it again.
		testing.ContextLog(ctx, "Filled in the correct account information")
		return nil
	}
}

// signIn signs in to Microsoft Office account.
func (app *MicrosoftWebOffice) signIn(ctx context.Context) error {
	testing.ContextLog(ctx, "Signing in to Microsoft Office")

	nextButton := nodewith.Name("Next").Role(role.Button)

	fillAccount := func(ctx context.Context) error {
		testing.ContextLog(ctx, "Filling in the account field")
		return uiauto.Combine("fill in the account",
			app.checkIfAccountFilled(app.username),
			app.uiHdl.Click(nextButton),
		)(ctx)
	}

	passwordField := nodewith.Name("Enter the password for " + app.username).Role(role.TextField)
	signInButton := nodewith.Name("Sign in").Role(role.Button)
	staySignInHeading := nodewith.Name("Stay signed in?").Role(role.Heading)
	yesButton := nodewith.Name("Yes").Role(role.Button)
	closeButton := nodewith.Name("Close first run experience").Role(role.Button)

	fillPassword := func(ctx context.Context) error {
		testing.ContextLog(ctx, "Filling in the password field")
		return uiauto.Combine("fill in the password",
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
		fillAccount,
	)(ctx); err != nil {
		return err
	}

	// Sometimes it will login directly without filling in the password.
	return app.ui.IfSuccessThen(
		app.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(passwordField),
		fillPassword,
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
