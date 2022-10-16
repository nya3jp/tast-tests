// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package familylink is used for writing Family Link tests.
package familylink

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// AddEduSecondaryAccount opens the EDU Coexistence in-session flow
// and attempts to add a secondary account for a Family Link (FL)
// primary user.
// FL users can only have EDU secondary accounts. Trying to add other account
// types will fail.
// If `verifyEduSecondaryAddSuccess` is set to true - the account addition will
// be verified ("School account added" message in the dialog + confirming that
// new account was added in OS Settings).
// Precondition: The current logged in user must be FL (such as Geller
// or Unicorn).
func AddEduSecondaryAccount(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn,
	parentUser, parentPass, secondUser, secondPass string,
	verifyEduSecondaryAddSuccess bool) error {

	ui := uiauto.New(tconn).WithTimeout(20 * time.Second)

	testing.ContextLog(ctx, "Checking logged in user is Family Link")
	if err := ui.Exists(nodewith.Name("This account is managed by Family Link").Role(role.Image))(ctx); err != nil {
		return errors.Wrap(err, "logged in user is not Family Link")
	}

	testing.ContextLog(ctx, "Launching the settings app")
	addSchoolAccountButton := nodewith.Name("Add school account").Role(role.Button)
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "accountManager", ui.Exists(addSchoolAccountButton)); err != nil {
		return errors.Wrap(err, "failed to launch Account Manager page")
	}

	selectParentOption := nodewith.NameContaining(strings.ToLower(parentUser)).Role(role.ListBoxOption)
	parentPasswordField := nodewith.Name("Parent password").Role(role.TextField)
	// This condition passes if either selectParentOption or parentPassswordField exists.
	condition := func(ctx context.Context) error {
		err1 := ui.Exists(selectParentOption)(ctx)
		err2 := ui.Exists(parentPasswordField)(ctx)
		if err1 != nil && err2 != nil {
			return errors.Wrap(err1, "Neither the select parent option nor the parent password field exists: "+err2.Error())
		}
		return nil
	}

	if err := ui.WithInterval(time.Second).LeftClickUntil(addSchoolAccountButton, condition)(ctx); err != nil {
		return errors.Wrap(err, "failed to open in-session EDU Coexistence flow")
	}

	if err := maybePressSelectParentOption(ctx, tconn, selectParentOption, parentPasswordField, parentUser); err != nil {
		return err
	}

	if err := NavigateEduCoexistenceFlow(ctx, cr, tconn, parentPass, secondUser, secondPass); err != nil {
		return errors.Wrap(err, "failed to navigate in-session EDU Coexistence flow")
	}

	if verifyEduSecondaryAddSuccess {
		return verifyAccountAddition(ctx, cr, tconn, secondUser)
	}

	return nil
}

// maybePressSelectParentOption selects the correct parent from parent
// list if there's multiple parents. Otherwise, the EDU Coexistence
// flow skips directly to the parent password page.
func maybePressSelectParentOption(ctx context.Context, tconn *chrome.TestConn, selectParentOption, parentPasswordField *nodewith.Finder, parentUser string) error {
	ui := uiauto.New(tconn).WithTimeout(20 * time.Second)

	if err := ui.Exists(selectParentOption)(ctx); err != nil {
		return nil
	}

	testing.ContextLog(ctx, "Clicking button that matches parent email: ", parentUser)
	if err := ui.WithInterval(time.Second).LeftClickUntil(selectParentOption, ui.Exists(parentPasswordField))(ctx); err != nil {
		return errors.Wrap(err, "failed to click button that matches parent email")
	}
	return nil

}

// verifyAccountAddition verifies account addition ("School account added"
// message in the dialog + confirming that new account was added in OS Settings).
func verifyAccountAddition(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, eduEmail string) error {
	ui := uiauto.New(tconn).WithTimeout(20 * time.Second)

	testing.ContextLog(ctx, "Clicking next on the final page to wrap up")
	schoolAccountAddedHeader := nodewith.Name("School account added").Role(role.Heading)
	if err := uiauto.Combine("Clicking next button and wrapping up",
		ui.WaitUntilExists(schoolAccountAddedHeader),
		ui.LeftClickUntil(nodewith.Name("Next").Role(role.Button), ui.Gone(schoolAccountAddedHeader)))(ctx); err != nil {
		return errors.Wrap(err, "failed to click next button")
	}

	testing.ContextLog(ctx, "Verifying the EDU secondary account added successfully")
	// There should be a "more actions" button to remove the EDU secondary account.
	moreActionsButton := nodewith.Name("More actions, " + eduEmail).Role(role.Button)
	if err := ui.WaitUntilExists(moreActionsButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to detect EDU secondary account")
	}

	return nil
}

