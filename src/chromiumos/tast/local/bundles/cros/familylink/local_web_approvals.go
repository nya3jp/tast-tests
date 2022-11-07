// Copyright 2022 The ChromiumOS Authors
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
		Func:         LocalWebApprovals,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks that parent can approve blocked sites locally",
		Contacts:     []string{"courtneywong@chromium.org", "cros-families-eng+test@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Vars:         []string{"unicorn.matureSite", "family.parentEmail", "family.parentPassword"},
		Fixture:      "familyLinkUnicornLoginWithWebApprovals",
	})
}

func LocalWebApprovals(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(familylink.HasTestConn).TestConn()
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// TODO(b/254891227): Remove this when chrome.New() doesn't have a race condition.
	testing.Sleep(ctx, 5*time.Second)

	matureSite := s.RequiredVar("unicorn.matureSite")
	conn, err := cr.NewConn(ctx, matureSite)

	if err != nil {
		s.Fatal("Failed to navigate to website: ", err)
	}
	defer conn.Close()

	ui := uiauto.New(tconn).WithTimeout(20 * time.Second)

	if err := ui.WaitUntilExists(nodewith.Name("This site is blocked").Role(role.StaticText))(ctx); err != nil {
		s.Fatal("Mature website is not blocked for Unicorn user: ", err)
	}

	askInPerson := nodewith.Name("Ask in person").Role(role.Button)
	if err := ui.WaitUntilExists(askInPerson)(ctx); err != nil {
		s.Fatal("Failed to load block interstitial: ", err)
	}

	testing.ContextLog(ctx, "Clicking ask in person")
	parentAccess := nodewith.Name("Parent access").Role(role.RootWebArea)
	if err := ui.LeftClickUntil(askInPerson, ui.Exists(parentAccess))(ctx); err != nil {
		s.Fatal("Failed to load parent access widget: ", err)
	}

	parentEmail := s.RequiredVar("family.parentEmail")
	parentPassword := s.RequiredVar("family.parentPassword")
	if err := familylink.NavigateParentAccessDialog(ctx, tconn, parentEmail, parentPassword); err != nil {
		s.Fatal("Failed to navigate parent access widget: ", err)
	}

	// Only test the deny scenario, as clicking approve would change the state of the blocked sites list for the account.
	denyButton := nodewith.Name("Deny").Role(role.Button)
	if err := ui.WaitUntilExists(denyButton)(ctx); err != nil {
		s.Fatal("Failed to render Deny button: ", err)
	}
	if err := ui.Exists(nodewith.NameContaining(matureSite).Role(role.StaticText))(ctx); err != nil {
		s.Fatal("Blocked website URL is not shown: ", err)
	}

	testing.ContextLog(ctx, "Clicking deny")
	if err := ui.LeftClick(denyButton)(ctx); err != nil {
		s.Fatal("Failed to click deny button: ", err)
	}
	if err := ui.Gone(parentAccess)(ctx); err != nil {
		s.Fatal("Parent access dialog is still open: ", err)
	}
}
