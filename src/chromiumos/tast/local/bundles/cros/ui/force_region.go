// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ForceRegion,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that region is forced in Chrome tests",
		Contacts:     []string{"nya@chromium.org", "chromeos-ui@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Val: browser.TypeAsh,
		}, {
			Name:              "lacros",
			Val:               browser.TypeLacros,
			ExtraSoftwareDeps: []string{"lacros"},
			ExtraAttr:         []string{"informational"},
		}},
		Vars: []string{browserfixt.LacrosDeployedBinary},
	})
}

func ForceRegion(ctx context.Context, s *testing.State) {
	const (
		region   = "jp"
		wantLang = "ja"
	)

	// Connect to a fresh ash-chrome instance (cr) and get a browser instance (br) for browser functionality.
	bt := s.Param().(browser.Type)
	cr, br, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, bt, lacrosfixt.NewConfigFromState(s),
		chrome.Region(region))
	if err != nil {
		s.Fatalf("Chrome login failed with %v browser: %v", bt, err)
	}
	defer cr.Close(ctx)
	defer closeBrowser(ctx)

	// Get a TestConn to active browser.
	bTconn, err := br.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	var lang string
	if err := bTconn.Eval(ctx, "chrome.i18n.getUILanguage()", &lang); err != nil {
		s.Fatal("Failed to call chrome.i18n.getUILanguage: ", err)
	} else if lang != wantLang {
		s.Fatalf("UI language is %s; want %s", lang, wantLang)
	}
}
