// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
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
		Func:         OverviewCombineDesks,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that desks can be combined",
		Contacts: []string{
			"benbecker@chromium.org",
			"chromeos-wmp@google.com",
			"cros-commercial-productivity-eng@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		SearchFlags: []*testing.StringPair{{
			Key: "feature_id",
			// Close the desk.
			Value: "screenplay-ddf72b6c-7951-4336-9c83-e1db13628c16",
		}},
		Params: []testing.Param{{
			Val: browser.TypeAsh,
		}, {
			Name:              "lacros",
			Val:               browser.TypeLacros,
			ExtraSoftwareDeps: []string{"lacros"},
		}},
		Timeout: chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
		VarDeps: []string{"ui.gaiaPoolDefault"},
	})
}

func OverviewCombineDesks(ctx context.Context, s *testing.State) {
	// Reserves five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	bt := s.Param().(browser.Type)
	cr, _, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, bt, lacrosfixt.NewConfig(),
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.EnableFeatures("DesksCloseAll"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)
	defer closeBrowser(cleanupCtx)

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

	// Enters overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}
	defer ash.SetOverviewModeAndWait(cleanupCtx, tconn, false)
	defer ash.CleanUpDesks(cleanupCtx, tconn)

	newDeskButton := nodewith.ClassName("ZeroStateIconButton")
	desk2NameView := nodewith.ClassName("DeskNameView").Name("Desk 2")
	const desk2Name = "BusyDesk"
	if err := uiauto.Combine(
		"create a new desk by clicking new desk button",
		ac.DoDefault(newDeskButton),
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

	// If we are in lacros-chrome, then browserfixt.SetUp has already opened a
	// blank browser window in the first desk. In that case, we want to move the
	// already-existing browser window over to the second desk with a keyboard
	// shortcut and wait for the window to finish moving.
	if bt == browser.TypeLacros {
		if err := ash.MoveActiveWindowToAdjacentDesk(ctx, tconn, ash.WindowMovementDirectionRight); err != nil {
			s.Fatal("Failed to move lacros window to desk 2: ", err)
		}
	}

	// Activates the second desk and launch app windows on it.
	if err := ash.ActivateDeskAtIndex(ctx, tconn, 1); err != nil {
		s.Fatal("Failed to activate desk 2: ", err)
	}

	// Opens Chrome and Files. As mentioned above, if we are in lacros-chrome we
	// will already have a chrome window, so if that is the case then we can skip
	// opening another browser window.
	for _, app := range []apps.App{apps.Chrome, apps.FilesSWA} {
		if bt == browser.TypeLacros && app == apps.Chrome {
			continue
		}

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

	desk2DeskMiniView := nodewith.ClassName("DeskMiniView").Name("Desk: " + desk2Name)

	// Gets the location of the second desk mini view.
	desk2DeskMiniViewLoc, err := ac.Location(ctx, desk2DeskMiniView)
	if err != nil {
		s.Fatalf("Failed to get the mini view location of desk %s: %v", desk2Name, err)
	}

	// Moves the mouse to second desk mini view and hover.
	if err := mouse.Move(tconn, desk2DeskMiniViewLoc.CenterPoint(), 100*time.Millisecond)(ctx); err != nil {
		s.Fatal("Failed to hover at the second desk mini view: ", err)
	}

	// Finds the "Combine Desks" button.
	combineDesksButton := nodewith.ClassName("CloseButton").Name("Combine with Desk 1")

	// Combines the second desk with the first desk.
	if err := pc.Click(combineDesksButton)(ctx); err != nil {
		s.Fatal("Failed to combine the second desk with the first desk: ", err)
	}

	// Exits overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		s.Fatal("Failed to exit overview mode: ", err)
	}

	// There should still be 2 windows.
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to count windows: ", err)
	}

	if len(ws) != 2 {
		for _, window := range ws {
			s.Log("Found window ", window)
		}

		s.Fatalf("Unexpected number of windows: got %v, want 2", len(ws))
	}

	// There should be only one desk remaining.
	dc, err := ash.GetDeskCount(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to count desks: ", err)
	}

	if dc != 1 {
		s.Fatalf("Unexpected number of desks: got %v, want 1", dc)
	}
}
