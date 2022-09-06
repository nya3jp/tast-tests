// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package familylink is used for writing Family Link tests.
package familylink

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DeniedSitesBlocked,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks that parent-blocked sites are blocked for Unicorn users",
		Contacts:     []string{"danan@chromium.org", "cros-families-eng+test@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Vars:         []string{"unicorn.blockedSite"},
		Fixture:      "familyLinkUnicornLogin",
	})
}

func DeniedSitesBlocked(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*familylink.FixtData).TestConn
	cr := s.FixtValue().(*familylink.FixtData).Chrome

	blockedSite := s.RequiredVar("unicorn.blockedSite")

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)

	if err := familylink.WaitForSyncToComplete(ctx, tconn, browser.TypeAsh, 4*time.Minute); err != nil {
		s.Fatal("Failed to wait for sync: ", err)
	}

	// Navigate to the blocked site.
	conn, err := cr.NewConn(ctx, blockedSite)
	if err != nil {
		s.Fatal("Failed to open browser to website: ", err)
	}
	defer conn.Close()

	// Check that the blocked site interstitial appears.
	if err := ui.WaitUntilExists(nodewith.Name("Ask your parent").Role(role.StaticText))(ctx); err != nil {
		s.Fatal("Failed to detect blocked site interstitial: ", err)
	}
}
