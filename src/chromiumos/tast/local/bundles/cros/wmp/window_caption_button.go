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
	const timeout = 30 * time.Second

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Second)
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

	// Open either an ash-chrome or lacros-chrome browser window.
	browserApp, err := apps.PrimaryBrowser(ctx, tconn)
	if err != nil {
		s.Fatal("Could not find browser app info: ", err)
	}
	if err := apps.Launch(ctx, tconn, browserApp.ID); err != nil {
		s.Fatal("Failed to launch Chrome: ", err)
	}

	// Ensure that there is only one open window that is the primary browser.
	bt := s.Param().(browser.Type)
	bw, err := wmputils.EnsureOnlyBrowserWindowOpen(ctx, tconn, bt)
	if err != nil {
		s.Fatal("Failed to ensure only one browser window is open: ", err)
	}
	defer bw.CloseWindow(cleanupCtx, tconn)

	ui := uiauto.New(tconn)

	// Set the Chrome window to normal state before testing the caption button actions.
	if err := ash.SetWindowStateAndWait(ctx, tconn, bw.ID, ash.WindowStateNormal); err != nil {
		s.Fatal("Failed to set Chrome window state to \"Normal\": ", err)
	}

	// Minimize the window using the minimize caption button.
	minimizeButton := nodewith.Name("Minimize")
	if err := ui.LeftClick(minimizeButton)(ctx); err != nil {
		s.Fatal("Failed to click on the minimize caption button: ", err)
	}

	// Verify the Chrome window has minimized state now.
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == bw.ID && w.State == ash.WindowStateMinimized && !w.IsAnimating
	}, &testing.PollOptions{Timeout: timeout, Interval: time.Second}); err != nil {
		s.Fatalf("Unexpected Chrome window state: got %s, want %s", bw.State, ash.WindowStateMinimized)
	}

	// Click on the browser app to bring window back. This is under the assumption that the Chrome icon is the first icon on the shelf.
	chromeIcon := nodewith.ClassName("ash/ShelfAppButton").First()
	if err := ui.LeftClick(chromeIcon)(ctx); err != nil {
		s.Fatal("Failed to click on the Chrome icon: ", err)
	}

	// Verify the Chrome window has normal state now.
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == bw.ID && w.State == ash.WindowStateNormal && !w.IsAnimating
	}, &testing.PollOptions{Timeout: timeout, Interval: time.Second}); err != nil {
		s.Fatalf("Unexpected Chrome window state: got %s, want %s", bw.State, ash.WindowStateNormal)
	}
	faillog.DumpUITree(ctx, s.OutDir(), tconn)

	// Maximize the window using the maximize caption button.
	maximizeButton := nodewith.Name("Maximize")
	if ui.LeftClick(maximizeButton)(ctx); err != nil {
		s.Fatal("Failed to click on the maximize caption button: ", err)
	}

	// Verify the Chrome window has maximized state now.
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == bw.ID && w.State == ash.WindowStateMaximized && !w.IsAnimating
	}, &testing.PollOptions{Timeout: timeout, Interval: time.Second}); err != nil {
		s.Fatalf("Unexpected Chrome window state: got %s, want %s", bw.State, ash.WindowStateMaximized)
	}

	// Restore the Chrome window using the restore caption button.
	restoreButton := nodewith.Name("Restore")
	if err := ui.LeftClick(restoreButton)(ctx); err != nil {
		s.Fatal("Failed to click on the restore caption button: ", err)
	}

	// Verify the Chrome window has normal state now.
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == bw.ID && w.State == ash.WindowStateNormal && !w.IsAnimating
	}, &testing.PollOptions{Timeout: timeout, Interval: time.Second}); err != nil {
		s.Fatalf("Unexpected Chrome window state: got %s, want %s", bw.State, ash.WindowStateNormal)
	}

	// Open the FilesApp window.
	if _, err := filesapp.Launch(ctx, tconn); err != nil {
		s.Fatal("Failed to launch FilesApp window: ", err)
	}

	filesApp, err := ash.GetActiveWindow(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the active window: ", err)
	}

	// Set the FilesApp window state to normal state before testing the caption button actions.
	if err := ash.SetWindowStateAndWait(ctx, tconn, filesApp.ID, ash.WindowStateNormal); err != nil {
		s.Fatal("Failed to set FilesApp window state to \"Normal\": ", err)
	}

	// Maximize the FilesApp window by using the keyboard shortcuts `Alt+=`.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer kb.Close()

	if err := kb.AccelAction("Alt+=")(ctx); err != nil {
		s.Fatal("Failed to maximize the FilesApp window with keyboard shortcut: ", err)
	}

	// Verify the FilesApp window has maximized state now.
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == filesApp.ID && w.State == ash.WindowStateMaximized && !w.IsAnimating
	}, &testing.PollOptions{Timeout: timeout, Interval: time.Second}); err != nil {
		s.Fatalf("Unexpected Chrome window state: got %s, want %s", bw.State, ash.WindowStateMaximized)
	}

	// Close the FilesApp window by pressing the Close caption button on it.
	window := nodewith.NameContaining("Files").HasClass("BrowserFrame")
	closeButton := nodewith.Name("Close").Ancestor(window)
	if err := ui.LeftClick(closeButton)(ctx); err != nil {
		s.Fatal("Failed to click on the close caption button: ", err)
	}

	// Verify that there's only one browser window after the FilesApp window was closed.
	ws, err := ash.GetAllWindows(ctx, tconn)
	if len(ws) != 1 {
		s.Fatalf("Got %d window(s), want 1 window", len(ws))
	}
	if ws[0].ID != bw.ID {
		s.Fatalf("Unexpected open window; got %d, want %d", ws[0].ID, bw.ID)
	}
}
