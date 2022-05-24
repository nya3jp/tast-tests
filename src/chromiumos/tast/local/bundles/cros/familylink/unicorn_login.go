// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
		Func:         UnicornLogin,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks if Unicorn login is working",
		Contacts:     []string{"chromeos-sw-engprod@google.com", "cros-oac@google.com", "tobyhuang@chromium.org", "cros-families-eng+test@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      time.Minute,
		VarDeps: []string{
			"unicorn.parentUser",
			"unicorn.parentPassword",
			"unicorn.childUser",
			"unicorn.childPassword",
		},
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

func UnicornLogin(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*familylink.FixtData).Chrome
	tconn := s.FixtValue().(*familylink.FixtData).TestConn

	if cr == nil {
		s.Fatal("Failed to start Chrome")
	}
	if tconn == nil {
		s.Fatal("Failed to create test API connection")
	}
	if err := familylink.VerifyUserSignedIntoBrowserAsChild(ctx, cr, tconn, s.Param().(browser.Type), s.RequiredVar("unicorn.childUser")); err != nil {
		s.Fatal("Failed to verify user signed into browser: ", err)
	}
}
