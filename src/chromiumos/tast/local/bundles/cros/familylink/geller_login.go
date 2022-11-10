// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package familylink is used for writing Family Link tests.
package familylink

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GellerLogin,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks if Geller login is working",
		Contacts:     []string{"chromeos-sw-engprod@google.com", "cros-oac@google.com", "tobyhuang@chromium.org", "cros-families-eng+test@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      2 * time.Minute,
		Params: []testing.Param{{
			Fixture: "familyLinkGellerLogin",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           "familyLinkGellerLoginWithLacros",
			Val:               browser.TypeLacros,
		}},
		VarDeps: []string{
			"family.gellerEmail",
		},
	})
}

func GellerLogin(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn := s.FixtValue().(familylink.HasTestConn).TestConn()

	// TODO(b/254891227): Remove this when chrome.New() doesn't have a race condition.
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to wait for Login to complete: ", err)
	}

	if cr == nil {
		s.Fatal("Failed to start Chrome")
	}
	if tconn == nil {
		s.Fatal("Failed to create test API connection")
	}
	if err := familylink.VerifyUserSignedIntoBrowserAsChild(ctx, cr, tconn, s.Param().(browser.Type), s.RequiredVar("family.gellerEmail"), s.OutDir()); err != nil {
		s.Fatal("Failed to verify user signed into browser: ", err)
	}
}
