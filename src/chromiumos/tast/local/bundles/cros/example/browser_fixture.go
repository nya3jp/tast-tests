// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
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
			Fixture:           "exampleLacrosPrimary",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
		Vars: []string{"lacrosDeployedBinary"},
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

	// Set up a browser. TODO: Consider adding a convenient util to browserfixt
	var br *browser.Browser
	if bt == browser.TypeAsh {
		br = cr.Browser()
	}
	if bt == browser.TypeLacros {
		lacrosExecPath, _ /* lacrosUserData */ := cr.EnsureLacrosReadyForLaunch(ctx)
		l, err := lacros.LaunchFromShelf(ctx, tconn, lacrosExecPath)
		if err != nil {
			// TODO(crbug.com/1298962): Replace with lacrosfaillog to save lacros.log on failure for debugging.
			if out, ok := testing.ContextOutDir(ctx); !ok {
				testing.ContextLog(ctx, "OutDir not found")
			} else if err := fsutil.CopyFile(filepath.Join(lacros.UserDataDir, "lacros.log"), filepath.Join(out, "lacros.log")); err != nil {
				testing.ContextLogf(ctx, "Failed to copy lacros.log from %v to %v, err: %v", lacros.UserDataDir, out, err)
			}
			s.Fatal("Failed to launch lacros-chrome: ", err)
		}
		closeBrowser := func(ctx context.Context) {
			if err := l.Close(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to close lacros-chrome: ", err)
			}
		}
		br = l.Browser()
		defer closeBrowser(cleanupCtx)
	}

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
