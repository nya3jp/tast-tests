// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
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
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DesksTemplatesBasic,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks desks can be saved as a desk template",
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

func DesksTemplatesBasic(ctx context.Context, s *testing.State) {
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.EnableFeatures("DesksTemplates", "EnableSavedDesks"),
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

	// Setup for launching ARC apps.
	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store and Close: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(cleanupCtx)

	if err := a.WaitIntentHelper(ctx); err != nil {
		s.Fatal("Failed to wait for ARC Intent Helper: ", err)
	}

	// Open PlayStore, Chrome and Files.
	for _, app := range []apps.App{apps.PlayStore, apps.Chrome, apps.Files} {
		if err := apps.Launch(ctx, tconn, app.ID); err != nil {
			s.Fatalf("Failed to open %s: %v", app.Name, err)
		}
		if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
			s.Fatalf("%s did not appear in shelf after launch: %s", app.Name, err)
		}
	}

	if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		s.Fatal("Failed to wait for app launch events to be completed: ", err)
	}

	// Define keyboard to perform keyboard shortcuts.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Cannot create keyboard: ", err)
	}
	defer kb.Close()

	// Enter overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}
	defer ash.SetOverviewModeAndWait(cleanupCtx, tconn, false)

	// Find the save desk buttons and the grid views.
	saveDeskAsTemplateButton := nodewith.ClassName("SaveDeskTemplateButton").Nth(0)
	savedTemplateGridView := nodewith.ClassName("SavedDeskGridView").Nth(0)
	saveDeskForLaterButton := nodewith.ClassName("SaveDeskTemplateButton").Nth(1)
	savedForLaterDeskGridView := nodewith.ClassName("SavedDeskGridView").Nth(1)

	if err := uiauto.Combine(
		"save a desk template",
		ac.LeftClick(saveDeskAsTemplateButton),
		// Wait for the template grid to show up.
		ac.WaitUntilExists(savedTemplateGridView),
	)(ctx); err != nil {
		s.Fatal("Failed to save a desk template: ", err)
	}

	// Type "Template 1" and press "Enter".
	if err := kb.Type(ctx, "Template 1"); err != nil {
		s.Fatal("Cannot type 'Template 1': ", err)
	}
	if err := kb.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Cannot press 'Enter': ", err)
	}

	// Exit overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		s.Fatal("Failed to exit overview mode: ", err)
	}

	// Re-enter overview mode, so we can save a desk for later.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}

	// Save a desk for later.
	if err := uiauto.Combine(
		"save a desk for later",
		ac.LeftClick(saveDeskForLaterButton),
		// Wait for the saved for later grid to show up.
		ac.WaitUntilExists(savedForLaterDeskGridView),
	)(ctx); err != nil {
		s.Fatal("Failed to save a desk for later: ", err)
	}

	// Type "Saved Desk 1" and press "Enter".
	if err := kb.Type(ctx, "Saved Desk 1"); err != nil {
		s.Fatal("Cannot type 'Saved Desk 1': ", err)
	}
	if err := kb.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Cannot press 'Enter': ", err)
	}

	// Verify that there are two saved desks.
	savedDeskViewInfo, err := ash.FindDeskTemplates(ctx, ac)
	if err != nil {
		s.Fatal("Failed to find saved desks: ", err)
	}
	if len(savedDeskViewInfo) != 2 {
		s.Fatalf("Found inconsistent number of desks(s): got %v, want 2", len(savedDeskViewInfo))
	}

	// Exit overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		s.Fatal("Failed to exit overview mode: ", err)
	}

	// Verify that the app windows are closed.
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to count windows: ", err)
	}
	if len(ws) != 0 {
		for _, window := range ws {
			s.Log("Found window ", window)
		}
		s.Fatalf("Found inconsistent number of window(s): got %v, want 0", len(ws))
	}
}
