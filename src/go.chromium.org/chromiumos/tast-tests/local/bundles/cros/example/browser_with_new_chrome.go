// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"fmt"
	"time"

	"go.chromium.org/chromiumos/tast/ctxutil"
	"go.chromium.org/chromiumos/tast/errors"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/ash"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/browser"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/browser/browserfixt"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/lacros"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/lacros/lacrosfixt"
	"go.chromium.org/chromiumos/tast/testing"
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

	for _, param := range []struct {
		bt  browser.Type
		cfg *lacrosfixt.Config
	}{
		{browser.TypeAsh, nil},
		{browser.TypeAsh, lacrosfixt.NewConfig()},    // LacrosConfig is a no-op for ash-chrome.
		{browser.TypeLacros, lacrosfixt.NewConfig()}, // default config
		{browser.TypeLacros, lacrosfixt.NewConfig(
			lacrosfixt.Selection(lacros.Rootfs), lacrosfixt.Mode(lacros.LacrosSideBySide))}, // custom config
	} {
		bt := param.bt
		cfg := param.cfg
		s.Run(ctx, fmt.Sprintf("BrowserWithNewChrome browser: %v, cfg: %+v", bt, cfg), func(ctx context.Context, s *testing.State) {
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
			const url = "chrome://newtab"
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
}
