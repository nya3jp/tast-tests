// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCInstallUninstallApp,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Platform should support ARC++ install/uninstall apps",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"android_vm", "chrome"},
		Timeout:      chrome.GAIALoginTimeout + arc.BootTimeout + 10*time.Minute,
		VarDeps:      []string{"ui.gaiaPoolDefault"},
	})
}

func ARCInstallUninstallApp(ctx context.Context, s *testing.State) {

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	pkgs := map[string]apps.App{
		"com.google.android.apps.dynamite":          apps.Chat,
		"com.google.android.apps.docs.editors.docs": apps.Docs}

	// Setup Chrome.
	cr, err := chrome.New(
		ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store and Close: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(cleanupCtx)

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(cleanupCtx)

	for pkg, app := range pkgs {
		s.Logf("Installing %s app", app.Name)
		if err := playstore.InstallApp(ctx, a, d, pkg, &playstore.Options{}); err != nil {
			s.Fatalf("Failed to install %s: %v ", app.Name, err)
		}

		// Check the newly downloaded app in Launcher.
		if err := launcher.LaunchApp(tconn, app.Name)(ctx); err != nil {
			s.Fatalf("Failed to launch %s: %v ", app.Name, err)
		}

		if err := apps.Close(ctx, tconn, app.ID); err != nil {
			s.Fatalf("Failed to close %s: %v ", app.Name, err)
		}
	}

	// Turn off the Play Store.
	if err := optin.SetPlayStoreEnabled(ctx, tconn, false); err != nil {
		s.Fatal("Failed to Turn Off Play Store: ", err)
	}

	// Verify Play Store is Off.
	playStoreState, err := optin.GetPlayStoreState(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to check Google PlayStore State: ", err)
	}
	if playStoreState["enabled"] == true {
		s.Fatal("Failed as Playstore is still enabled")
	}

	for _, app := range pkgs {
		// Verify the app icon is not visible in Launcher and the app fails to launch.
		if err := launcher.LaunchApp(tconn, app.Name)(ctx); err == nil {
			s.Fatal("Installed app remained in launcher after play store disabled: ", err)
		}
	}

	// Turn on the Play Store.
	s.Log("Performing optin to Play Store")
	maxAttempts := 2
	if err := optin.PerformWithRetry(ctx, cr, maxAttempts); err != nil {
		s.Fatal("Failed to optin to Play Store: ", err)
	}

	if err := optin.WaitForPlayStoreShown(ctx, tconn, time.Minute); err != nil {
		s.Fatal("Failed to wait for Play Store: ", err)
	}

	// Check for ARC notification after enabling Play Store.
	const notificationWaitTime = 30 * time.Second
	const notificationID = "ARC_NOTIFICATION"
	_, err = ash.WaitForNotification(ctx, tconn, notificationWaitTime, ash.WaitIDContains(notificationID))
	if err != nil {
		s.Error("Failed to find required notification: ", err)
	}

	// Clear any notifications that are currently displayed.
	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to clear notifications: ", err)
	}
}
