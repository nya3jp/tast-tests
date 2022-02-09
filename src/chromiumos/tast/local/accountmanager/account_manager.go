// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package accountmanager

import (
	"context"
	"regexp"
	"time"

	androidui "chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// ChromePreWithFeaturesEnabled returns a precondition with flags enabled
func ChromePreWithFeaturesEnabled() testing.Precondition {
	return chrome.NewPrecondition("chrome_pre_with_arc_restrictions", chrome.EnableFeatures("ArcAccountRestrictions"))
}

// DefaultUITimeout is the default timeout for UI interactions.
const DefaultUITimeout = 20 * time.Second

// LongUITimeout is for interaction with webpages to make sure that page is loaded.
const LongUITimeout = time.Minute

// GetAddAccountDialog returns a root node of the system account addition dialog.
func GetAddAccountDialog() *nodewith.Finder {
	return nodewith.Name("Sign in to add a Google account").Role(role.RootWebArea)
}

// AddAccount adds an account in-session. Account addition dialog should be already open.
func AddAccount(ctx context.Context, tconn *chrome.TestConn, email, password string) error {
	// Set up keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard")
	}
	defer kb.Close()

	ui := uiauto.New(tconn).WithTimeout(DefaultUITimeout)

	// All nodes in the dialog should be inside the `root`.
	root := GetAddAccountDialog()

	// Click OK.
	okButton := nodewith.NameRegex(regexp.MustCompile("(OK|Continue)")).Role(role.Button).Ancestor(root)
	if err := uiauto.Combine("Click on OK and proceed",
		ui.WaitUntilExists(okButton),
		ui.LeftClick(okButton),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to click OK. Is Account addition dialog open?")
	}

	// Use long timeout to wait for the initial Gaia webpage load.
	if err := ui.WithTimeout(LongUITimeout).WaitUntilExists(nodewith.Role(role.Iframe).Ancestor(root))(ctx); err != nil {
		return errors.Wrap(err, "failed to find the iframe")
	}

	emailField := nodewith.Name("Email or phone").Role(role.TextField).Ancestor(root)
	backButton := nodewith.Name("Back").Role(role.Button).Ancestor(root)
	// After the iframe loads, the ui tree may not get updated. In this case only one retry (which hides and then shows the iframe again) is required.
	// TODO(b/211420351): remove this when the issue is fixed.
	if err := ui.Retry(2, func(ctx context.Context) error {
		if err := uiauto.Combine("Click on Username",
			ui.WaitUntilExists(emailField),
			ui.LeftClick(emailField),
		)(ctx); err == nil {
			// The email field input is found, the test can proceed.
			return nil
		}

		testing.ContextLog(ctx, "Couldn't find and click on user name inside the iframe node. Refreshing the ui tree")
		// Click 'Back' and then 'OK' to refresh the ui tree.
		// Note: This should be fast because it will just hide and show the webview/iframe node, but will not reload the webpage.
		if err := uiauto.Combine("Click 'Back' and 'OK' to refresh the iframe",
			ui.WaitUntilExists(backButton),
			ui.LeftClick(backButton),
			ui.WaitUntilExists(okButton),
			ui.LeftClick(okButton),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to click 'Back' and 'OK' to refresh the iframe")
		}

		return errors.New("failed to find and click on user name inside the iframe")
	})(ctx); err != nil {
		return errors.Wrap(err, "failed to click on user name")
	}

	// Enter the User Name.
	if err := kb.Type(ctx, email+"\n"); err != nil {
		return errors.Wrap(err, "failed to type user name")
	}

	// Enter Password.
	passwordField := nodewith.Name("Enter your password").Role(role.TextField).Ancestor(root)
	if err := uiauto.Combine("Click on Password",
		ui.WaitUntilExists(passwordField),
		ui.LeftClick(passwordField),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to click on password")
	}

	if err := kb.Type(ctx, password); err != nil {
		return errors.Wrap(err, "failed to type password")
	}

	nextButton := nodewith.Name("Next").Role(role.Button).Ancestor(root)
	iAgreeButton := nodewith.Name("I agree").Role(role.Button).Ancestor(root)
	if err := uiauto.Combine("Agree and Finish Adding Account",
		ui.LeftClick(nextButton),
		// We need to focus the button first to click at right location
		// as it returns wrong coordinates when button is offscreen.
		ui.FocusAndWait(iAgreeButton),
		ui.LeftClick(iAgreeButton),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to add account")
	}
	return nil
}

