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
		Contacts:     []string{"tobyhuang@chromium.org", "cros-families-eng@google.com"},
		Attr:         []string{"group:mainline"}, // Temporary change to test in CQ.
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
