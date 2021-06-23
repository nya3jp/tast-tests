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
		Func:         NonEducoexistenceInsession,
		Desc:         "Checks that Unicorn account trying to add a non-EDU secondary account fails",
		Contacts:     []string{"tobyhuang@chromium.org", "cros-families-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      chrome.GAIALoginTimeout + 5*time.Minute,
		Vars:         []string{"unicorn.parentUser", "unicorn.parentPassword", "unicorn.parentFirstName", "unicorn.parentLastName", "geller.parentUser", "geller.parentPassword"},
		Fixture:      "familyLinkUnicornLogin",
	})
}

func NonEducoexistenceInsession(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*familylink.FixtData).TestConn
	cr := s.FixtValue().(*familylink.FixtData).Chrome

	unicornParentUser := s.RequiredVar("unicorn.parentUser")
	unicornParentPass := s.RequiredVar("unicorn.parentPassword")
	unicornParentFirstName := s.RequiredVar("unicorn.parentFirstName")
	unicornParentLastName := s.RequiredVar("unicorn.parentLastName")
	gellerParentUser := s.RequiredVar("geller.parentUser")
	gellerParentPass := s.RequiredVar("geller.parentPassword")

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)

	s.Log("Launching the in-session Edu Coexistence flow")
	// Passing geller parent credentials instead of Edu should fail.
	if err := familylink.AddEduSecondaryAccount(ctx, cr, tconn, unicornParentFirstName, unicornParentLastName, unicornParentUser, unicornParentPass, gellerParentUser, gellerParentPass); err != nil {
		s.Fatal("Failed to go through the in-session Edu Coexistence flow: ", err)
	}

	s.Log("Verifying the attempt to add a non-EDU secondary account failed")
	if err := ui.WaitUntilExists(nodewith.Name("Can’t add account").Role(role.Heading))(ctx); err != nil {
		s.Fatal("Failed to detect can't add acccount error message: ", err)
	}
}
