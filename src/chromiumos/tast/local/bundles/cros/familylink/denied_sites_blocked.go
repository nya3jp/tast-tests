// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package familylink is used for writing Family Link tests.
package familylink

import (
	"context"
	"time"

	"chromiumos/tast/errors"
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
	tconn := s.FixtValue().(familylink.HasTestConn).TestConn()
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	blockedSite := s.RequiredVar("unicorn.blockedSite")

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)

	// The allow/block list can take a while to sync so loop checking
	// for the website to be blocked.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		conn, err := cr.NewConn(ctx, blockedSite)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to open browser to website"))
		}
		defer conn.Close()

		if err := ui.WaitUntilExists(nodewith.Name("Ask your parent").Role(role.StaticText))(ctx); err != nil {
			return errors.Wrap(err, "failed to detect blocked site interstitial")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Minute}); err != nil {
		s.Fatal("Parent-blocked website is not blocked for Unicorn user: ", err)
	}
}
