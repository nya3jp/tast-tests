// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OverviewMode,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that overview mode works correctly",
		Contacts: []string{
			"yichenz@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
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

func OverviewMode(ctx context.Context, s *testing.State) {
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Ensure there is no window open before test starts.
	if err := ash.CloseAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to ensure no window is open: ", err)
	}

	ac := uiauto.New(tconn)

	bt := s.Param().(browser.Type)
	browserApp, err := apps.PrimaryBrowser(ctx, tconn)
	if err != nil {
		s.Fatal("Could not find browser app info: ", err)
	}
	for _, app := range []apps.App{apps.FilesSWA, browserApp} {
		if err := apps.Launch(ctx, tconn, app.ID); err != nil {
			s.Fatalf("Failed to launch %s: %s", app.Name, err)
		}
		if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
			s.Fatalf("%s did not appear in shelf after launch: %s", app.Name, err)
		}
		if err := ash.WaitForAppWindow(ctx, tconn, app.ID); err != nil {
			s.Fatalf("%s did not become visible: %s", app.Name, err)
		}
	}
	// Set Chrome window's state to maximized and Files window's state to normal.
	if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
		if ash.BrowserTypeMatch(bt)(w) {
			return ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateMaximized)
		}
		if strings.Contains(w.Title, "Files") {
			return ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateNormal)
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to set window states: ", err)
	}

	// Overview only animates the user visible windows for performance reasons.
	// Here the Chrome window is maximized and completely occludes the Files window,
	// so the expectation is that only the Chrome window animates.
	var animationError error
	go func() {
		testing.Poll(ctx, func(ctx context.Context) error {
			ws, err := ash.GetAllWindows(ctx, tconn)
			if err != nil {
				animationError = errors.Wrap(err, "failed to get the window list")
				return testing.PollBreak(animationError)
			}
			for _, window := range ws {
				if ash.BrowserTypeMatch(bt)(window) && !window.IsAnimating {
					animationError = errors.New("chrome window is not animating")
					return animationError
				}
				if strings.Contains(window.Title, "Files") && window.IsAnimating {
					animationError = errors.New("files window is animating")
					return animationError
				}
			}
			animationError = nil
			return nil
		}, &testing.PollOptions{Timeout: time.Second, Interval: 50 * time.Millisecond})
	}()
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to enter into the overview mode: ", err)
	}
	defer ash.SetOverviewModeAndWait(cleanupCtx, tconn, false)
	if animationError != nil {
		s.Fatal("Maximized and(or) normal windows didn't open in the overview as expected: ", animationError)
	}

	// Clicking the close button in overview should close the window.
	chromeOverviewItemView := nodewith.NameRegex(regexp.MustCompile(".*New Tab")).ClassName("OverviewItemView")
	closeChromeButton := nodewith.ClassName("CloseButton").Ancestor(chromeOverviewItemView)
	if err := ac.LeftClick(closeChromeButton)(ctx); err != nil {
		s.Fatal("Failed to close chrome window: ", err)
	}
	if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		s.Fatal("Failed to wait for location-change events to be completed: ", err)
	}
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the window list: ", err)
	}
	if len(ws) != 1 {
		s.Fatalf("Expected 1 window, got %v window(s)", len(ws))
	}
	if ash.BrowserTypeMatch(bt)(ws[0]) {
		s.Fatal("Chrome window still exists after closing it in overview")
	}
}
