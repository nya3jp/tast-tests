// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package familylink is used for writing Family Link tests.
package familylink

import (
	"context"
	"time"

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
		Desc:         "Checks that matures sites are blocked for Unicorn users",
		Contacts:     []string{"tobyhuang@chromium.org", "cros-families-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Vars:         []string{"unicorn.matureSite"},
		Fixture:      "familyLinkUnicornLogin",
	})
}

func MatureSitesBlocked(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*familylink.FixtData).TestConn
	cr := s.FixtValue().(*familylink.FixtData).Chrome

	matureSite := s.RequiredVar("unicorn.matureSite")

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	conn, err := cr.NewConn(ctx, matureSite)
	if err != nil {
		s.Fatal("Failed to navigate to website: ", err)
	}
	defer conn.Close()

	ui := uiauto.New(tconn)
	if err := ui.WaitUntilExists(nodewith.Name("Site blocked").Role(role.StaticText))(ctx); err != nil {
		s.Fatal("Mature website is not blocked for Unicorn user: ", err)
	}
}
