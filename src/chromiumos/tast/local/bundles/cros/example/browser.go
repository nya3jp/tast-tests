// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros"
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
	// Set an ash-chrome instance and a tconn to Test API extension.
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the test API connection: ", err)
	}

	// Set a browser instance from the existing chrome session.
	// It will open a new window for lacros-chrome, but not for ash-chrome.
	bt := s.Param().(browser.Type)
	br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), bt)
	if err != nil {
		s.Fatalf("Failed to set up a browser: %v, err: %v", bt, err)
	}
	defer closeBrowser(ctx)

	// Open a few more blank tabs and windows.
	const numNewWindows = 3
	for i := 0; i < numNewWindows; i++ {
		var opts []browser.CreateTargetOption
		if i%2 == 0 {
			opts = append(opts, browser.WithNewWindow())
		}
		if _, err := br.NewConn(ctx, chrome.BlankURL, opts...); err != nil {
			s.Fatalf("Failed to open a window, browser: %v, err: %v", bt, err)
		}
		// TODO: Remove the boilerplate below that gives a demo on different ways to check the browser window is visible for each browser.
		// This could be simplified with a new browser util in a follow up.
		switch bt {
		case browser.TypeAsh:
			// Check if the app is visible by querying the app ID.
			var app apps.App
			app, err = apps.ChromeOrChromium(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to find Chrome or Chromium app info: ", err)
			}
			if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
				s.Fatalf("Failed to wait for %v browser window to be visible", bt)
			}
		case browser.TypeLacros:
			// Check if the app is visible by querying the app ID, the window name and title as well.
			if err := lacros.WaitForLacrosWindow(ctx, tconn, ""); err != nil {
				s.Fatalf("Failed to wait for %v browser window to be visible", bt)
			}
		default:
			s.Fatal("Unknown browser type", bt)
		}
	}

	// Close all the tabs and windows opened above.
	for i := 0; i < numNewWindows; i++ {
		err := browserfixt.CloseWindow(ctx, br, bt, chrome.BlankURL)
		if err != nil {
			s.Fatalf("Failed to close about:blank, browser: %v, err: %v", bt, err)
		}
	}

	// Ensure there is no window left open for ash-chrome, and still one open for lacros-chrome.
	windows, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get all windows: ", err)
	}
	if (bt == browser.TypeLacros && len(windows) > 1) || (bt == browser.TypeAsh && len(windows) > 0) {
		s.Fatalf("Failed to close all open windows, browser: %v, err: %v", bt, err)
	}
}
