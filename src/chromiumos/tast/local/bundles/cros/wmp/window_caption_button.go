// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/wmp/wmputils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowCaptionButton,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that window caption buttons work properly",
		Contacts: []string{
			"conniekxu@chromium.org",
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

func WindowCaptionButton(ctx context.Context, s *testing.State) {
	const (
		timeout = 30 * time.Second
	)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	// Ensure there is no window open before test starts.
	if err := ash.CloseAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to ensure no window is open: ", err)
	}

	// Open a browser window either ash-chrome or lacros-chrome.
	browserApp, err := apps.PrimaryBrowser(ctx, tconn)
	if err != nil {
		s.Fatal("Could not find browser app info: ", err)
	}
	if err := apps.Launch(ctx, tconn, browserApp.ID); err != nil {
		s.Fatal("Failed to launch chrome: ", err)
	}

	// Ensure that there is only one open window that is the primary browser. Wait for the browser to be visible to avoid a race that may cause test flakiness.
	bt := s.Param().(browser.Type)
	bw, err := wmputils.EnsureOnlyBrowserWindowOpen(ctx, tconn, bt)
	if err != nil {
		s.Fatal("Expected there's only one browser window open but got: ", err)
	}
	defer bw.CloseWindow(cleanupCtx, tconn)

	// Minimize the window using the minimize caption button.
	ui := uiauto.New(tconn)

	minimizeButton := nodewith.Name("Minimize")
	if err := ui.LeftClick(minimizeButton)(ctx); err != nil {
		s.Fatal("Failed to click on the minimize caption button: ", err)
	}

	// Verify the chrome window has minimized state now.
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == bw.ID && w.State == ash.WindowStateMinimized && !w.IsAnimating
	}, &testing.PollOptions{Timeout: timeout, Interval: time.Second}); err != nil {
		s.Fatalf("Expected the chrome window to be minimized but it is %s", bw.State)
	}

	// Click on the browser app to bring window back.
	chromeIcon := nodewith.ClassName("ash/ShelfAppButton").First()
	if err := ui.LeftClick(chromeIcon)(ctx); err != nil {
		s.Fatal("Failed to click on the chrome icon: ", err)
	}

	// Verify the chrome window has normal state now.
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == bw.ID && w.State == ash.WindowStateNormal && !w.IsAnimating
	}, &testing.PollOptions{Timeout: timeout, Interval: time.Second}); err != nil {
		s.Fatalf("Expected the browser window to be normal but it is %s", bw.State)
	}
	faillog.DumpUITree(ctx, s.OutDir(), tconn)

	// Maximize the window using the maximize caption button.
	maximizeButton := nodewith.Name("Maximize")
	if ui.LeftClick(maximizeButton)(ctx); err != nil {
		s.Fatal("Failed to click on the maximize caption button: ", err)
	}

	// Verify the chrome window has maximized state now.
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == bw.ID && w.State == ash.WindowStateMaximized && !w.IsAnimating
	}, &testing.PollOptions{Timeout: timeout, Interval: time.Second}); err != nil {
		s.Fatalf("Expected the browser window to be maximized but it is %s", bw.State)
	}

	// Restore the chrome window using the restore caption button.
	restoreButton := nodewith.Name("Restore")
	if err := ui.LeftClick(restoreButton)(ctx); err != nil {
		s.Fatal("Failed to click on the restore caption button: ", err)
	}

	// Verify the chrome window has normal state now.
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == bw.ID && w.State == ash.WindowStateNormal && !w.IsAnimating
	}, &testing.PollOptions{Timeout: timeout, Interval: time.Second}); err != nil {
		s.Fatalf("Expected the browser window to be normal but it is %s", bw.State)
	}

	// Open the FilesApp window.
	if _, err := filesapp.Launch(ctx, tconn); err != nil {
		s.Fatal("Failed to launch FilesApp window: ", err)
	}

	filesApp, err := ash.GetActiveWindow(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the active window: ", err)
	}

	// Maximize the FilesApp window by using the keyboard shortcuts `Alt+=`.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer kb.Close()
	if err := uiauto.Combine(
		"Maximize the FilesApp window using the keyboard shortcuts",
		kb.AccelPressAction("Alt+="),
		kb.AccelReleaseAction("Alt+="),
	)(ctx); err != nil {
		s.Fatal("Failed to maximize the FilesApp window: ", err)
	}

	// Verify the FilesApp window has maximized state now.
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == filesApp.ID && w.State == ash.WindowStateMaximized && !w.IsAnimating
	}, &testing.PollOptions{Timeout: timeout, Interval: time.Second}); err != nil {
		s.Fatalf("Expected the FilesApp window to be normal but it is %s", bw.State)
	}

	// Close the FilesApp window by pressing the Close caption button on it.
	window := nodewith.NameContaining("Files").HasClass("BrowserFrame")
	closeButton := nodewith.Name("Close").Ancestor(window)
	if err := ui.LeftClick(closeButton)(ctx); err != nil {
		s.Fatal("Failed to click on the close caption button: ", err)
	}

	// Verify that the FilesApp window has been closed.
	// Verify that there's only one browser window after the FilesApp window was closed.
	ws, err := ash.GetAllWindows(ctx, tconn)
	if len(ws) != 1 {
		s.Fatalf("Got %d window(s), Expected 1 window", len(ws))
	}
	if ws[0].ID != bw.ID {
		s.Fatal("Failed to close FilesApp window")
	}
}