// CheckArcToggleStatus compares the state of the "ARC toggle" in the account addition flow with the expected value.
func CheckArcToggleStatus(ctx context.Context, tconn *chrome.TestConn, brType browser.Type, expectedVal bool) error {
	if brType != browser.TypeLacros {
		// The feature is applied only if Lacros is enabled.
		return nil
	}
	ui := uiauto.New(tconn).WithTimeout(DefaultUITimeout)
	root := GetAddAccountDialog()
	toggle := nodewith.NameStartingWith("Use this account with Android apps").Role(role.ToggleButton).Ancestor(root)
	if err := ui.WaitUntilExists(toggle)(ctx); err != nil {
		return errors.Wrap(err, "failed to find ARC toggle")
	}

	toggleInfo, err := ui.Info(ctx, toggle)
	if err != nil {
		return errors.Wrap(err, "failed to get ARC toggle info")
	}
	isToggleChecked := (toggleInfo.Checked == checked.True)
	if isToggleChecked != expectedVal {
		return errors.Errorf("expected toggle checked state to be %t but got %t", expectedVal, isToggleChecked)
	}

	return nil
}

// CheckOneGoogleBar opens OGB and checks that provided condition is true.
func CheckOneGoogleBar(ctx context.Context, tconn *chrome.TestConn, br *browser.Browser, condition uiauto.Action) error {
	if err := OpenOneGoogleBar(ctx, tconn, br); err != nil {
		return errors.Wrap(err, "failed to open OGB")
	}

	if err := testing.Poll(ctx, condition, nil); err != nil {
		return errors.Wrap(err, "failed to match condition after opening OGB")
	}

	return nil
}

// OpenOneGoogleBar opens google.com page in the browser and clicks on the One Google Bar.
func OpenOneGoogleBar(ctx context.Context, tconn *chrome.TestConn, br *browser.Browser) error {
	conn, err := br.NewConn(ctx, "chrome://newtab")
	if err != nil {
		return errors.Wrap(err, "failed to create connection to chrome://newtab")
	}
	defer conn.Close()

	if err := openOGB(ctx, tconn, 30*time.Second); err != nil {
		// The page may have loaded in logged out state: reload and try again.
		reloadTab(ctx, br)
		if err := openOGB(ctx, tconn, LongUITimeout); err != nil {
			return errors.Wrap(err, "failed to find OGB")
		}
	}
	return nil
}

func reloadTab(ctx context.Context, br *browser.Browser) error {
	tconn, err := br.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect Test API")
	}

	if err := tconn.Eval(ctx, "chrome.tabs.reload()", nil); err != nil {
		return errors.Wrap(err, "failed to reload")
	}
	if err := tconn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
		return errors.Wrap(err, "failed to wait for the ready state")
	}
	return nil
}

