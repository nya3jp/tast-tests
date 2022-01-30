// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/browser/browserutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Browser,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests the Browser Tast library. See http://go/lacros-tast-porting for the guidelines on how to use",
		Contacts:     []string{"lacros-tast@google.com"},
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
	br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), bt)
	if err != nil {
		s.Fatalf("Failed to set up a browser: %v, err: %v", bt, err)
	}
	defer closeBrowser(ctx)

	// Open a few more blank pages.
	const numNewWindows = 3
	for i := 0; i < numNewWindows; i++ {
		_, err := br.NewConn(ctx, chrome.BlankURL, browser.WithNewWindow())
		if err != nil {
			s.Fatalf("Failed to open a window, browser: %v, err: %v", bt, err)
		}
		testing.Sleep(ctx, time.Second)
	}

	// Close all the windows opened above by br.NewConn.
	for i := 0; i < numNewWindows; i++ {
		err := browserutil.CloseAboutBlank(ctx, br, s.Param().(browser.Type))
		if err != nil {
			s.Fatalf("Failed to close about:blank, browser: %v, err: %v", bt, err)
		}
	}

	// Ensure there is no window left open for ash-chrome, and still one open for lacros-chrome.
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the test API connection: ", err)
	}
	windows, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get all windows: ", err)
	}
	if (bt == browser.TypeLacros && len(windows) > 1) || (bt == browser.TypeAsh && len(windows) > 0) {
		s.Fatalf("Failed to close all open windows, browser: %v, err: %v", bt, err)
	}
}
