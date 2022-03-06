// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BrowserFixture,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests the Browser Tast library with lacros fixture. See http://go/lacros-tast-porting for the guidelines on how to use",
		Contacts:     []string{"lacros-tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			Fixture:           "lacrosPrimary",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
		// Vars: []string{browserfixt.LacrosDeployedBinary},
	})
}

func BrowserFixture(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	bt := s.Param().(browser.Type)
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the test API connection: ", err)
	}

	// Set up a browser.
	br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), bt)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	// Open a few more blank windows.
	var numNewWindows = 2
	for i := 0; i < numNewWindows; i++ {
		if _, err := br.NewConn(ctx, chrome.BlankURL, browser.WithNewWindow()); err != nil {
			s.Fatalf("Failed to open a window, browser: %v, err: %v", bt, err)
		}
	}
	if bt == browser.TypeLacros {
		numNewWindows++ // Lacros should open one extra window when instantiated.
	}

	// Verify that the correct number of browser windows are open.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		ws, err := ash.FindAllWindows(ctx, tconn, func(w *ash.Window) bool {
			if bt == browser.TypeAsh {
				return w.IsVisible && w.WindowType == ash.WindowTypeBrowser
			}
			if bt == browser.TypeLacros {
				return w.IsVisible && w.WindowType == ash.WindowTypeLacros
			}
			return false
		})
		if err != nil {
			testing.PollBreak(errors.Wrap(err, "failed to find all browser windows"))
		}
		if len(ws) != numNewWindows {
			return errors.Errorf("expected %v windows, got %v", numNewWindows, len(ws))
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
		s.Fatal("Timed out waiting for browser windows to become visible")
	}
}
