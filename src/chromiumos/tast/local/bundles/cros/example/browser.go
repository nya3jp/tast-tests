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

	// TODO: Confirm browser.CreateWindow works with any URLs. Yes, Okay with any URLs.
	const url = "chrome://newtab" // chrome.BlankURL
	// const url = chrome.BlankURL

	// Set a browser instance from the existing chrome session.
	// It will open a new window for lacros-chrome, but not for ash-chrome.
	bt := s.Param().(browser.Type)
	// TODO: SetUpWithURL will faile for the blank URL for lacros-chrome.
	// It calls NewConnForTarget under the hood that will be timed out in waiting for chrome://newtab that shows up with an empty address.
	_, br, closeBrowser, err := browserfixt.SetUpWithURL(ctx, s.FixtValue(), bt, url)
	if err != nil {
		s.Fatalf("Failed to set up a browser: %v, err: %v", bt, err)
	}
	defer closeBrowser(ctx)

	// Open a few more blank tabs and windows.
	numNewWindows := 3
	for i := 0; i < numNewWindows; i++ {
		var opts []browser.CreateTargetOption
		// if i%2 == 0 {
		opts = append(opts, browser.WithNewWindow())
		// }
		if err := browser.CreateWindows(ctx, br, bt, url, 1, opts...); err != nil {
			s.Fatalf("Failed to open a window, browser: %v, err: %v", bt, err)
		}
		testing.Sleep(ctx, 3*time.Second)
	}

	// Ensure there is no window left open for ash-chrome, and still one open for lacros-chrome.
	// ash.GetAllWindows doesn't count a new tab page in the same window as one window.
	// TODO: we should probably use FindTargets to count tabs and windows.
	windows, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get all windows: ", err)
	}
	// Lacros would have an extra blank window opened via browserfixt.SetUp*.
	if bt == browser.TypeLacros {
		numNewWindows += 1
	}
	if len(windows) != numNewWindows {
		s.Fatalf("Failed to open the desired number of %v browser windows, expected: %v, actual: %v", bt, numNewWindows, len(windows))
	}
}
