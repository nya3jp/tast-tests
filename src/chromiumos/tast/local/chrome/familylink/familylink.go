// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package familylink is used for writing Family Link tests.
package familylink

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// AddEduSecondaryAccount opens the Edu Coexistence in-session flow
// and attempts to add a secondary account for a Family Link (FL)
// primary user. FL users can only have EDU secondary accounts. Trying
// to add other account types will fail.
// Precondition: The current logged in user must be FL (such as Geller
// or Unicorn).
func AddEduSecondaryAccount(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn,
	parentFirstName, parentLastName, parentUser, parentPass,
	secondUser, secondPass string) error {

	ui := uiauto.New(tconn).WithTimeout(20 * time.Second)

	testing.ContextLog(ctx, "Checking logged in user is Family Link")
	if err := ui.Exists(nodewith.Name("This account is managed by Family Link").Role(role.Image))(ctx); err != nil {
		return errors.Wrap(err, "logged in user is not Family Link")
	}

	testing.ContextLog(ctx, "Launching the settings app")
	googleAccountsButton := nodewith.Name("Google Accounts").Role(role.Button)
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "people", ui.Exists(googleAccountsButton)); err != nil {
		return errors.Wrap(err, "failed to launch people settings page")
	}

	testing.ContextLog(ctx, "Opening the in-session EDU Coexistence flow")
	addSchoolAccountButton := nodewith.Name("Add school account").Role(role.Button)
	selectParentOption := nodewith.NameStartingWith(parentFirstName + " " + parentLastName).Role(role.ListBoxOption)
	if err := uiauto.Combine("open in-session edu coexistence flow",
		ui.WaitUntilExists(googleAccountsButton),
		ui.FocusAndWait(googleAccountsButton), // scroll the button into view
		ui.WithInterval(time.Second).LeftClickUntil(googleAccountsButton, ui.Exists(addSchoolAccountButton)),
		ui.WithInterval(time.Second).LeftClickUntil(addSchoolAccountButton, ui.Exists(selectParentOption)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to open in-session edu coexistence flow")
	}

	testing.ContextLog(ctx, "Clicking button that matches parent email: ", parentUser)
	if err := ui.WithInterval(time.Second).LeftClickUntil(selectParentOption, ui.Exists(nodewith.Name("Parent password").Role(role.TextField)))(ctx); err != nil {
		return errors.Wrap(err, "failed to click button that matches parent email")
	}

	if err := NavigateEduCoexistenceFlow(ctx, cr, tconn, parentPass, secondUser, secondPass); err != nil {
		return errors.Wrap(err, "failed to navigate in-session Edu Coexistence flow")
	}

	return nil
}

// NavigateEduCoexistenceFlow goes through the Edu Coexistence
// in-session flow and attempts to add a secondary account for a
// Family Link (FL) primary user. FL users can only have EDU secondary
// accounts. Trying to add other account types will fail.
// Precondition: The current logged in user must be FL (such as Geller
// or Unicorn).
func NavigateEduCoexistenceFlow(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn,
	parentPass, secondUser, secondPass string) error {
	ui := uiauto.New(tconn)

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
// setting. It's value needs to be overridden for test.
func CreateUsageTimeLimitPolicy() *policy.UsageTimeLimit {
	now := time.Now()
	dailyTimeUsageLimit := policy.RefTimeUsageLimitEntry{
		LastUpdatedMillis: strconv.FormatInt(now.Unix() /*base=*/, 10),
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
