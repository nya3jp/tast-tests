// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package familylink is used for writing Family Link tests.
package familylink

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Gellerization,
		Desc:         "Checks if the gellerization flow is working",
		Contacts:     []string{"lienhoang@chromium.org", "cros-families-eng+test@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      chrome.GAIALoginTimeout + 2*time.Minute,
		Vars: []string{
			"familylink.Gellerization.parentUser",
			"familylink.Gellerization.parentPassword",
			"familylink.Gellerization.childUser",
			"familylink.Gellerization.childPassword",
		},
	})
}

func Gellerization(ctx context.Context, s *testing.State) {
	childUser := s.RequiredVar("familylink.Gellerization.childUser")
	childPass := s.RequiredVar("familylink.Gellerization.childPassword")
	parentUser := s.RequiredVar("familylink.Gellerization.parentUser")
	parentPass := s.RequiredVar("familylink.Gellerization.parentPassword")

	cr, err := chrome.New(
		ctx,
		chrome.GAIALogin(chrome.Creds{User: childUser, Pass: childPass}))

	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	ui := uiauto.New(tconn)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("Setting up keyboard")
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	s.Log("Launching the settings app")
	if err := apps.Launch(ctx, tconn, apps.Settings.ID); err != nil {
		s.Fatal("Failed to launch the settings app: ", err)
	}

	s.Log("Launching the gellerization flow")
	setUpButton := nodewith.Name("Parental controls").Role(role.Button)
	getStartedButton := nodewith.Name("Get started").Role(role.Button)
	if err := uiauto.Combine("Opening gellerization flow",
		ui.WaitUntilExists(setUpButton),
		ui.FocusAndWait(setUpButton), // scroll the button into view
		ui.WithInterval(200*time.Millisecond).LeftClickUntil(setUpButton, ui.Exists(getStartedButton)),
	)(ctx); err != nil {
		s.Fatal("Failed to open gellerization flow: ", err)
	}

	s.Log("Navigating to the parent authentication page")
	nextButton := nodewith.NameRegex(regexp.MustCompile("(Next|More|Yes.*)")).Role(role.Button)
	parentEmailText := nodewith.Name("Email or phone").Role(role.TextField)
	if err := uiauto.Combine("Clicking next",
		ui.WithInterval(200*time.Millisecond).LeftClickUntil(getStartedButton, ui.Exists(nextButton)),
		ui.WithInterval(200*time.Millisecond).LeftClickUntil(nextButton, ui.Exists(parentEmailText)),
	)(ctx); err != nil {
		s.Fatal("Failed to navigate to the parent auth page: ", err)
	}

	s.Log("Authenticating parent")
	if err := ui.LeftClick(parentEmailText)(ctx); err != nil {
		s.Fatal("Failed to click parent email text field: ", err)
	}
	if err := kb.Type(ctx, parentUser+"\n"); err != nil {
		s.Fatal("Failed to type parent user email: ", err)
	}
	parentPassText := nodewith.Name("Enter your password").Role(role.TextField)
	if err := uiauto.Combine("Clicking parent account password text field",
		ui.WaitUntilExists(parentPassText), ui.LeftClick(parentPassText))(ctx); err != nil {
		s.Fatal("Failed to click parent account password text field: ", err)
	}
	s.Log("Typing parent account password")
	if err := kb.Type(ctx, parentPass+"\n"); err != nil {
		s.Fatal("Failed to type edu user password: ", err)
	}

	s.Log("Clicking to the end of the consent page")
	confirmHeading := nodewith.Name("About supervision").Role(role.Heading)
	agreeButton := nodewith.Name("Agree").Role(role.Button)
	if err := uiauto.Combine("Waiting for and clicking More through consent page",
		ui.WaitUntilExists(confirmHeading),
		ui.WithInterval(200*time.Millisecond).LeftClickUntil(nextButton, ui.Exists(agreeButton)),
	)(ctx); err != nil {
		s.Fatal("Failed to get to the end of consent page: ", err)
	}

	s.Log("Clicking child password text field")
	childPassText := nodewith.NameContaining("enter your password").Role(role.TextField)
	if err := ui.LeftClick(childPassText)(ctx); err != nil {
		s.Fatal("Failed to click child password text field: ", err)
	}
}