// NavigateEduCoexistenceFlow goes through the EDU Coexistence
// in-session flow and attempts to add a secondary account for a
// Family Link (FL) primary user. FL users can only have EDU secondary
// accounts. Trying to add other account types will fail.
// Precondition: The current logged in user must be FL (such as Geller
// or Unicorn).
func NavigateEduCoexistenceFlow(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn,
	parentPass, secondUser, secondPass string) error {
	ui := uiauto.New(tconn).WithTimeout(20 * time.Second)

	testing.ContextLog(ctx, "Checking logged in user is Family Link")
	if err := ui.Exists(nodewith.Name("This account is managed by Family Link").Role(role.Image))(ctx); err != nil {
		return errors.Wrap(err, "logged in user is not Family Link")
	}

	testing.ContextLog(ctx, "Clicking the parent password text field")
	if err := ui.LeftClick(nodewith.Name("Parent password").Role(role.TextField))(ctx); err != nil {
		return errors.Wrap(err, "failed to click parent password text")
	}

	testing.ContextLog(ctx, "Setting up keyboard")
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard")
	}
	defer kb.Close()

	// TODO(chromium:12227440): Reduce typing flakiness and replace \n with a more
	// consistent way to navigate to the next screen, here and other places.
	testing.ContextLog(ctx, "Typing the parent password")
	if err := kb.Type(ctx, parentPass+"\n"); err != nil {
		return errors.Wrap(err, "failed to type parent password")
	}

	testing.ContextLog(ctx, "Clicking next on school account information for parents and Google workspace for education information pages")
	nextButton := nodewith.Name("Next").Role(role.Button)
	enterSchoolEmailText := nodewith.Name("School email").Role(role.TextField)
	if err := uiauto.Combine("Clicking next",
		ui.WaitUntilExists(nextButton),
		ui.WithInterval(time.Second).LeftClickUntil(nextButton, ui.Exists(enterSchoolEmailText)))(ctx); err != nil {
		return errors.Wrap(err, "failed to click Next button")
	}

	gaiaConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURLPrefix("https://accounts.google.com/"))
	if err != nil {
		return errors.Wrap(err, "failed to connect to GAIA webview target")
	}
	defer gaiaConn.Close()

	testing.ContextLog(ctx, "Authenticating secondary EDU account")
	if err := InsertFieldVal(ctx, gaiaConn, "input[name=identifier]", secondUser); err != nil {
		return errors.Wrap(err, "failed to fill in school email")
	}
	if err := ui.LeftClick(nextButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to click next on school email page")
	}

	if err := InsertFieldVal(ctx, gaiaConn, "input[name=password]", secondPass); err != nil {
		return errors.Wrap(err, "failed to fill in school password")
	}
	if err := ui.LeftClick(nextButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to click next on school password page")
	}

	return nil
}

// InsertFieldVal directly sets the value of an input field using JS.
func InsertFieldVal(ctx context.Context, conn *chrome.Conn, selector, value string) error {
	if err := conn.WaitForExpr(ctx, fmt.Sprintf(
		"document.querySelector(%q)", selector)); err != nil {
		return errors.Wrapf(err, "failed to wait for %q", selector)
	}

	if err := conn.Call(ctx, nil, `(selector, value) => {
	  const field = document.querySelector(selector);
	  field.value = value;
	}`, selector, value); err != nil {
		return errors.Wrapf(err, "failed to set the value for field %q", selector)
	}

	return nil
}

