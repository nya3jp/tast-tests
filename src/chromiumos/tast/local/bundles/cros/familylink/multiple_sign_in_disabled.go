// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package familylink

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/ui/quicksettings"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MultipleSignInDisabled,
		Desc: "Verifies that multiple sign-in is disabled for Unicorn users. Geller users should behave similarly",
		Contacts: []string{
			"tobyhuang@chromium.org", "cros-families-eng+test@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		VarDeps:      []string{"unicorn.childFirstName", "unicorn.childLastName"},
		Fixture:      "familyLinkUnicornLoginNonOwner",
	})
}

func MultipleSignInDisabled(ctx context.Context, s *testing.State) {
	// After the fixture runs, the DUT has two users on the
	// device: a regular owner and a Unicorn secondary user. The
	// Unicorn user is logged in.
	tconn := s.FixtValue().(*familylink.FixtData).TestConn

	childFirstName := s.RequiredVar("unicorn.childFirstName")
	childLastName := s.RequiredVar("unicorn.childLastName")

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	ui := uiauto.New(tconn)

	s.Log("Opening the system status tray")
	if err := quicksettings.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to open Quick Settings: ", err)
	}
	defer quicksettings.Hide(ctx, tconn)

	s.Log("Attempting to add multiple profiles")
	userProfileName := fmt.Sprintf("%s%s", strings.ToLower(childFirstName), strings.ToLower(childLastName))
	s.Logf("Looking for user profile name %q", userProfileName)
	userProfileIcon := nodewith.NameContaining(userProfileName).Role(role.Button)
	if err := ui.WaitUntilExists(userProfileIcon)(ctx); err != nil {
		s.Fatal("Failed to find the user profile icon: ", err)
	}

	// Family Link users are never allowed to use multi-user sign-
	// in, so the the profile icon button should be disabled.
	if err := ui.WaitUntilExists(userProfileIcon.Focusable())(ctx); err == nil {
		s.Fatal("User profile button should be disabled for Family Link users: ", err)
	}
}
