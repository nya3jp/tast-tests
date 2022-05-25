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
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks if Unicorn user can add extension with parent permission",
		Contacts:     []string{"chromeos-sw-engprod@google.com", "cros-oac@google.com", "tobyhuang@chromium.org", "cros-families-eng+test@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      time.Minute,
		VarDeps: []string{
			"family.parentEmail",
			"family.parentPassword",
		},
		Fixture: "familyLinkUnicornLogin",
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

	// TODO(https://crbug.com/1313067) set browser type to be Ash or LaCrOS based on param.
	if err := familylink.NavigateExtensionApprovalFlow(ctx, cr, tconn, browser.TypeAsh, s.RequiredVar("family.parentEmail"), s.RequiredVar("family.parentPassword")); err != nil {
		s.Fatal("Failed to add extension: ", err)
	}
}