// CreateUsageTimeLimitPolicy returns the *policy.UsageTimeLimit with default
// settings. Its value needs to be overridden for test.
func CreateUsageTimeLimitPolicy() *policy.UsageTimeLimit {
	now := time.Now()
	dailyTimeUsageLimit := policy.RefTimeUsageLimitEntry{
		LastUpdatedMillis: strconv.FormatInt(now.Unix(), 10 /*base*/),
		// There's 1440 minutes in a day, so no screen time limit.
		UsageQuotaMins: 1440,
	}

	hour, _, _ := now.Clock()
	resetTime := policy.RefTime{
		// Make sure the policy doesn't reset before the test ends
		Hour:   (hour + 2) % 24,
		Minute: 0,
	}
	return &policy.UsageTimeLimit{
		Val: &policy.UsageTimeLimitValue{
			Overrides: []*policy.UsageTimeLimitValueOverrides{},
			TimeUsageLimit: &policy.UsageTimeLimitValueTimeUsageLimit{
				Friday:    &dailyTimeUsageLimit,
				Monday:    &dailyTimeUsageLimit,
				ResetAt:   &resetTime,
				Saturday:  &dailyTimeUsageLimit,
				Sunday:    &dailyTimeUsageLimit,
				Thursday:  &dailyTimeUsageLimit,
				Tuesday:   &dailyTimeUsageLimit,
				Wednesday: &dailyTimeUsageLimit,
			},
			TimeWindowLimit: &policy.UsageTimeLimitValueTimeWindowLimit{
				Entries: []*policy.UsageTimeLimitValueTimeWindowLimitEntries{},
			},
		},
	}
}

// VerifyUserSignedIntoBrowserAsChild creates and opens the browser, then checks that the provided email is signed in and recognized as a child user.
// Note that `cr` and `tconn` passed in should be from ash-chrome.
func VerifyUserSignedIntoBrowserAsChild(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, bt browser.Type, email, outDir string) (err error) {
	testing.ContextLog(ctx, "Verifying user is signed in as a child in the browser")

	// Reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Set up browser and open a new tab window.
	testing.ContextLog(ctx, "Opening browser with family link internals page")
	conn, _, closeBrowser, err := browserfixt.SetUpWithURL(ctx, cr, bt, "chrome://family-link-user-internals/")
	if err != nil {
		return errors.Wrap(err, "failed to set up browser")
	}
	defer closeBrowser(cleanupCtx)
	defer conn.Close()

	ui := uiauto.New(tconn).WithTimeout(time.Minute)
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, func() bool { return err != nil }, cr, "ui_tree")

	// Parse family link internals table into sections.
	testing.ContextLog(ctx, "Parsing family link internals page")
	sectionsContainers := nodewith.Role(role.GenericContainer).Ancestor(nodewith.Role(role.WebView))
	if err := ui.WaitUntilExists(sectionsContainers.First())(ctx); err != nil {
		return errors.Wrap(err, "failed to parse family link user internals table")
	}
	sectionsNodes, err := ui.NodesInfo(ctx, sectionsContainers)
	if err != nil {
		return errors.Wrap(err, "failed to retrieve nodes info for table sections")
	}

	// Find profile section in the table. Profile section contains information to verify further.
	testing.ContextLog(ctx, "Looking for profile section")
	index := -1
	for i := range sectionsNodes {
		container := sectionsContainers.Nth(i)
		profileSection := nodewith.Role(role.Heading).Name("Profile").Ancestor(container)
		if err := ui.Exists(profileSection)(ctx); err != nil {
			continue
		}
		testing.ContextLog(ctx, "Found profile section at index:", i)
		index = i
		break
	}

	if index == -1 {
		return errors.Wrap(err, "profile section not found")
	}

	// Parse table rows in the profile section.
	testing.ContextLog(ctx, "Parsing profile section rows")
	profileRows := nodewith.Role(role.LayoutTableRow).Ancestor(sectionsContainers.Nth(index))
	if err := ui.WaitUntilExists(profileRows.First())(ctx); err != nil {
		return errors.Wrap(err, "failed to parse profile rows")
	}
	profileNodes, err := ui.NodesInfo(ctx, profileRows)
	if err != nil {
		return errors.Wrap(err, "failed to retrieve nodes info for profile rows")
	}

	// Find account email row and verify that the email value.
	testing.ContextLog(ctx, "Verifying account email row")
	emailVerified := false
	for i := range profileNodes {
		row := profileRows.Nth(i)
		accountCell := nodewith.Role(role.LayoutTableCell).Name("Account").Ancestor(row)
		if err := ui.Exists(accountCell)(ctx); err != nil {
			continue
		}
		emailCell := nodewith.Role(role.LayoutTableCell).Name(strings.ToLower(email)).Ancestor(row)
		if err := ui.Exists(emailCell)(ctx); err != nil {
			return errors.Wrapf(err, "user with email %s is not logged into browser", email)
		}
		testing.ContextLog(ctx, "Verified user logged into browser")
		emailVerified = true
		break
	}

	if !emailVerified {
		return errors.Wrap(err, "account email row not found")
	}

	// Find child status row and verify that user is recognized as child.
	testing.ContextLog(ctx, "Verifying child status row")
	childStatusVerified := false
	for i := range profileNodes {
		row := profileRows.Nth(i)
		childCell := nodewith.Role(role.LayoutTableCell).Name("Child").Ancestor(row)
		if err := ui.Exists(childCell)(ctx); err != nil {
			continue
		}
		boolCell := nodewith.Role(role.LayoutTableCell).Name("true").Ancestor(row)
		if err := ui.Exists(boolCell)(ctx); err != nil {
			return errors.Wrap(err, "user not recognized as child")
		}
		testing.ContextLog(ctx, "Verified user recognized as child")
		childStatusVerified = true
		break
	}

	if !childStatusVerified {
		return errors.Wrap(err, "child status row not found")
	}

	return nil
}

