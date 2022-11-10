// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FloatWindow,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test that the float shortcut works on a floatable window",
		Contacts: []string{
			"hewer@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val: browser.TypeAsh,
		}, {
			Name:              "lacros",
			Val:               browser.TypeLacros,
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

// FloatWindow floats an open app window using the keyboard shortcut.
func FloatWindow(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	bt := s.Param().(browser.Type)
	cr, _, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, bt, lacrosfixt.NewConfig(),
		chrome.EnableFeatures("CrOSLabsFloatWindow"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)
	defer closeBrowser(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	// Ensure there is no window open before test starts.
	if err := ash.CloseAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to close any existing windows: ", err)
	}

	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch the files app: ", err)
	}
	defer filesApp.Close(cleanupCtx)

	window, err := ash.WaitForAnyWindow(ctx, tconn, func(w *ash.Window) bool { return w.AppID == apps.FilesSWA.ID && w.IsVisible && !w.IsAnimating })
	if err != nil {
		s.Fatal("Failed to wait for files app to be visible and stop animating: ", err)
	}

	// Set the app to normal state so we can check unfloating goes
	// back to normal state.
	if err := ash.SetWindowStateAndWait(ctx, tconn, window.ID, ash.WindowStateNormal); err != nil {
		s.Fatal("Failed to set files app window state to \"Normal\": ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to initialize keyboard: ", err)
	}
	defer kb.Close()

	if err := kb.Accel(ctx, "Search+Alt+F"); err != nil {
		s.Fatal("Failed to input float accelerator: ", err)
	}

	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == window.ID && w.State == ash.WindowStateFloated && !w.IsAnimating
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatalf("Unexpected files app window state: got %s, want %s", window.State, ash.WindowStateFloated)
	}

	if err := kb.Accel(ctx, "Search+Alt+F"); err != nil {
		s.Fatal("Failed to input unfloat accelerator: ", err)
	}

	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == window.ID && w.State == ash.WindowStateNormal && !w.IsAnimating
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatalf("Unexpected files app window state: got %s, want %s", window.State, ash.WindowStateNormal)
	}
}
