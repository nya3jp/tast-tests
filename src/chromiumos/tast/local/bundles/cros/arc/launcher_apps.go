// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LauncherApps,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "A functional test that checks if installed app appears in Launcher",
		Contacts:     []string{"cpiao@google.com", "arc-core@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "chrome"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm", "chrome"},
		}},
		Timeout: chrome.GAIALoginTimeout + arc.BootTimeout + 10*time.Minute,
		VarDeps: []string{"ui.gaiaPoolDefault"},
	})
}

func LauncherApps(ctx context.Context, s *testing.State) {
	const (
		pkgName = "com.google.android.apps.dynamite"
	)

	// Setup Chrome.
	cr, err := chrome.New(
		ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store and Close: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	// Install app.
	s.Log("Installing app")
	if err := playstore.InstallApp(ctx, a, d, pkgName, &playstore.Options{}); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	// Check the newly downloaded app in Launcher.
	// TODO(b/210702593): Replace with LaunchAndWaitForAppOpen once fixed.
	if err := launcher.LaunchApp(tconn, apps.Chat.Name)(ctx); err != nil {
		s.Fatal("Failed to launch: ", err)
	}

	ui := uiauto.New(tconn)
	chatButton := nodewith.Name(apps.Chat.Name).ClassName("ash/ShelfAppButton")
	if err := ui.WaitUntilExists(chatButton)(ctx); err != nil {
		s.Fatal("Failed to find Google Chat in Shelf: ", err)
	}

	if err := apps.Close(ctx, tconn, apps.Chat.ID); err != nil {
		s.Fatal("Failed to close: ", err)
	}

	// Turn off the Play Store
	if err := optin.SetPlayStoreEnabled(ctx, tconn, false); err != nil {
		s.Fatal("Failed to Turn Off Play Store: ", err)
	}

	// Verify Play Store is Off
	playStoreState, err := optin.GetPlayStoreState(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to check Google PlayStore State: ", err)
	}
	if playStoreState["enabled"] == true {
		s.Fatal("Playstore Still Enabled")
	}

	// Verify the app icon is not visible in Launcher and the app fails to launch.
	if err := launcher.LaunchApp(tconn, apps.Chat.Name)(ctx); err == nil {
		s.Fatal("Installed app remained in launcher after play store disabled")
	}
}
