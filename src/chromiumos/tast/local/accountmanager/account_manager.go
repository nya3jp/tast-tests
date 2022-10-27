// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package accountmanager

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/action"
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

// DefaultUITimeout is the default timeout for UI interactions.
const DefaultUITimeout = 20 * time.Second

// LongUITimeout is for interaction with webpages to make sure that page is loaded.
const LongUITimeout = time.Minute

// ARCAccountOptions is a struct containing options for `CheckIsAccountPresentInARC` call.
type ARCAccountOptions struct {
	// Name of the account to be checked
	accountName string
	// Whether the account is expected to be present in ARC
	expectedPresentInARC bool
	// Whether the account was previously present in ARC. If set to `true` -
	// `WaitUntilGone` will be used instead of `WaitForExists` while checking the
	// account presence.
	previouslyPresentInARC bool
}

// NewARCAccountOptions returns a `ARCAccountOptions` object for provided username.
// `expectedPresentInARC` and `previouslyPresentInARC` flags are set to `false`.
func NewARCAccountOptions(accountName string) ARCAccountOptions {
	return ARCAccountOptions{
		accountName:            accountName,
		expectedPresentInARC:   false,
		previouslyPresentInARC: false,
	}
}

// PreviouslyPresentInARC returns a copy of the struct with
// `previouslyPresentInARC` flag set to the specified value.
func (c ARCAccountOptions) PreviouslyPresentInARC(present bool) ARCAccountOptions {
	return ARCAccountOptions{
		accountName:            c.accountName,
		expectedPresentInARC:   c.expectedPresentInARC,
		previouslyPresentInARC: present,
	}
}

// ExpectedPresentInARC returns a copy of the struct with
// `expectedPresentInARC` flag set to the specified value.
func (c ARCAccountOptions) ExpectedPresentInARC(expected bool) ARCAccountOptions {
	return ARCAccountOptions{
		accountName:            c.accountName,
		expectedPresentInARC:   expected,
		previouslyPresentInARC: c.previouslyPresentInARC,
	}
}

// AccountName return `accountName`.
func (c ARCAccountOptions) AccountName() string {
	return c.accountName
}

// IsPreviouslyPresentInARC return `previouslyPresentInARC`.
func (c ARCAccountOptions) IsPreviouslyPresentInARC() bool {
	return c.previouslyPresentInARC
}

// IsExpectedPresentInARC return `expectedPresentInARC`.
func (c ARCAccountOptions) IsExpectedPresentInARC() bool {
	return c.expectedPresentInARC
}

// AddAccountDialog returns a root node of the system account addition dialog.
func AddAccountDialog() *nodewith.Finder {
	return nodewith.Name("Sign in to add a Google account").Role(role.RootWebArea)
}

// RemoveActionButton returns a button which removes the selected account on click.
func RemoveActionButton() *nodewith.Finder {
	return nodewith.Name("Remove this account").Role(role.MenuItem)
}

// GetChromeProfileWindow returns a nodewith.Finder to the Chrome window which
// matches the provided condition.
func GetChromeProfileWindow(ctx context.Context, tconn *chrome.TestConn, condition func(uiauto.NodeInfo) bool) (*nodewith.Finder, error) {
	profileWindow := nodewith.NameContaining("Google Chrome").Role(role.Window).HasClass("BrowserRootView")
	ui := uiauto.New(tconn).WithTimeout(DefaultUITimeout)

	var result *nodewith.Finder
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		profileInfos, err := ui.NodesInfo(ctx, profileWindow)
		if err != nil {
			return testing.PollBreak(errors.Wrapf(err, "failed to get info for %v", profileWindow.Pretty()))
		}
		for i := range profileInfos {
			if condition(profileInfos[i]) {
				result = profileWindow.Name(profileInfos[i].Name)
				return nil
			}
		}
		return errors.Errorf("failed to find the Chrome window matching the condition: %v windows were checked", len(profileInfos))
	}, &testing.PollOptions{Timeout: DefaultUITimeout}); err != nil {
		return nil, errors.Wrap(err, "failed to find the Chrome window matching the condition")
	}

	return result, nil
}

// OpenAccountManagerSettingsAction returns an action that opens OS Settings > Accounts.
func OpenAccountManagerSettingsAction(tconn *chrome.TestConn, cr *chrome.Chrome) action.Action {
	return func(ctx context.Context) error {
		ui := uiauto.New(tconn).WithTimeout(DefaultUITimeout)
		// Open Account Manager page in OS Settings and find Add Google Account button.
		if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "accountManager", ui.Exists(nodewith.Name("Add Google Account").Role(role.Button))); err != nil {
			return errors.Wrap(err, "failed to launch Account Manager page")
		}
		return nil
	}
}

// startAddAccount navigates to the Gaia screen in account addition dialog.
// On success, the email page is shown and the email field is focused.
func startAddAccount(ctx context.Context, kb *input.KeyboardEventWriter, ui *uiauto.Context, email string) error {
	// All nodes in the dialog should be inside the `root`.
	root := AddAccountDialog()

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
			ui.LeftClickUntil(emailField, ui.Exists(emailField.Focused())),
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

	return nil
}

