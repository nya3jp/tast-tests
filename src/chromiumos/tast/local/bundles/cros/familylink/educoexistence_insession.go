// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package familylink is used for writing Family Link tests.
package familylink

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EducoexistenceInsession,
		Desc:         "Checks if in-session EDU Coexistence flow is working",
		Contacts:     []string{"tobyhuang@chromium.org", "cros-families-eng+test@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      chrome.GAIALoginTimeout + 5*time.Minute,
		VarDeps:      []string{"unicorn.parentUser", "unicorn.parentPassword", "unicorn.parentFirstName", "unicorn.parentLastName", "edu.user", "edu.password"},
		Fixture:      "familyLinkUnicornLogin",
	})
}

func EducoexistenceInsession(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*familylink.FixtData).TestConn
	cr := s.FixtValue().(*familylink.FixtData).Chrome

	parentUser := s.RequiredVar("unicorn.parentUser")
	parentPass := s.RequiredVar("unicorn.parentPassword")
	parentFirstName := s.RequiredVar("unicorn.parentFirstName")
	parentLastName := s.RequiredVar("unicorn.parentLastName")
	eduUser := s.RequiredVar("edu.user")
	eduPass := s.RequiredVar("edu.password")

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// TODO(b/190680218): Fixture reset should close all windows.
	// We shouldn't need this. Remove once fixed.
	defer func() {
		s.Log("Cleaning up dialogs")
		if !s.HasError() {
			s.Log("No error, nothing to clean up")
			return
		}
		s.Log("Closing system modal dialog for next test")
		ui := uiauto.New(tconn)
		// There are two close buttons, one on the settings page
		// parent window and one on the Edu Coexistence flow system
		// modal dialog. We want the second button to close the system
		// modal dialog. The fixture will take care of closing other
		// windows when it resets.
		closeButton := nodewith.Name("Close").Role(role.Button).Nth(1)
		if err := ui.IfSuccessThen(ui.WithTimeout(10*time.Second).WaitUntilExists(closeButton), ui.LeftClick(closeButton))(ctx); err != nil {
			s.Fatal("Failed to click close button: ", err)
		}
	}()

	ui := uiauto.New(tconn)

	s.Log("Launching the in-session Edu Coexistence flow")
	if err := familylink.AddEduSecondaryAccount(ctx, cr, tconn, parentFirstName, parentLastName, parentUser, parentPass, eduUser, eduPass); err != nil {
		s.Fatal("Failed to go through the in-session Edu Coexistence flow: ", err)
	}

	s.Log("Clicking next on the final page to wrap up")
	schoolAccountAddedHeader := nodewith.Name("School account added").Role(role.Heading)
	if err := uiauto.Combine("Clicking next button and wrapping up",
		ui.WaitUntilExists(schoolAccountAddedHeader),
		ui.LeftClickUntil(nodewith.Name("Next").Role(role.Button), ui.Gone(schoolAccountAddedHeader)))(ctx); err != nil {
		s.Fatal("Failed to click next button: ", err)
	}

	s.Log("Verifying the EDU secondary account added successfully")
	// There should be a "more actions" button to remove the EDU secondary account.
	moreActionsButton := nodewith.Name("More actions, " + eduUser).Role(role.Button)
	if err := ui.WaitUntilExists(moreActionsButton)(ctx); err != nil {
		s.Fatal("Failed to detect EDU secondary account: ", err)
	}
}
