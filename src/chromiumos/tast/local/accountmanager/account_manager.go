// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package accountmanager

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// DefaultUITimeout is the default timeout for UI interactions.
const DefaultUITimeout = 20 * time.Second

// longUITimeout is for interaction with webpages to make sure that page is loaded.
const longUITimeout = time.Minute

// AddAccount adds an account in-session. Account addition dialog should be already open.
func AddAccount(ctx context.Context, tconn *chrome.TestConn,
	email, password string) error {
	ui := uiauto.New(tconn).WithTimeout(longUITimeout)
	// Click OK and Enter User Name.
	if err := uiauto.Combine("Click on OK and proceed",
		ui.WaitUntilExists(nodewith.Name("OK").Role(role.Button)),
		ui.LeftClick(nodewith.Name("OK").Role(role.Button)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to click OK. Is Account addition dialog open?")
	}

	if err := uiauto.Combine("Click on Username",
		ui.WaitUntilExists(nodewith.Name("Email or phone").Role(role.TextField)),
		ui.LeftClick(nodewith.Name("Email or phone").Role(role.TextField)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to click on user name")
	}

	// Set up keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard")
	}
	defer kb.Close()

	if err := kb.Type(ctx, email+"\n"); err != nil {
		return errors.Wrap(err, "failed to type user name")
	}

	// Enter Password.
	if err := uiauto.Combine("Click on Password",
		ui.WaitUntilExists(nodewith.Name("Enter your password").Role(role.TextField)),
		ui.LeftClick(nodewith.Name("Enter your password").Role(role.TextField)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to click on password")
	}

	if err := kb.Type(ctx, password); err != nil {
		return errors.Wrap(err, "failed to type password")
	}

	if err := uiauto.Combine("Agree and Finish Adding Account",
		ui.LeftClick(nodewith.Name("Next").Role(role.Button)),
		// We need to focus the button first to click at right location
		// as it returns wrong coordinates when button is offscreen.
		ui.FocusAndWait(nodewith.Name("I agree").Role(role.Button)),
		ui.LeftClick(nodewith.Name("I agree").Role(role.Button)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to add account")
	}
	return nil
}

// OpenOneGoogleBar opens google.com page in the browser and clicks on the One Google Bar.
func OpenOneGoogleBar(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) error {
	conn, err := cr.NewConn(ctx, "https://www.google.com")
	if err != nil {
		return errors.Wrap(err, "failed to create connection to google.com")
	}
	defer conn.Close()

	ui := uiauto.New(tconn).WithTimeout(longUITimeout)
	if err := openOGB(ctx, ui); err != nil {
		// The page may have loaded in logged out state: reload and try again.
		tconn.Eval(ctx, "chrome.tabs.reload()", nil)
		if err := openOGB(ctx, ui); err != nil {
			return errors.Wrap(err, "failed to find OGB")
		}
	}
	return nil
}

// CheckOneGoogleBar opens OGB and checks that provided condition is true.
func CheckOneGoogleBar(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, condition uiauto.Action) error {
	if err := OpenOneGoogleBar(ctx, cr, tconn); err != nil {
		return errors.Wrap(err, "failed to open OGB")
	}

	if err := testing.Poll(ctx, condition, nil); err != nil {
		return errors.Wrap(err, "failed to match condition after opening OGB")
	}

	return nil
}

// openOGB opens OGB on already loaded webpage.
func openOGB(ctx context.Context, ui *uiauto.Context) error {
	ogb := nodewith.NameStartingWith("Google Account").Role(role.Button)
	addAccount := nodewith.Name("Add another account").Role(role.Link)
	if err := uiauto.Combine("Click OGB",
		ui.WaitUntilExists(ogb),
		ui.LeftClick(ogb),
		ui.WaitUntilExists(addAccount),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to find and click OGB")
	}
	return nil
}
