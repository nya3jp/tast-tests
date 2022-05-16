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
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DebugBug1324662,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Reproduces potentially related issue in https://crbug.com/1324662",
		Contacts:     []string{"ramsaroop@chromium.org", "chromeos-perfmetrics-eng@google.com"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "lacros",
	})
}

func DebugBug1324662(ctx context.Context, s *testing.State) {
	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()

	cr, l, cs, err := lacros.Setup(ctx, s.FixtValue().(chrome.HasChrome).Chrome(), browser.TypeLacros)
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

	_, err = cs.NewConn(ctx, "https://youtube.com", browser.WithNewWindow())
	if err != nil {
		s.Fatal("Failed to open the second website: ", err)
	}

	pc := pointer.NewMouse(tconn)
	defer pc.Close()

	snapLeftPoint := coords.NewPoint(info.WorkArea.Left+1, info.WorkArea.CenterY())
	snapRightPoint := coords.NewPoint(info.WorkArea.Right()-1, info.WorkArea.CenterY())

	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}

	ws, err := ash.GetAllWindows(ctx, tconn)
	firstWindow := ws[0]
	secondWindow := ws[1]

	// This drag succeeds
	if err := pc.Drag(
		firstWindow.OverviewInfo.Bounds.CenterPoint(),
		pc.DragTo(snapLeftPoint, time.Second),
	)(ctx); err != nil {
		s.Fatal("Failed to drag window: ", err)
	}

	// This one never actually drags the window, but doesn't throw an error
	if err := pc.Drag(
		secondWindow.OverviewInfo.Bounds.CenterPoint(),
		pc.DragTo(snapRightPoint, time.Second),
	)(ctx); err != nil {
		s.Fatal("Failed to drag window: ", err)
	}
	testing.Sleep(ctx, 30*time.Second)
}