// NavigateExtensionApprovalFlow runs through flow to add extension up to the point of actually adding it, but does not add to avoid interfering with future runs of the test.
func NavigateExtensionApprovalFlow(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, bt browser.Type, parentEmail, parentPassword string) error {
	testing.ContextLog(ctx, "Adding extension as a supervised user")

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Set up browser.
	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, bt)
	if err != nil {
		return errors.Wrap(err, "failed to set up browser")
	}
	defer closeBrowser(cleanupCtx)

	// Open webstore in browser.
	const extensionID = "djflhoibgkdhkhhcedjiklpkjnoahfmg" // Google-developed extension from Chrome Store.
	const extensionURL = "https://chrome.google.com/webstore/detail/" + extensionID + "?hl=en"
	conn, err := br.NewConn(ctx, extensionURL, browser.WithNewWindow())
	if err != nil {
		return errors.Wrap(err, "failed to open webstore")
	}
	defer conn.Close()

	// Load page contents.
	ui := uiauto.New(tconn).WithTimeout(time.Minute)

	// Install extension parent permission flow.
	testing.ContextLog(ctx, "Finding button that adds the extension")
	addButton := nodewith.Name("Add to Chrome").Role(role.Button).First()
	if err := ui.WaitUntilExists(addButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to load page")
	}

	testing.ContextLog(ctx, "Clicking button that adds the extension")
	if err := ui.LeftClick(addButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to click add extension")
	}

	testing.ContextLog(ctx, "Clicking ask parent")
	askParentButton := nodewith.Name("Ask a parent").Role(role.Button)
	// The "Ask parent" button may not immediately be clickable.
	if err := ui.LeftClickUntil(askParentButton, ui.Gone(askParentButton))(ctx); err != nil {
		return errors.Wrap(err, "failed to click ask parent")
	}

	testing.ContextLog(ctx, "Selecting parent email"+strings.ToLower(parentEmail))
	parentEmailRadio := nodewith.Name(strings.ToLower(parentEmail)).Role(role.RadioButton)
	parentEmailText := nodewith.Name(strings.ToLower(parentEmail))
	// If there are two parents, the dialog contains a radio button with both parent emails.
	if err := ui.LeftClick(parentEmailRadio)(ctx); err != nil {
		// If there is no radio button, this indicates that there is only one parent. Verify
		// that the email is present as text, and return an error if it is not present.
		if err := ui.Exists(parentEmailText)(ctx); err != nil {
			return errors.Wrapf(err, "failed to find parent email %q", parentEmail)
		}
	}

	testing.ContextLog(ctx, "Clicking the parent password text field")
	parentPasswordField := nodewith.Name("Enter password").Role(role.TextField)
	if err := ui.LeftClick(parentPasswordField)(ctx); err != nil {
		return errors.Wrap(err, "failed to click parent password text")
	}

	testing.ContextLog(ctx, "Setting up keyboard")
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard")
	}
	defer kb.Close()

	testing.ContextLog(ctx, "Typing the parent password")
	if err := kb.Type(ctx, parentPassword); err != nil {
		return errors.Wrap(err, "failed to type parent password")
	}

	testing.ContextLog(ctx, "Verifying Approve and Cancel buttons enabled")
	approveButton := nodewith.Name("Approve").Role(role.Button)
	if err := ui.CheckRestriction(approveButton, restriction.None)(ctx); err != nil {
		return errors.Wrap(err, "failed to verify Approve button enabled")
	}
	cancelButton := nodewith.Name("Cancel").Role(role.Button)
	if err := ui.CheckRestriction(cancelButton, restriction.None)(ctx); err != nil {
		return errors.Wrap(err, "failed to verify Cancel button enabled")
	}

	return nil
}
