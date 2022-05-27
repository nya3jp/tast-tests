// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package familylink is used for writing Family Link tests.
package familylink

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MatureSitesBlocked,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that matures sites are blocked for Unicorn users",
		Contacts:     []string{"tobyhuang@chromium.org", "cros-families-eng+test@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Vars:         []string{"unicorn.matureSite"},
		Params: []testing.Param{{
			Val:     browser.TypeAsh,
			Fixture: "familyLinkUnicornLogin",
		}, {
			Name:    "lacros",
			Val:     browser.TypeLacros,
			Fixture: "familyLinkUnicornLoginWithLacros",
		}},
	})
}

func MatureSitesBlocked(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*familylink.FixtData).TestConn
	cr := s.FixtValue().(*familylink.FixtData).Chrome
	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to set up browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	matureSite := s.RequiredVar("unicorn.matureSite")
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	conn, err := br.NewConn(ctx, matureSite, browser.WithNewWindow())
	if err != nil {
		s.Fatal("Failed to navigate to website: ", err)
	}
	defer conn.Close()
	ui := uiauto.New(tconn)
	if err := ui.WaitUntilExists(nodewith.Name("Site blocked").Role(role.StaticText))(ctx); err != nil {
		s.Fatal("Mature website is not blocked for Unicorn user: ", err)
	}
}
