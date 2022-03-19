// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ForceRegion,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks that region is forced in Chrome tests",
		Contacts:     []string{"nya@chromium.org", "chromeos-ui@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Val: browser.TypeAsh,
		}, {
			Name: "lacros",
			Val:  browser.TypeLacros,
		}},
		Vars: []string{browserfixt.LacrosDeployedBinary},
	})
}

func ForceRegion(ctx context.Context, s *testing.State) {
	const (
		region   = "jp"
		wantLang = "ja"
	)

	// Reserve some time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Connect to a fresh ash-chrome instance (cr) and get a browser instance (br) for browser functionality.
	bt := s.Param().(browser.Type)
	cr, br, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, bt, browserfixt.DefaultLacrosConfig.WithVar(s),
		chrome.Region(region))
	if err != nil {
		s.Fatalf("Chrome login failed with %v browser: %v", bt, err)
	}
	defer cr.Close(cleanupCtx)
	defer closeBrowser(cleanupCtx)

	// Get a TestConn to active browser.
	btconn, err := br.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	var lang string
	if err := btconn.Eval(ctx, "chrome.i18n.getUILanguage()", &lang); err != nil {
		s.Fatal("Failed to call chrome.i18n.getUILanguage: ", err)
	} else if lang != wantLang {
		s.Fatalf("UI language is %s; want %s", lang, wantLang)
	}
}
