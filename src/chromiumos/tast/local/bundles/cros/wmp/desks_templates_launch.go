// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DesksTemplatesLaunch,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks desks templates can be launched",
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

func DesksTemplatesLaunch(ctx context.Context, s *testing.State) {
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.EnableFeatures("DesksTemplates"),
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

	// Opens PlayStore, Chrome and Files.
	appsList := []apps.App{apps.PlayStore, apps.Chrome, apps.Files}
	for _, app := range appsList {
		if err := apps.Launch(ctx, tconn, app.ID); err != nil {
			s.Fatalf("Failed to open %s: %v", app.Name, err)
		}
		if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
			s.Fatalf("%s did not appear in shelf after launch: %s", app.Name, err)
		}
	}

	if err := ac.WaitForLocation(nodewith.Root())(ctx); err != nil {
		s.Fatal("Failed to wait for app launch events to be completed: ", err)
	}

	// Enters overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}

	if err := ac.WaitForLocation(nodewith.Root())(ctx); err != nil {
		s.Fatal("Failed to wait for overview animation to be completed: ", err)
	}

	// Find the "save desk as a template" button.
	saveDeskButton := nodewith.ClassName("SaveDeskTemplateButton")
	desksTemplatesGridView := nodewith.ClassName("DesksTemplatesGridView")

	if err := uiauto.Combine(
		"save a desk template",
		ac.LeftClick(saveDeskButton),
		// Wait for the desk templates grid shows up.
		ac.WaitUntilExists(desksTemplatesGridView),
	)(ctx); err != nil {
		s.Fatal("Failed to save a desk template: ", err)
	}

	// Exits overview mode.
	if err = ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		s.Fatal("Failed to exit overview mode: ", err)
	}

	// Close all existing windows.
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get all open windows: ", err)
	}
	for _, w := range ws {
		if err := w.CloseWindow(ctx, tconn); err != nil {
			s.Fatalf("Failed to close window (%+v): %v", w, err)
		}
	}

	// Enters overview mode, and launch the saved desk template.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}

	// Find the "Templates" button.
	templatesButton := nodewith.Name("Templates")

	// Show saved desk template.
	if err := uiauto.Combine(
		"show the saved desks template",
		ac.LeftClick(templatesButton),
		// Wait for the desks templates grid shows up.
		ac.WaitUntilExists(desksTemplatesGridView),
	)(ctx); err != nil {
		s.Fatal("Failed to show saved desks templates: ", err)
	}

	// Confirm there is one desk template.
	deskTemplatesInfo, err := ash.FindDeskTemplates(ctx, ac)
	if err != nil {
		s.Fatal("Failed to find desk templates: ", err)
	}
	if len(deskTemplatesInfo) != 1 {
		s.Fatalf("Got %v desk template(s), there should be one desk template", len(deskTemplatesInfo))
	}

	// Find the the first desk template.
	firstDeskTemplate := nodewith.ClassName("DesksTemplatesItemView")
	newDeskMiniView :=
		nodewith.ClassName("DeskMiniView").Name(fmt.Sprintf("Desk: %s", "Desk 1 (1)"))

	// Launch the saved desk template.
	if err := uiauto.Combine(
		"launch the saved desk template",
		ac.LeftClick(firstDeskTemplate),
		// Wait for the new desk to appear.
		ac.WaitUntilExists(newDeskMiniView),
	)(ctx); err != nil {
		s.Fatal("Failed to launch a desk template: ", err)
	}

	// Exits overview mode.
	if err = ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		s.Fatal("Failed to exit overview mode: ", err)
	}

	// Verifies that there are the app windows.
	ws, err = ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get all open windows: ", err)
	}

	if len(ws) != len(appsList) {
		s.Fatalf("Got %v window(s), should have %v windows", len(ws), len(appsList))
	}
}
