// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ZeroStateDesksBar,
		Desc: "Checks that zero state desks bar in overview works correctly",
		Contacts: []string{
			"minch@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

func ZeroStateDesksBar(ctx context.Context, s *testing.State) {
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 55*time.Second)
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
	defer cleanup(cleanupCtx)

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	ac := uiauto.New(tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	pc := pointer.NewMouse(tconn)
	defer pc.Close()

	// Enters overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}

	// Zero state desks bar is shown when there is only one desk. Tab to
	// default desk button inside and press "Enter" should expand the desks
	// bar but no new desk should be created.
	desk1DeskNameView := nodewith.ClassName("DeskNameView").Name("Desk 1")
	if err := uiauto.Combine(
		"press the default desk button to expanded desks bar",
		kb.AccelAction("Tab"),
		kb.AccelAction("Enter"),
		// The desk name view of the default desk mini view should be focused.
		ac.WaitUntilExists(desk1DeskNameView.Focused()),
	)(ctx); err != nil {
		s.Fatal("Failed to switch to expanded desks bar: ", err)
	}
	// Verifies that there is only 1 desk.
	ash.FindDeskMiniViews(ctx, ac, 1)

	// Tab to the new desk button inside expanded desks bar and press "Enter"
	// should create a new desk.
	desk2DeskNameView := nodewith.ClassName("DeskNameView").Name("Desk 2")
	newDeskName := "new desk"
	if err := uiauto.Combine(
		"create a new desk through new desk button in the expanded desks bar",
		kb.AccelAction("Tab"),
		kb.AccelAction("Enter"),
		ac.WaitUntilExists(desk2DeskNameView.Focused()),
		kb.TypeAction(newDeskName),
		kb.AccelAction("Enter"),
	)(ctx); err != nil {
		s.Fatal("Failed to create a new desk through expanded new desk button: ", err)
	}
	// Verifies that there 2 desks.
	deskMiniViewsInfo, err := ash.FindDeskMiniViews(ctx, ac, 2)
	if err != nil {
		s.Fatal("Failed to find desks: ", err)
	}

	firstDeskMiniViewLoc, secondDeskMiniViewLoc := deskMiniViewsInfo[0].Location, deskMiniViewsInfo[1].Location
	// Verifies that the new desk button inside the expanded desks bar has the same size as the desk preview.
	newDeskButtonInExpandedDesksLoc, err := ac.Location(ctx, nodewith.ClassName("ExpandedDesksBarButton"))
	if err != nil {
		s.Fatal("Failed to get the location of the new desk button inside expanded desks bar: ", err)
	}
	if (*newDeskButtonInExpandedDesksLoc).Width != secondDeskMiniViewLoc.Width {
		s.Fatal("New desk button inside expanded desks bar has different width as desk preview")
	}
	if (*newDeskButtonInExpandedDesksLoc).Height != secondDeskMiniViewLoc.Height {
		s.Fatal("New desk button inside expanded desks bar has different height as desk preview")
	}

	// Drag "new desk" to be the first desk and delete it.
	if err := pc.Drag(
		secondDeskMiniViewLoc.CenterPoint(),
		pc.DragTo(firstDeskMiniViewLoc.CenterPoint(), time.Second))(ctx); err != nil {
		s.Fatal("Failed to drag and drop desks: ", err)
	}
	closeDeskButton := nodewith.ClassName("CloseDeskButton")
	if err := ac.LeftClick(closeDeskButton)(ctx); err != nil {
		s.Fatal("Failed to delete desk 1: ", err)
	}
	// Verifies that there is only 1 desk.
	ash.FindDeskMiniViews(ctx, ac, 1)

	defer cancel()

	// It should switch back to zero state desks bar when there is only 1 desk.
	// Tab to the new desk button inside the zero state desks bar and press "Enter"
	// should expand the desks bar and create a new desk.
	if err := uiauto.Combine(
		"create a new desk through new desk button in zero state desks bar",
		uiauto.Repeat(2, kb.AccelAction("Tab")),
		kb.AccelAction("Enter"),
		ac.WaitUntilExists(desk2DeskNameView.Focused()),
		kb.TypeAction(newDeskName),
		kb.AccelAction("Enter"),
	)(ctx); err != nil {
		s.Fatal("Failed to create a new desk through expanded new desk button: ", err)
	}
	ash.FindDeskMiniViews(ctx, ac, 2)
}
