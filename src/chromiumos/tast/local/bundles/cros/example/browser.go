// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Browser,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests the Browser Tast library. See http://go/lacros-tast-porting for the guidelines on how to use",
		Contacts:     []string{"hyungtaekim@chromium.org", "hidehiko@chromium.org", "lacros-tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
	})
}

func Browser(ctx context.Context, s *testing.State) {
	// Set a browser instance from the existing chrome session.
	// It will open a new window for lacros-chrome, but not for ash-chrome.
	bt := s.Param().(browser.Type)
	br, _ /* closeBrowser */, err := browserfixt.SetUp(ctx, s.FixtValue(), bt)
	if err != nil {
		s.Fatalf("Failed to set up a browser: %v, err: %v", bt, err)
	}
	// Clean up browser and free resources associated with it.
	// defer closeBrowser(ctx)
	defer br.Close(ctx)
	defer br.Close(ctx) // br.Close on already closed session won't raise a nil point exception directly, but let the underlying browser to handle it.

	// Open a new browser window.
	if _, err := br.NewConn(ctx, chrome.BlankURL, browser.WithNewWindow()); err != nil {
		s.Fatalf("Failed to open a window, browser: %v, err: %v", bt, err)
	}
}
