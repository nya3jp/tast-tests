// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"fmt"
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
			Fixture:           "lacrosPrimary",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
	})
}

func Browser(ctx context.Context, s *testing.State) {
	// Reserve some time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the test API connection: ", err)
	}

	bt := s.Param().(browser.Type)
	const numNewWindows = 2
	for _, url := range []string{
		"about:blank", // pass
		"",            // pass
		// TODO(crbug.com/1290318): Uncomment when the issue of lacros-chrome opening empty window for chrome:// URLs is fixed.
		// "chrome://newtab", // fail for lacros-chrome
		// "chrome://version", // fail for lacros-chrome
		// "http://www.google.com", // fail for lacros-chrome
	} {
		s.Run(ctx, fmt.Sprintf("browserfixt.CreateWindows browser: %v, url: %v", bt, url), func(ctx context.Context, s *testing.State) {
			// Make sure no window is open.
			if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
				if err := w.CloseWindow(ctx, tconn); err != nil {
					return errors.Wrap(err, "failed to close the window")
				}
				return nil
			}); err != nil {
				s.Fatal("Failed to close all windows: ", err)
			}

			// Open the exact number of browser windows.
			conn, _ /* br */, closeBrowser, err := browserfixt.CreateWindows(ctx, s.FixtValue(), bt, tconn, url, numNewWindows)
			if err != nil {
				s.Fatalf("Failed to set up a browser: %v, err: %v", bt, err)
			}
			defer closeBrowser(cleanupCtx)
			defer conn.Close()

			if err := testing.Poll(ctx, func(ctx context.Context) error {
				ws, err := ash.GetAllWindows(ctx, tconn)
				if err != nil {
					return errors.Wrap(err, "failed to get all windows")
				}
				if len(ws) != numNewWindows {
					return errors.Wrapf(err, "failed to find open windows expected: %v, got: %v", numNewWindows, len(ws))
				}
				return nil
			}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second}); err != nil {
				s.Fatalf("Failed to open the desired number of %v browser windows: %v", bt, err)
			}
		})
	}
}
