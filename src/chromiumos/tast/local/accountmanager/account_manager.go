// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package accountmanager

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	androidui "chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// ChromeInterface is the common interface of chrome.Chrome and launcher.LacrosChrome.
type ChromeInterface interface {
	// TestAPIConn returns a new chrome.TestConn instance for the Chrome browser.
	TestAPIConn(ctx context.Context) (*chrome.TestConn, error)
	// NewConn creates a new Chrome renderer and returns a connection to it.
	NewConn(ctx context.Context, url string, opts ...cdputil.CreateTargetOption) (*chrome.Conn, error)
}

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
func OpenOneGoogleBar(ctx context.Context, tconn *chrome.TestConn, ci ChromeInterface) error {
	conn, err := ci.NewConn(ctx, "https://www.google.com")
	if err != nil {
		return errors.Wrap(err, "failed to create connection to google.com")
	}
	defer conn.Close()

	ui := uiauto.New(tconn)
	if err := openOGB(ctx, ui.WithTimeout(30*time.Second)); err != nil {
		// The page may have loaded in logged out state: reload and try again.
		reloadTab(ctx, ci)
		if err := openOGB(ctx, ui.WithTimeout(longUITimeout)); err != nil {
			return errors.Wrap(err, "failed to find OGB")
		}
	}
	return nil
}

func reloadTab(ctx context.Context, ci ChromeInterface) error {
	tconn, err := ci.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to test API")
	}

	tconn.Eval(ctx, "chrome.tabs.reload()", nil)
	return nil
}

// CheckOneGoogleBar opens OGB and checks that provided condition is true.
func CheckOneGoogleBar(ctx context.Context, tconn *chrome.TestConn, ci ChromeInterface, condition uiauto.Action) error {
	if err := OpenOneGoogleBar(ctx, tconn, ci); err != nil {
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

// IsAccountPresentInArc returns `true` if account is present in ARC Settings > Accounts.
func IsAccountPresentInArc(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, accountName string) (bool, error) {
	const (
		scrollClassName   = "android.widget.ScrollView"
		textViewClassName = "android.widget.TextView"
	)

	// Initialize UI automator for ARC.
	d, err := a.NewUIDevice(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed initializing UI Automator")
	}
	defer d.Close(ctx)

	if err := openARCSettings(ctx, tconn); err != nil {
		return false, errors.Wrap(err, "failed to Open ARC Settings")
	}

	defer func(ctx context.Context) error {
		// Cleanup: close the ARC Settings window.
		activeWindow, err := ash.GetActiveWindow(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get the active window")
		}
		if err := activeWindow.CloseWindow(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to close the active window "+activeWindow.Name)
		}
		return nil
	}(ctx)

	// Scroll until Accounts is visible.
	scrollLayout := d.Object(androidui.ClassName(scrollClassName),
		androidui.Scrollable(true))
	accounts := d.Object(androidui.ClassName("android.widget.TextView"),
		androidui.TextMatches("(?i)Accounts"), androidui.Enabled(true))
	if err := scrollLayout.WaitForExists(ctx, DefaultUITimeout); err == nil {
		scrollLayout.ScrollTo(ctx, accounts)
	}
	if err := accounts.Click(ctx); err != nil {
		return false, errors.Wrap(err, "failed to click Accounts in ARC settings")
	}

	account := d.Object(androidui.ClassName("android.widget.TextView"),
		androidui.TextMatches(accountName), androidui.Enabled(true))
	if err := scrollLayout.WaitForExists(ctx, DefaultUITimeout); err == nil {
		scrollLayout.ScrollTo(ctx, account)
	}

	if err := account.Exists(ctx); err != nil {
		return false, nil
	}
	return true, nil
}

// openARCSettings opens the ARC Settings Page from Chrome Settings.
func openARCSettings(ctx context.Context, tconn *chrome.TestConn) error {
	settings, err := ossettings.LaunchAtPage(ctx, tconn,
		nodewith.Name("Apps").Role(role.Heading))
	if err != nil {
		return errors.Wrap(err, "failed to open settings page")
	}
	playStoreButton := nodewith.Name("Google Play Store").Role(role.Button)
	if err := uiauto.Combine("Open Android Settings",
		settings.FocusAndWait(playStoreButton),
		settings.LeftClick(playStoreButton),
		settings.LeftClick(nodewith.Name("Manage Android preferences").Role(role.Link)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to open ARC settings page")
	}
	return nil
}
