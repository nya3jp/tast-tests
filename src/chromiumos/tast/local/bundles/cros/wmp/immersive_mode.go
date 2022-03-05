// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ImmersiveMode,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that immersive mode works correctly",
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
			Fixture:           "lacrosPrimary",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
	})
}

func ImmersiveMode(ctx context.Context, s *testing.State) {
	const (
		timeout = 30 * time.Second
	)

	// Shorten context for cleanup.
	closeCtx := ctx
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
	defer cleanup(closeCtx)

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Ensure there is no window open before test starts.
	if err := ash.CloseAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to ensure no window is open: ", err)
	}

	// Open a browser window either ash-chrome or lacros-chrome.
	bt := s.Param().(browser.Type)
	browserApp, err := apps.PrimaryBrowser(ctx, tconn, bt)
	if err != nil {
		s.Fatalf("Could not find %v browser app info: %v", bt, err)
	}
	if err := apps.Launch(ctx, tconn, browserApp.ID); err != nil {
		s.Fatal("Failed to launch chrome: ", err)
	}

	// Ensure that there is only one open window that is the primary browser. Wait for the browser to be visible to avoid a race that may cause test flakiness.
	bw, err := wmputils.EnsureOnlyBrowserWindowOpen(ctx, tconn, bt)
	if err != nil {
		s.Fatal("Expected the window to be fullscreen but got: ", err)
	}
	defer bw.CloseWindow(closeCtx, tconn)

	// Press F4 to trigger immersive mode.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer kb.Close()
	if err = kb.Accel(ctx, "F4"); err != nil {
		s.Fatal("Failed to press F4: ", err)
	}

	// Check the chrome window is in immersive mode.
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == bw.ID && w.State == ash.WindowStateFullscreen && !w.IsAnimating
	}, &testing.PollOptions{Timeout: timeout, Interval: time.Second}); err != nil {
		s.Fatalf("Expected the window to be fullscreen but it is %s", bw.State)
	}

	// Launcher should be hidden in immersive mode.
	ac := uiauto.New(tconn).WithTimeout(timeout)
	launcher := nodewith.Name("Launcher").ClassName("ash/HomeButton").Role(role.Button)
	if err := ac.WaitUntilGone(launcher)(ctx); err != nil {
		s.Fatal("Launcher is present in immersive mode: ", err)
	}

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}
	// Swipe up from the bottom edge to the center.
	tc, err := touch.New(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the touch context: ", err)
	}
	screenBottomPt := info.Bounds.BottomCenter().Sub(coords.NewPoint(0, 1))
	screenCenterPt := info.Bounds.CenterPoint()
	if err := uiauto.Combine(
		"swipe up from the bottom edge, and check if the launcher appears",
		tc.Swipe(screenBottomPt, tc.SwipeTo(screenCenterPt, time.Second)),
		// Launcher should appear after the swipe.
		ac.WaitUntilExists(launcher),
	)(ctx); err != nil {
		s.Fatal("Failed to swipe up to reveal launcher: ", err)
	}
}
