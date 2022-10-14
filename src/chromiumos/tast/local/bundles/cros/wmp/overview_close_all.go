// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
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
		Func:         OverviewCloseAll,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that windows and desk can be closed",
		Contacts: []string{
			"yzd@chromium.org",
			"chromeos-wmp@google.com",
			"cros-commercial-productivity-eng@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Params: []testing.Param{{
			Val: browser.TypeAsh,
		}, {
			Name:              "lacros",
			Val:               browser.TypeLacros,
			ExtraSoftwareDeps: []string{"lacros"},
		}},
		Timeout: chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
		SearchFlags: []*testing.StringPair{{
			Key: "feature_id",
			// Close the desk and its windows after task completion.
			Value: "screenplay-a8ad8d9d-ed34-48e1-a984-25570a099f75",
		}},
		VarDeps: []string{"ui.gaiaPoolDefault"},
	})
}

func OverviewCloseAll(ctx context.Context, s *testing.State) {
	// Reserves five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	bt := s.Param().(browser.Type)
	cr, _, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, bt, lacrosfixt.NewConfig(),
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.EnableFeatures("DesksCloseAll"),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
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

	// Opens PlayStore, Chrome and Files. As mentioned above, if we are in
	// lacros-chrome we will already have a chrome window, so if that is the case
	// then we can skip opening another browser window.
	for _, app := range []apps.App{apps.PlayStore, apps.Chrome, apps.FilesSWA} {
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

	// There should be no visible windows at this point.
	wc, err := ash.CountVisibleWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to count visible windows: ", err)
	}
	if wc != 0 {
		s.Fatalf("Unexpected number of visible windows: got %v, want 0", wc)
	}

	// Exits overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		s.Fatal("Failed to exit overview mode: ", err)
	}

	// Waits for the CloseAll toast to show up and disappear.
	if err := uiauto.Combine(
		"Wait for CloseAll toast",
		ac.WaitUntilExists(nodewith.ClassName("ToastOverlay")),
		ac.WaitUntilGone(nodewith.ClassName("ToastOverlay")),
	)(ctx); err != nil {
		s.Fatal("Failed to find CloseAll toast: ", err)
	}

	// We force close windows with a posted task 1 second after the toast's
	// destruction starts to close them asynchronously, so the windows may still
	// be closing after 1 second. To account for this possible delay, we give the
	// windows 2 seconds to fully close in case force closing a window takes extra
	// time.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// There should be 0 existing windows after one second.
		ws, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to count windows"))
		}
		if len(ws) != 0 {
			return errors.Errorf("unexpected number of windows: got %v, want 0", len(ws))
		}

		return nil
	}, &testing.PollOptions{
		Timeout:  2 * time.Second,
		Interval: 100 * time.Millisecond,
	}); err != nil {
		s.Fatal("Did not reach expected state: ", err)
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
