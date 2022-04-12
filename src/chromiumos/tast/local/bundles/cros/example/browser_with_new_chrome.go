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
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros"
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
		Vars:         []string{browserfixt.LacrosDeployedBinary},
	})
}

func BrowserWithNewChrome(ctx context.Context, s *testing.State) {
	// Reserve some time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	for _, param := range []struct {
		bt  browser.Type
		cfg *browserfixt.LacrosConfig
	}{
		{browser.TypeAsh, nil},
		{browser.TypeAsh, browserfixt.DefaultLacrosConfig},                                        // LacrosConfig is a no-op for ash-chrome.
		{browser.TypeLacros, browserfixt.DefaultLacrosConfig},                                     // default config
		{browser.TypeLacros, browserfixt.DefaultLacrosConfig.WithVar(s)},                          // default config with the var --lacrosDeployedBinary specified
		{browser.TypeLacros, browserfixt.NewLacrosConfig(lacros.Rootfs, lacros.LacrosSideBySide)}, // custom config
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
