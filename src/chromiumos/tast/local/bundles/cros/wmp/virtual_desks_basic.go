// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
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
		Func:         VirtualDesksBasic,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that virtual desks works correctly",
		Contacts: []string{
			"yichenz@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
		}, {
			Name:              "lacros",
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

func VirtualDesksBasic(ctx context.Context, s *testing.State) {
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

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	pc := pointer.NewMouse(tconn)
	defer pc.Close()

	// Opens Files and Chrome.
	browserApp, err := apps.PrimaryBrowser(ctx, tconn)
	if err != nil {
		s.Fatal("Could not find browser app info: ", err)
	}
	for _, app := range []apps.App{browserApp, apps.FilesSWA} {
		if err := apps.Launch(ctx, tconn, app.ID); err != nil {
			s.Fatalf("Failed to open %s: %v", app.Name, err)
		}
		if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
			s.Fatalf("%s did not appear in shelf after launch: %s", app.Name, err)
		}
	}

	// Enters overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}
	defer ash.SetOverviewModeAndWait(cleanupCtx, tconn, false)

	// Creates new desk.
	addDeskButton := nodewith.ClassName("ZeroStateIconButton")
	newDeskNameView := nodewith.ClassName("DeskNameView").Name("Desk 2")
	newDeskName := "new desk"
	newDeskMiniView :=
		nodewith.ClassName("DeskMiniView").Name(fmt.Sprintf("Desk: %s", newDeskName))
	if err := uiauto.Combine(
		"create a new desk",
		ac.DoDefault(addDeskButton),
		// The focus on the new desk should be on the desk name field.
		ac.WaitUntilExists(newDeskNameView.Focused()),
		kb.TypeAction(newDeskName),
		kb.AccelAction("Enter"),
	)(ctx); err != nil {
		s.Fatal("Failed to create a new desk: ", err)
	}
	// Verifies that there are 2 desks.
	deskMiniViewsInfo, err := ash.FindDeskMiniViews(ctx, ac)
	if err != nil {
		s.Fatal("Failed to find desks: ", err)
	}
	if len(deskMiniViewsInfo) != 2 {
		s.Fatalf("Got %v desks, want 2 desks", len(deskMiniViewsInfo))
	}

	// Reorders desks by drag and drop.
	firstDeskMiniViewLoc, secondDeskMiniViewLoc := deskMiniViewsInfo[0].Location, deskMiniViewsInfo[1].Location
	if err := pc.Drag(
		firstDeskMiniViewLoc.CenterPoint(),
		pc.DragTo(secondDeskMiniViewLoc.CenterPoint(), 3*time.Second))(ctx); err != nil {
		s.Fatal("Failed to drag and drop desks: ", err)
	}
	// The new desk should be the first desk on the list.
	newDeskLoc, err := ac.Location(ctx, newDeskMiniView)
	if err != nil {
		s.Fatal("Failed to get the location of the new desk mini view: ", err)
	}
	if *newDeskLoc != firstDeskMiniViewLoc {
		s.Fatal("New desk is not the first desk")
	}

	// Drags Files App into the new desk.
	filesAppWindowView := nodewith.ClassName("BrowserFrame").Name("Files - My files")
	filesAppWindowViewLoc, err := ac.Location(ctx, filesAppWindowView)
	if err != nil {
		s.Fatal("Failed to get the location of the Files app: ", err)
	}
	if err := pc.Drag(
		filesAppWindowViewLoc.CenterPoint(),
		pc.DragTo(firstDeskMiniViewLoc.CenterPoint(), 3*time.Second))(ctx); err != nil {
		s.Fatal("Failed to drag Files app into the new desk: ", err)
	}
	// Checks that Files App is in the new desk. The new desk is inactive.
	if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
		if (w.Title == "Files - My files") && w.OnActiveDesk == true {
			return errors.New("Files app should be in the inactive desk")
		}
		if (w.Title == "Chrome - New Tab") && w.OnActiveDesk == false {
			return errors.New("Chrome app should be in the active desk")
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to verify the desk of the app: ", err)
	}

	// Delete the new desk.
	closeDeskButton := nodewith.ClassName("CloseButton").Ancestor(newDeskMiniView).First()
	if err := uiauto.Combine(
		"Delete a new desk",
		ac.DoDefault(closeDeskButton),
		ac.WaitUntilGone(newDeskMiniView),
	)(ctx); err != nil {
		s.Fatal("Failed to delete the new desk: ", err)
	}
	// There should still be 2 visible windows. Deleting the new desk won't delete the
	// Files app in it.
	windowCount, err := ash.CountVisibleWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to count visible windows: ", err)
	}
	if windowCount != 2 {
		s.Fatalf("Expected 2 visible windows, got %v instead", windowCount)
	}
}
