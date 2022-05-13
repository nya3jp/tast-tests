// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DebugBug1324662,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Reproduces the problem underlying https://crbug.com/1324662, with as little code as possible",
		Contacts:     []string{"amusbach@chromium.org", "chromeos-perfmetrics-eng@google.com"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "lacros",
	})
}

func DebugBug1324662(ctx context.Context, s *testing.State) {
	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()

	cr, l, _, err := lacros.Setup(ctx, s.FixtValue().(chrome.HasChrome).Chrome(), browser.TypeLacros)
	if err != nil {
		s.Fatal("Failed to set up lacros: ", err)
	}
	defer lacros.CloseLacros(closeCtx, l)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test api: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(closeCtx)

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}

	pc := pointer.NewMouse(tconn)
	defer pc.Close()

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the window list: ", err)
	}
	if len(ws) != 1 {
		s.Errorf("Unexpected number of windows: got %d; want 1", len(ws))
	}
	wID := ws[0].ID

	if err := ash.SetWindowStateAndWait(ctx, tconn, wID, ash.WindowStateMaximized); err != nil {
		s.Fatal("Failed to maximize window: ", err)
	}

	if err := pc.Click(
		nodewith.Name("Minimize").ClassName("FrameCaptionButton").Role(role.Button),
	)(ctx); err != nil {
		s.Fatal("Failed to click minimize button: ", err)
	}
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == wID && w.State == ash.WindowStateMinimized && !w.IsAnimating
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to wait for window to become minimized: ", err)
	}

	if err := ash.SetWindowStateAndWait(ctx, tconn, wID, ash.WindowStateNormal); err != nil {
		s.Fatal("Failed to restore window: ", err)
	}

	w, err := ash.GetWindow(ctx, tconn, wID)
	if err != nil {
		s.Fatal("Failed to get window info: ", err)
	}

	if err := pc.Drag(
		coords.NewPoint(w.BoundsInRoot.Left+w.BoundsInRoot.Width*3/4, w.BoundsInRoot.Top+10),
		pc.DragTo(info.WorkArea.CenterPoint(), 2*time.Second),
	)(ctx); err != nil {
		s.Fatal("Failed to drag window: ", err)
	}
}
