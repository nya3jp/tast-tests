// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GuestModeExtension,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Check Tast extension can be loaded in Guest mode",
		Contacts:     []string{"benreich@chromium.org", "chromeos-engprod-syd@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Fixture: "chromeLoggedInGuest",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			Fixture:           "lacrosGuest",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
	})
}

func GuestModeExtension(ctx context.Context, s *testing.State) {
	br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to set up browser: ", err)
	}
	defer closeBrowser(ctx)

	// Attempt to open the Test API connection.
	if _, err := br.TestAPIConn(ctx); err != nil {
		s.Fatal("Failed to create Test API Connection: ", err)
	}
}
