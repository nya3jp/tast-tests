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
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BrowserWithNewChrome,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests SetUpWithNewChrome in the browserfixt package. See http://go/lacros-tast-porting for the guidelines on how to use",
		Contacts:     []string{"hyungtaekim@chromium.org", "lacros-team@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Timeout:      4 * time.Minute,
	})
}

func BrowserWithNewChrome(ctx context.Context, s *testing.State) {
	// Reserve some time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	s.Log("SetUpWithNewChrome starts")
	for _, param := range []struct {
		bt  browser.Type
		cfg *lacrosfixt.Config
	}{
		{browser.TypeAsh, nil},
		{browser.TypeAsh, lacrosfixt.NewConfig()},    // LacrosConfig is a no-op for ash-chrome.
		{browser.TypeLacros, lacrosfixt.NewConfig()}, // default config
		{browser.TypeLacros, lacrosfixt.NewConfig(
			lacrosfixt.Selection(lacros.Rootfs), lacrosfixt.Mode(lacros.LacrosOnly))}, // custom config
	} {
		bt := param.bt
		cfg := param.cfg
		s.Run(ctx, fmt.Sprintf("SetUpWithNewChrome browser: %v, cfg: %+v", bt, cfg), func(ctx context.Context, s *testing.State) {
			// Connect to a fresh ash-chrome instance (cr) and set a browser instance (br) to use browser functionality.
			cr, br, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, bt, cfg)
			if err != nil {
				s.Fatalf("Failed to connect to %v browser: %v", bt, err)
			}
			defer cr.Close(cleanupCtx)
			defer closeBrowser(cleanupCtx)

			numNewWindows := 0
			if bt == browser.TypeLacros {
				numNewWindows = 1 // Lacros opens an extra window in browserfixt.SetUp*.
			}

			// Open a new window.
			const url = chrome.NewTabURL
			conn, err := br.NewConn(ctx, url, browser.WithNewWindow())
			if err != nil {
				s.Fatalf("Failed to open new window with url: %v, %v", url, err)
			}
			defer conn.Close()
			numNewWindows++

			// Verify that the expected number of browser windows are open.
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create test API connection: ", err)
			}
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				ws, err := ash.FindAllWindows(ctx, tconn, func(w *ash.Window) bool {
					return (bt == browser.TypeAsh && w.WindowType == ash.WindowTypeBrowser) ||
						(bt == browser.TypeLacros && w.WindowType == ash.WindowTypeLacros)
				})
				if err != nil {
					return errors.Wrap(err, "failed to get all browser windows")
				}
				if len(ws) != numNewWindows {
					return errors.Wrapf(err, "failed to find open browser windows. expected: %v, got: %v", numNewWindows, len(ws))
				}
				return nil
			}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second}); err != nil {
				s.Fatalf("Failed to find %v browser windows: %v", bt, err)
			}
		})
	}

	s.Log("SetUpWithNewChromeAtURL starts")
	for _, param := range []struct {
		bt  browser.Type
		cfg *lacrosfixt.Config
		url string
	}{
		{browser.TypeAsh, lacrosfixt.NewConfig(), chrome.BlankURL}, // LacrosConfig is a no-op for ash-chrome.
		{browser.TypeLacros, lacrosfixt.NewConfig(), chrome.BlankURL},
	} {
		bt := param.bt
		cfg := param.cfg
		url := param.url
		s.Run(ctx, fmt.Sprintf("SetUpWithNewChromeAtURL browser: %v, cfg: %+v", bt, cfg), func(ctx context.Context, s *testing.State) {
			// Set up the browser, open a first browser window with a given URL.
			conn, cr, br, closeBrowser, err := browserfixt.SetUpWithNewChromeAtURL(ctx, bt, url, cfg)
			if err != nil {
				s.Fatalf("Failed to connect to %v browser: %v", bt, err)
			}
			defer cr.Close(cleanupCtx)
			defer closeBrowser(cleanupCtx)
			defer conn.Close()

			// Open the version page in another new window.
			conn, err = br.NewConn(ctx, chrome.VersionURL, browser.WithNewWindow())
			if err != nil {
				s.Fatal("Failed to open new window with version page: ", err)
			}
			defer conn.Close()

			// Verify that there are two windows open of the given browser type.
			const numWindows = 2
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create test API connection: ", err)
			}
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				ws, err := ash.FindAllWindows(ctx, tconn, ash.BrowserTypeMatch(bt))
				if err != nil {
					return errors.Wrap(err, "failed to get all browser windows")
				}
				if len(ws) != numWindows {
					return errors.Wrapf(err, "failed to find open browser windows. expected: %v, got: %v", numWindows, len(ws))
				}
				return nil
			}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second}); err != nil {
				s.Fatalf("Failed to find %v browser windows: %v", bt, err)
			}
		})
	}
}