// AddAccount adds an account in-session. Account addition dialog must be already open.
func AddAccount(ctx context.Context, tconn *chrome.TestConn, email, password string) error {
	// Set up keyboard.
	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard")
	}
	defer kb.Close()

	ui := uiauto.New(tconn).WithTimeout(DefaultUITimeout)

	if err := startAddAccount(ctx, kb, ui, email); err != nil {
		return errors.Wrap(err, "failed to start account addition")
	}

	// All nodes in the dialog should be inside the `root`.
	root := AddAccountDialog()

	passwordField := nodewith.Name("Enter your password").Role(role.TextField).Ancestor(root)
	nextButton := nodewith.Name("Next").Role(role.Button).Ancestor(root)
	iAgreeButton := nodewith.Name("I agree").Role(role.Button).Ancestor(root)

	if err := uiauto.Combine("Enter email and password",
		// Enter the User Name.
		kb.TypeAction(email+"\n"),
		ui.WaitUntilExists(passwordField),
		ui.LeftClick(passwordField),
		// Enter the Password.
		kb.TypeAction(password),
		ui.LeftClick(nextButton),
		// We need to focus the button first to click at right location
		// as it returns wrong coordinates when button is offscreen.
		ui.FocusAndWait(iAgreeButton),
		ui.LeftClick(iAgreeButton),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to enter email and password")
	}

	return nil
}

// AddAccountSAML adds a SAML (at the moment only Microsoft) account in-session.
// Account addition dialog must be already open.
func AddAccountSAML(ctx context.Context, tconn *chrome.TestConn, email, password string) error {
	// Set up keyboard.
	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard")
	}
	defer kb.Close()

	ui := uiauto.New(tconn).WithTimeout(DefaultUITimeout)

	if err := startAddAccount(ctx, kb, ui, email); err != nil {
		return errors.Wrap(err, "failed to start account addition")
	}

	// All nodes in the dialog should be inside the `root`.
	root := AddAccountDialog()

	samlEmailField := nodewith.NameContaining("Enter your email, phone, or Skype").Role(role.TextField).Ancestor(root)
	passwordField := nodewith.NameContaining("Enter the password").Role(role.TextField).Ancestor(root)
	noButton := nodewith.Name("No").Role(role.Button).Ancestor(root).Focusable()

	if err := uiauto.Combine("Enter SAML email and password",
		// Enter the User Name.
		kb.TypeAction(email+"\n"),
		// Enter the User Name on the SAML page.
		ui.WaitUntilExists(samlEmailField),
		ui.LeftClickUntil(samlEmailField, ui.Exists(samlEmailField.Focused())),
		kb.TypeAction(email+"\n"),
		// Enter the Password.
		ui.WaitUntilExists(passwordField),
		ui.LeftClickUntil(passwordField, ui.Exists(passwordField.Focused())),
		kb.TypeAction(password+"\n"),
		// On "Stay signed in?" screen select "No".
		ui.WaitUntilExists(noButton),
		ui.DoDefault(noButton),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to enter SAML email and password")
	}

	return nil
}

