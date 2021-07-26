// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package familylink is used for writing Family Link tests.
package familylink

import (
	"context"
	"time"

	"chromiumos/tast/errors"
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
		Desc:         "Checks that parent-blocked sites are blocked for Unicorn users",
		Contacts:     []string{"danan@chromium.org", "cros-families-eng+test@google.com"},
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

	// The allow/block list can take a while to sync so infinitely loop checking
	// for the website to be blocked.  If it doesn't happen within the test timeout
	// period, this will fail via the timeout.
	success := false
	finalError := errors.New("foo")

	defer func() {
		if !success {
			s.Fatal("Parent-blocked website is not blocked for Unicorn user: ", finalError)
		}
	}()

	for attempts := 1; ; attempts++ {
		conn, err := cr.NewConn(ctx, blockedSite)
		if err != nil {
			s.Fatal("Failed to open browser to website: ", err)
		}
		defer conn.Close()

		askYourParentNode := nodewith.Name("Ask your parent").Role(role.StaticText)
		if finalError = errors.Wrap(ui.WaitUntilExists(askYourParentNode)(ctx), "failed to find Ask your parent interstitial"); err != nil {
			s.Logf("%d attempts to detect blocked site interstitial failed", attempts)
			continue
		}
		success = true
		break
	}
}
