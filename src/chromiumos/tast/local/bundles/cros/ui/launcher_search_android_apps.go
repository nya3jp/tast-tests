// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
<<<<<<< HEAD   (166d58 Tast: Stop Assistant after assistant tests)
	"chromiumos/tast/local/bundles/cros/ui/faillog"
=======
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
>>>>>>> CHANGE (00bf32 tast-tests: use GAIA in ui.LauncherSearchAndroidApps)
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LauncherSearchAndroidApps,
		Desc: "Launches an Android app through the launcher",
		Contacts: []string{
			"bhansknecht@chromium.org",
			"dhaddock@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.Booted(),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBooted(),
		}},
		Vars: []string{"ui.LauncherSearchAndroidApps.username", "ui.LauncherSearchAndroidApps.password"},
	})
}

func LauncherSearchAndroidApps(ctx context.Context, s *testing.State) {
	username := s.RequiredVar("ui.LauncherSearchAndroidApps.username")
	password := s.RequiredVar("ui.LauncherSearchAndroidApps.password")

	args := []string{"--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off"}
	cr, err := chrome.New(ctx, chrome.GAIALogin(), chrome.Auth(username, password, "gaia-id"), chrome.ARCSupported(),
		chrome.ExtraArgs(args...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Optin to Play Store.
	s.Log("Opting into Play Store")
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store: ", err)
	}
	if err := optin.WaitForPlayStoreShown(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for Play Store: ", err)
	}
	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to close Play Store: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()
	if err := a.WaitIntentHelper(ctx); err != nil {
		s.Fatal("Failed to wait for ARC Intent Helper: ", err)
	}

<<<<<<< HEAD   (166d58 Tast: Stop Assistant after assistant tests)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s, tconn)

=======
>>>>>>> CHANGE (00bf32 tast-tests: use GAIA in ui.LauncherSearchAndroidApps)
	if err := launcher.SearchAndLaunch(ctx, tconn, apps.PlayStore.Name); err != nil {
		s.Fatal("Failed to launch Play Store: ", err)
	}

	if err := ash.WaitForApp(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to wait for Play Store: ", err)
	}
}