// CheckARCToggleStatus compares the state of the "ARC toggle" in the account
// addition flow with the expected value.
func CheckARCToggleStatus(ctx context.Context, tconn *chrome.TestConn, brType browser.Type, expectedVal bool) error {
	if brType != browser.TypeLacros {
		// The feature is applied only if Lacros is enabled.
		return nil
	}
	ui := uiauto.New(tconn).WithTimeout(DefaultUITimeout)
	root := AddAccountDialog()
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

// CheckARCToggleStatusAction returns an action that runs accountmanager.CheckARCToggleStatus.
func CheckARCToggleStatusAction(tconn *chrome.TestConn, brType browser.Type, expectedVal bool) action.Action {
	return func(ctx context.Context) error {
		if err := CheckARCToggleStatus(ctx, tconn, brType, expectedVal); err != nil {
			return errors.Wrap(err, "failed to check ARC toggle status")
		}
		return nil
	}
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
		br.ReloadActiveTab(ctx)
		if err := openOGB(ctx, tconn, LongUITimeout); err != nil {
			return errors.Wrap(err, "failed to find OGB")
		}
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

// CheckIsAccountPresentInARCAction returns an action that checks whether account
// is present in ARC depending on expectedPresentInARC parameter.
func CheckIsAccountPresentInARCAction(tconn *chrome.TestConn, d *androidui.Device, options ARCAccountOptions) action.Action {
	return func(ctx context.Context) error {
		if err := OpenARCAccountsInARCSettings(ctx, tconn, d); err != nil {
			return err
		}
		return CheckIsAccountPresentInARC(ctx, tconn, d, options)
	}
}

// CheckIsAccountPresentInARC checks whether account is present in ARC depending
// on expectedPresentInARC parameter. The ARC accounts page should be already open.
func CheckIsAccountPresentInARC(ctx context.Context, tconn *chrome.TestConn, d *androidui.Device, options ARCAccountOptions) error {
	const (
		// Note: it may take long time for account to be propagated to ARC.
		// When increasing this timeout, consider increasing timeout of the tests which call this method.
		arcAccountCheckTimeout = time.Minute
		scrollClassName        = "android.widget.ScrollView"
	)

	account := d.Object(androidui.ClassName("android.widget.TextView"),
		androidui.TextMatches(options.AccountName()), androidui.Enabled(true))

	if options.IsPreviouslyPresentInARC() {
		goneErr := account.WaitUntilGone(ctx, arcAccountCheckTimeout)
		if goneErr == nil && options.IsExpectedPresentInARC() {
			return errors.New("the account is gone, but expected to be present")
		} else if goneErr != nil && !options.IsExpectedPresentInARC() {
			return errors.Wrap(goneErr, "failed to wait for account to be gone")
		}

		return nil
	}

	existsErr := account.WaitForExists(ctx, arcAccountCheckTimeout)
	if existsErr == nil && !options.IsExpectedPresentInARC() {
		return errors.New("the account is present, but expected to be gone")
	} else if existsErr != nil && options.IsExpectedPresentInARC() {
		return errors.Wrap(existsErr, "failed to wait for account present")
	}

	return nil
}

// OpenARCAccountsInARCSettings opens ARC account list page in ARC Settings.
func OpenARCAccountsInARCSettings(ctx context.Context, tconn *chrome.TestConn, d *androidui.Device) error {
	const scrollClassName = "android.widget.ScrollView"

	if err := apps.Launch(ctx, tconn, apps.AndroidSettings.ID); err != nil {
		return errors.Wrap(err, "failed to launch AndroidSettings")
	}

	// Scroll until Accounts is visible.
	scrollLayout := d.Object(androidui.ClassName(scrollClassName),
		androidui.Scrollable(true))
	accounts := d.Object(androidui.ClassName("android.widget.TextView"),
		androidui.TextMatches("(?i)Accounts"), androidui.Enabled(true))
	if err := scrollLayout.WaitForExists(ctx, DefaultUITimeout); err == nil {
		if scrollErr := scrollLayout.ScrollTo(ctx, accounts); scrollErr != nil {
			return errors.Wrap(scrollErr, "failed to scroll to accounts button")
		}
	}
	if err := accounts.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Accounts in ARC settings")
	}
	// Confirm the accounts page was opened.
	accountsLabel := d.Object(androidui.ClassName("android.widget.TextView"),
		androidui.TextMatches("(?i)Accounts for owner"))
	if err := accountsLabel.WaitForExists(ctx, DefaultUITimeout); err != nil {
		return errors.Wrap(err, "failed to open Accounts in ARC settings")
	}
	return nil
}

// RemoveAccountFromOSSettings removes a secondary account with provided email from OS Settings.
func RemoveAccountFromOSSettings(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, email string) error {
	ui := uiauto.New(tconn).WithTimeout(DefaultUITimeout)
	moreActionsButton := nodewith.Name("More actions, " + email).Role(role.Button)

	if err := uiauto.Combine("Click More actions",
		// Open OS Settings again.
		OpenAccountManagerSettingsAction(tconn, cr),
		// Find and click "More actions, <email>" button.
		ui.FocusAndWait(moreActionsButton),
		ui.LeftClickUntil(moreActionsButton, ui.Exists(RemoveActionButton())),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to click More actions button")
	}

	if err := removeSelectedAccountFromOSSettings(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to remove account from OS Settings")
	}
	return nil
}

// removeSelectedAccountFromOSSettings removes a secondary account from OS Settings.
// The "More actions" menu should be already open for that account.
func removeSelectedAccountFromOSSettings(ctx context.Context, tconn *chrome.TestConn) error {
	testing.ContextLog(ctx, "Removing account")

	ui := uiauto.New(tconn).WithTimeout(DefaultUITimeout)
	removeAccountButton := RemoveActionButton()
	if err := uiauto.Combine("Click Remove account",
		ui.WaitUntilExists(removeAccountButton),
		ui.LeftClick(removeAccountButton),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to to click Remove account")
	}

	return nil
}

// TestCleanup removes all secondary accounts in-session. Should be called at
// the beginning of the test, so that results of the previous test don't
// interfere with the current test.
func TestCleanup(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) error {
	ui := uiauto.New(tconn).WithTimeout(DefaultUITimeout)

	if err := ui.Exists(AddAccountDialog())(ctx); err == nil {
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

	moreActionsButton := nodewith.NameStartingWith("More actions,").Role(role.Button).First()
	for {
		// Open Account Manager page in OS Settings.
		if err := OpenAccountManagerSettingsAction(tconn, cr)(ctx); err != nil {
			return errors.Wrap(err, "failed to launch Account Manager page")
		}

		// Wait for 5 seconds for the account list to appear.
		if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(moreActionsButton)(ctx); err != nil {
			if strings.Contains(err.Error(), nodewith.ErrNotFound) && strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
				// There are no "More actions, *" buttons left. It means all secondary accounts are removed.
				break
			}

			return errors.Wrap(err, "failed to wait for More actions buttons")
		}

		// Select specific "More actions, <email>" button.
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

		if err := removeSelectedAccountFromOSSettings(ctx, tconn); err != nil {
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
