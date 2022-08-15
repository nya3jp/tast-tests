// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package familylink is used for writing Family Link tests.
package familylink

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UnicornExtensions,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks if Unicorn user can add extension with parent permission",
		Contacts:     []string{"chromeos-sw-engprod@google.com", "cros-oac@google.com", "tobyhuang@chromium.org", "cros-families-eng+test@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      time.Minute,
		VarDeps: []string{
			"family.parentEmail",
			"family.parentPassword",
		},
		Params: []testing.Param{{
			Val:     browser.TypeAsh,
			Fixture: "familyLinkUnicornLogin",
		},
		/* Disabled due to <1% pass rate over 30 days. See b/241943380
		{
			Name:    "lacros",
			Val:     browser.TypeLacros,
			Fixture: "familyLinkUnicornLoginWithLacros",
		}
		*/
		},
	})
}

func UnicornExtensions(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*familylink.FixtData).Chrome
	tconn := s.FixtValue().(*familylink.FixtData).TestConn

	if cr == nil {
		s.Fatal("Failed to start Chrome")
	}
	if tconn == nil {
		s.Fatal("Failed to create test API connection")
	}

	if err := familylink.NavigateExtensionApprovalFlow(ctx, cr, tconn, s.Param().(browser.Type), s.RequiredVar("family.parentEmail"), s.RequiredVar("family.parentPassword")); err != nil {
		s.Fatal("Failed to add extension: ", err)
	}
}
