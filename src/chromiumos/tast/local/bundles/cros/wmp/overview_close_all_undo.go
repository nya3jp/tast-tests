// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OverviewCloseAllUndo,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks the action to close all windows and desks can be canceled",
		Contacts: []string{
			"yzd@chromium.org",
			"chromeos-wmp@google.com",
			"cros-commercial-productivity-eng@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Timeout:      chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
		VarDeps:      []string{"ui.gaiaPoolDefault"},
	})
}

func OverviewCloseAllUndo(ctx context.Context, s *testing.State) {
	// Reserves five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.EnableFeatures("DesksCloseAll"),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

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
	defer kb.Close()

	pc := pointer.NewMouse(tconn)
	defer pc.Close()

	// Sets up for launching ARC apps.
	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store and Close: ", err)
	}

	// Sets up ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(cleanupCtx)

	if err := a.WaitIntentHelper(ctx); err != nil {
		s.Fatal("Failed to wait for ARC Intent Helper: ", err)
	}

	// Enters overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}
	defer ash.SetOverviewModeAndWait(cleanupCtx, tconn, false)
	defer ash.CleanUpDesks(cleanupCtx, tconn)

	newDeskButton := nodewith.ClassName("ZeroStateIconButton")
	desk2NameView := nodewith.ClassName("DeskNameView").Name("Desk 2")
	desk2Name := "BusyDesk"
	if err := uiauto.Combine(
		"create a new desk by clicking new desk button",
		ac.LeftClick(newDeskButton),
		// The focus on the new desk should be on the desk name field.
		ac.WaitUntilExists(desk2NameView.Focused()),
		kb.TypeAction(desk2Name),
		kb.AccelAction("Enter"),
	)(ctx); err != nil {
		s.Fatal("Failed to change the name of the second desk: ", err)
	}

	// Exits overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		s.Fatal("Failed to exit overview mode: ", err)
	}

	// Activates the second desk and launch app windows on it.
	if err := ash.ActivateDeskAtIndex(ctx, tconn, 1); err != nil {
		s.Fatal("Failed to activate desk 2: ", err)
	}

	// Opens PlayStore, Chrome and Files.
	for _, app := range []apps.App{apps.PlayStore, apps.Chrome, apps.FilesSWA} {
		if err := apps.Launch(ctx, tconn, app.ID); err != nil {
			s.Fatalf("Failed to open %s: %v", app.Name, err)
		}
		if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
			s.Fatalf("%s did not appear in shelf after launch: %s", app.Name, err)
		}

		// Waits for the launched app window to become visible.
		if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
			return w.IsVisible && strings.Contains(w.Title, app.Name)
		}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
			s.Fatalf("%v app window not visible after launching: %v", app.Name, err)
		}
	}

	if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		s.Fatal("Failed to wait for app launch events to be completed: ", err)
	}

	// Enters overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}

	desk2DeskMiniView := nodewith.ClassName("DeskMiniView").Name(fmt.Sprintf("Desk: %s", desk2Name))

	// Gets the location of the second desk mini view.
	desk2DeskMiniViewLoc, err := ac.Location(ctx, desk2DeskMiniView)
	if err != nil {
		s.Fatalf("Failed to get the mini view location of desk %s: %v", desk2Name, err)
	}

	// Moves the mouse to second desk mini view and hover.
	if err := mouse.Move(tconn, desk2DeskMiniViewLoc.CenterPoint(), 100*time.Millisecond)(ctx); err != nil {
		s.Fatal("Failed to hover at the second desk mini view: ", err)
	}

	// Finds the "Close All" button.
	closeAllButton := nodewith.ClassName("CloseButton").Name("Close desk and windows")

	// Closes a desk and windows on it.
	if err := pc.Click(closeAllButton)(ctx); err != nil {
		s.Fatal("Failed to close all windows on a desk: ", err)
	}

	// Exits overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		s.Fatal("Failed to exit overview mode: ", err)
	}

	// Waits for the CloseAll toast to show up.
	if err := ac.WaitUntilExists(nodewith.ClassName("ToastOverlay"))(ctx); err != nil {
		s.Fatal("Failed to wait for CloseAll toast: ", err)
	}

	// There should be 0 visible windows since all windows are closed.
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to count windows: ", err)
	}
	for _, window := range ws {
		if window.IsVisible {
			s.Fatal("Found unexpected visible windows: ", window)
		}
	}

	// There should be one desk at this point.
	dc, err := ash.GetDeskCount(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to count desks: ", err)
	}

	if dc != 1 {
		s.Fatalf("Unexpected number of desks: got %v, want 1", dc)
	}

	// Finds the "Undo" button on the CloseAll toast.
	undoButton := nodewith.ClassName("PillButton").Name("Undo")

	// Clicks on the "Undo" button in the toast and waits for the toast to disappear.
	if err := uiauto.Combine(
		"Click on 'Undo' to dismiss the toast",
		ac.LeftClick(undoButton),
		ac.WaitUntilGone(nodewith.ClassName("ToastOverlay")),
	)(ctx); err != nil {
		s.Fatal("Failed to dismiss the CloseAll toast: ", err)
	}

	// There should still be 3 windows since "Close All" action was canceled.
	wc, err := ash.CountVisibleWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to count windows: ", err)
	}

	if wc != 3 {
		s.Fatalf("Unexpected number of windows: got %v, want 0", wc)
	}

	// There should be two desks remaining.
	dc, err = ash.GetDeskCount(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to count desks: ", err)
	}

	if dc != 2 {
		s.Fatalf("Unexpected number of desks: got %v, want 2", dc)
	}
}
