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
		Desc: "Verifies that multiple sign-in is disabled for Unicorn users",
		Contacts: []string{
			"tobyhuang@chromium.org", "cros-families-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		VarDeps:      []string{"unicorn.childFirstName", "unicorn.childLastName"},
		Fixture:      "familyLinkUnicornLoginNonOwner",
	})
}

func MultipleSignInDisabled(ctx context.Context, s *testing.State) {
	childFirstName := s.RequiredVar("unicorn.childFirstName")
	childLastName := s.RequiredVar("unicorn.childLastName")

	tconn := s.FixtValue().(*familylink.FixtData).TestConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	ui := uiauto.New(tconn)

	s.Log("Opening the system status tray")
	if err := quicksettings.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to open Quick Settings: ", err)
	}
	defer quicksettings.Hide(ctx, tconn)

	s.Log("Attempting to add multiple profiles")
	userProfileName := fmt.Sprintf("%s%s", strings.ToLower(childFirstName), strings.ToLower(childLastName))
	s.Logf("Looking for user profile name=%s", userProfileName)
	userProfileIcon := nodewith.NameContaining(userProfileName).Role(role.Button)
	if err := uiauto.Combine("Attempting to add multiple profiles",
		ui.WaitUntilExists(userProfileIcon),
		ui.LeftClick(userProfileIcon))(ctx); err != nil {
		s.Fatal("Failed to click the user profile icon: ", err)
	}

	s.Log("Checking for error message preventing Unicorn users from adding multi-profiles")
	// A regular user would see "Sign in another another user..."
	// at this point instead of the expected error message.
	if err := ui.WaitUntilExists(nodewith.Name("All available users have already been added to this session.").Role(role.StaticText))(ctx); err != nil {
		s.Fatal("Failed to detect error message preventing Unicorn users from adding multi-profiles: ", err)
	}
}