// openOGB opens OGB on already loaded webpage.
func openOGB(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration) error {
	ui := uiauto.New(tconn).WithTimeout(timeout)
	ogb := nodewith.NameStartingWith("Google Account").Role(role.Button)
	addAccount := nodewith.Name("Add another account").Role(role.Link)
	if err := uiauto.Combine("Click OGB",
		ui.WaitUntilExists(ogb),
		ui.WithInterval(time.Second).LeftClickUntil(ogb, ui.Exists(addAccount)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to find and click OGB")
	}
	return nil
}

// IsAccountPresentInArc returns `true` if account is present in ARC Settings > Accounts.
func IsAccountPresentInArc(ctx context.Context, tconn *chrome.TestConn, d *androidui.Device, accountName string) (bool, error) {
	const (
		scrollClassName   = "android.widget.ScrollView"
		textViewClassName = "android.widget.TextView"
	)

	if err := apps.Launch(ctx, tconn, apps.AndroidSettings.ID); err != nil {
		return false, errors.Wrap(err, "failed to launch AndroidSettings")
	}

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

// RemoveAccountFromOSSettings removes a secondary account from OS Settings. The "More actions" menu should be already open for that account.
func RemoveAccountFromOSSettings(ctx context.Context, tconn *chrome.TestConn, brType browser.Type) error {
	testing.ContextLog(ctx, "Removing account")

	ui := uiauto.New(tconn).WithTimeout(DefaultUITimeout)
	removeAccountButton := nodewith.Name("Remove this account").Role(role.MenuItem)
	if err := uiauto.Combine("Click Remove account",
		ui.WaitUntilExists(removeAccountButton),
		ui.LeftClick(removeAccountButton),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to to click Remove account")
	}

	if err := ui.WaitUntilExists(nodewith.Name("Remove this account?").First())(ctx); err != nil {
		if brType == browser.TypeLacros {
			return errors.Wrap(err, "failed to find confirmation dialog on Lacros")
		}
	} else {
		confirmRemoveButton := nodewith.Name("Remove").Role(role.Button)
		if err := uiauto.Combine("Confirm account removal",
			ui.WaitUntilExists(confirmRemoveButton),
			ui.LeftClick(confirmRemoveButton),
			ui.WaitUntilGone(confirmRemoveButton),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to click Remove account")
		}
	}
	return nil
}

// TestCleanup removes all secondary accounts in-session. Should be called at the beginning of the test, so that results of the previous test don't interfere with the current test.
func TestCleanup(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, brType browser.Type) error {
	ui := uiauto.New(tconn).WithTimeout(DefaultUITimeout)

	if err := ui.Exists(GetAddAccountDialog())(ctx); err == nil {
		// Set up keyboard.
		kb, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get keyboard")
		}
		defer kb.Close()
		// Press "Esc" to close the dialog.
		if err := kb.Accel(ctx, "Esc"); err != nil {
			return errors.Wrapf(err, "failed to write events %s", "Esc")
		}
	}

	// Open Account Manager page in OS Settings.
	addAccountButton := nodewith.Name("Add Google Account").Role(role.Button)
	_, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "accountManager", ui.Exists(addAccountButton))
	if err != nil {
		return errors.Wrap(err, "failed to launch Account Manager page")
	}

	moreActionsButton := nodewith.NameStartingWith("More actions,").Role(role.Button).First()
	for {
		moreActionsFound, err := ui.IsNodeFound(ctx, moreActionsButton)
		if err != nil {
			return errors.Wrap(err, "failed to search for More actions button")
		}
		if !moreActionsFound {
			// There are no "More actions, *" buttons left. It means all secondary accounts are removed.
			break
		}

		// Select specific "More actions, <email> button.
		info, err := ui.Info(ctx, moreActionsButton)
		if err != nil {
			return errors.Wrap(err, "failed to get More actions button info")
		}
		accountMoreActionsButton := nodewith.Name(info.Name).Role(role.Button)

		// Find and click "More actions, <email>" button.
		if err := uiauto.Combine("Click More actions",
			ui.WaitUntilExists(accountMoreActionsButton),
			ui.LeftClick(accountMoreActionsButton),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to click More actions button")
		}

		if err := RemoveAccountFromOSSettings(ctx, tconn, brType); err != nil {
			return errors.Wrap(err, "failed to remove account from OS Setting")
		}

		if err := ui.WaitUntilGone(accountMoreActionsButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait until account is removed")
		}
	}

	// Close all windows.
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get all open window")
	}
	for _, w := range ws {
		if err := w.CloseWindow(ctx, tconn); err != nil {
			return errors.Wrapf(err, "failed to close window (%+v)", w)
		}
	}

	return nil
}
