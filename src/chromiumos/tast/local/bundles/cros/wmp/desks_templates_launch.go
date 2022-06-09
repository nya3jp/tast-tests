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
	"chromiumos/tast/local/bundles/cros/wmp/wmputils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
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
		chrome.EnableFeatures("DesksTemplates", "EnableSavedDesks"),
		chrome.DisableFeatures("DeskTemplateSync"),
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
	appsList := []apps.App{apps.PlayStore, apps.Chrome, apps.Files}
	if err := wmputils.OpenApps(ctx, tconn, appsList); err != nil {
		s.Fatal("Failed to open apps: ", err)
	}

	// Enter overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}
	if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		s.Fatal("Failed to wait for the animation to be completed: ", err)
	}
	defer ash.SetOverviewModeAndWait(ctx, tconn, false)

	// Save current desk as `Template 1` of type `kTemplate`.
	if err := ash.SaveCurrentDesk(ctx, ac, "kTemplate", "Template 1"); err != nil {
		s.Fatal("Failed to save current desk as 'Template 1' of type 'kTemplate': ", err)
	}

	// Verify saved desk.
	if err := ash.VerifySavedDesk(ctx, ac, []string{"Template 1"}); err != nil {
		s.Fatal("Failed to verify saved desk: ", err)
	}

	// Exit overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}

	// Verify window count.
	if err := wmputils.VerifyWindowCount(ctx, tconn, len(appsList)); err != nil {
		s.Fatal("Failed to verify window count: ", err)
	}

	// Enter overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}
	if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		s.Fatal("Failed to wait for the animation to be completed: ", err)
	}

	// Save current desk as `Saved Desk 1` of type `kSaveAndRecall`.
	if err := ash.SaveCurrentDesk(ctx, ac, "kSaveAndRecall", "Saved Desk 1"); err != nil {
		s.Fatal("Failed to save current desk as 'Saved Desk 1' of type 'kSaveAndRecall': ", err)
	}

	// Verify saved desk.
	if err := ash.VerifySavedDesk(ctx, ac, []string{"Template 1", "Saved Desk 1"}); err != nil {
		s.Fatal("Failed to verify saved desk: ", err)
	}

	// Exit overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}

	// Verify window count.
	if err := wmputils.VerifyWindowCount(ctx, tconn, 0); err != nil {
		s.Fatal("Failed to verify window count: ", err)
	}

	// Enter overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}
	if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		s.Fatal("Failed to wait for the animation to be completed: ", err)
	}

	// Enter library page.
	if err := ash.EnterLibraryPage(ctx, ac); err != nil {
		s.Fatal("Failed to enter library page: ", err)
	}

	// Launch saved desk `Template 1` of type `kTemplate`.
	if err := ash.LaunchSavedDesk(ctx, ac, "Template 1", 0); err != nil {
		s.Fatal("Failed to launch saved desk 'Template 1' of type 'kTemplate': ", err)
	}

	// Wait for app launch.
	if err := wmputils.WaitforAppLaunch(ctx, tconn, ac, appsList); err != nil {
		s.Fatal("Failed to wait for app launch: ", err)
	}

	// Exit overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}

	// Close Play Store.
	if err := optin.ClosePlayStore(ctx, tconn); err != nil {
		s.Fatal("Failed to close Play Store: ", err)
	}

	// Close all existing windows.
	if err := ash.CloseAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to close all windows: ", err)
	}

	// Remove the active desk.
	if err := ash.RemoveActiveDesk(ctx, tconn); err != nil {
		s.Fatal("Failed to remove the active desk: ", err)
	}

	// Enter overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}
	if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		s.Fatal("Failed to wait for the animation to be completed: ", err)
	}

	// Enter library page.
	if err := ash.EnterLibraryPage(ctx, ac); err != nil {
		s.Fatal("Failed to enter library page: ", err)
	}

	// Verify saved desk.
	if err := ash.VerifySavedDesk(ctx, ac, []string{"Template 1", "Saved Desk 1"}); err != nil {
		s.Fatal("Failed to verify saved desk: ", err)
	}

	// Launch saved desk `Saved Desk 1` of type `kSaveAndRecall`.
	if err := ash.LaunchSavedDesk(ctx, ac, "Saved Desk 1", 1); err != nil {
		s.Fatal("Failed to launch saved desk 'Saved Desk 1' of type 'kSaveAndRecall': ", err)
	}

	// Wait for app launch.
	if err := wmputils.WaitforAppLaunch(ctx, tconn, ac, appsList); err != nil {
		s.Fatal("Failed to wait for app launch: ", err)
	}

	// Exit overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}

	// Close Play Store.
	if err := optin.ClosePlayStore(ctx, tconn); err != nil {
		s.Fatal("Failed to close Play Store: ", err)
	}

	// Close all existing windows.
	if err := ash.CloseAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to close all windows: ", err)
	}

	// Remove the active desk.
	if err := ash.RemoveActiveDesk(ctx, tconn); err != nil {
		s.Fatal("Failed to remove the active desk: ", err)
	}

	// Enter overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}
	if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		s.Fatal("Failed to wait for the animation to be completed: ", err)
	}

	// Enter library page.
	if err := ash.EnterLibraryPage(ctx, ac); err != nil {
		s.Fatal("Failed to enter library page: ", err)
	}

	// Verify saved desk.
	if err := ash.VerifySavedDesk(ctx, ac, []string{"Template 1"}); err != nil {
		s.Fatal("Failed to verify saved desk: ", err)
	}

}
