// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
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
		Func: ImmersiveMode,
		Desc: "Checks that immersive mode works correctly",
		Contacts: []string{
			"yichenz@chromium.org",
			"chromeos-wmp@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

func ImmersiveMode(ctx context.Context, s *testing.State) {
	const (
		timeout = 30 * time.Second
	)

	// Shorten context for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)
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

	if err := apps.Launch(ctx, tconn, apps.Chrome.ID); err != nil {
		s.Fatalf("Failed to open %s: %s", apps.Chrome.Name, err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer kb.Close()

	ac := uiauto.New(tconn).WithTimeout(timeout)

	// Press F4 to trigger immersive mode.
	if err = kb.Accel(ctx, "F4"); err != nil {
		s.Fatal("Failed to press F4: ", err)
	}
	// Check the chrome window is in immersive mode.
	ws, err := ash.GetAllWindows(ctx, tconn)
	if len(ws) != 1 {
		s.Fatalf("Expected 1 window, got %d window(s)", len(ws))
	}
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == ws[0].ID && w.State == ash.WindowStateFullscreen && !w.IsAnimating
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		s.Fatalf("Expected the window to be fullscreen but it is %s", ws[0].State)
	}

	// Launcher should be hidden in immersive mode.
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
	screenBottomPt := info.Bounds.BottomCenter().Sub(coords.Point{0, 1})
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
s