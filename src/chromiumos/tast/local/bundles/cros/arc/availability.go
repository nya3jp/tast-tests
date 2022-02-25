// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Availability,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Verifies that ARC is available in different scenarios",
		Contacts:     []string{"timkovich@chromium.org", "arc-eng@google.com"},
		SoftwareDeps: []string{"chrome"},
		// TODO(http://b/172011479): Test is disabled until it can be fixed
		// Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"ui.gaiaPoolDefault"},
	})
}

// isPlayStoreOpen checks that the Play Store icon is visible and that the apps search bar is available.
func isPlayStoreOpen(ctx context.Context, s *testing.State, d *ui.Device, tconn *chrome.TestConn) {
	if err := ash.WaitForApp(ctx, tconn, apps.PlayStore.ID, time.Minute); err != nil {
		s.Fatal("Play Store failed to open: ", err)
	}
	noThanksButton := d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches("(?i)No thanks"))
	searchBar := d.Object(ui.TextStartsWith("Search for apps"))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Click no thanks to close the pop-up if it exists.
		if err := noThanksButton.Exists(ctx); err == nil {
			noThanksButton.Click(ctx)
		}
		return searchBar.Exists(ctx)
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Fatal("Timed waiting for Play Store table of contents: ", err)
	}
}

// updateApp updates pkgName, if possible.
func updateApp(ctx context.Context, s *testing.State, a *arc.ARC, d *ui.Device, pkgName string) {
	if err := a.SendIntentCommand(ctx, "android.intent.action.VIEW", "market://details?id="+pkgName).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to send intent to open the Play Store: ", err)
	}

	updateBtn := d.Object(ui.Text("Update"), ui.ClassName("android.widget.Button"))
	if err := updateBtn.WaitForExists(ctx, 10*time.Second); err != nil {
		s.Log(pkgName, " is already up-to-date: ", err)
		return
	}

	if err := updateBtn.Click(ctx); err != nil {
		s.Error("Failed to click update: ", err)
	}

	// The update is finished when the "Deactivate" button appears.
	deactivateBtn := d.Object(ui.Text("Deactivate"), ui.ClassName("android.widget.Button"))
	if err := deactivateBtn.WaitForExists(ctx, 300*time.Second); err != nil {
		s.Fatal("Timed out updating app: ", err)
	}
}

// reopenPlayStore closes and reopens the Play Store so it goes back to the table of contents page.
func reopenPlayStore(ctx context.Context, s *testing.State, a *arc.ARC, d *ui.Device, tconn *chrome.TestConn) {
	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Log("Failed to close app: ", err)
	}

	if err := a.SendIntentCommand(ctx, "android.intent.action.VIEW", "https://play.google.com/store").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to send intent to open the Play Store: ", err)
	}

	openWith := d.Object(ui.Text("Play Store"))
	if err := openWith.WaitForExists(ctx, 10*time.Second); err != nil {
		// If we didn't get a prompt, the Play Store *might* be open, so keep going anyway.
		return
	}

	if err := openWith.Click(ctx); err != nil {
		s.Error("Failed to click 'Open with Play Store': ", err)
	}

	alwaysLink := d.Object(ui.Text("ALWAYS"))
	if err := alwaysLink.Click(ctx); err != nil {
		s.Error("Failed to click 'Always': ", err)
	}
}

// Availability Ensures that ARC is available after:
// * Login
// * Logout/Login
// * Updating GMS Core
// * Updating Play Store
func Availability(ctx context.Context, s *testing.State) {
	dumpUIOnErr := func(ctx context.Context, a *arc.ARC) {
		if s.HasError() {
			if err := a.Command(ctx, "uiautomator", "dump").Run(testexec.DumpLogOnError); err != nil {
				s.Error("Failed to dump UIAutomator: ", err)
			}
			if err := a.PullFile(ctx, "/sdcard/window_dump.xml", filepath.Join(s.OutDir(), "uiautomator_dump.xml")); err != nil {
				s.Error("Failed to pull UIAutomator dump: ", err)
			}
		}
	}

	var creds chrome.Creds

	func() {
		cr, err := chrome.New(ctx,
			chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
			chrome.ARCSupported(),
			chrome.ExtraArgs(arc.DisableSyncFlags()...))
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
		defer cr.Close(ctx)

		creds = cr.Creds()

		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to create test API connection: ", err)
		}

		// Optin to Play Store.
		s.Log("Opting into Play Store")
		maxAttempts := 2
		if err = optin.PerformWithRetry(ctx, cr, maxAttempts); err != nil {
			s.Fatal("Failed to optin to Play Store: ", err)
		}
		if err = optin.WaitForPlayStoreShown(ctx, tconn, time.Minute); err != nil {
			s.Fatal("Failed to wait for Play Store: ", err)
		}

		// Setup ARC.
		a, err := arc.New(ctx, s.OutDir())
		if err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}
		defer a.Close(ctx)
		defer dumpUIOnErr(ctx, a)

		d, err := a.NewUIDevice(ctx)
		if err != nil {
			s.Fatal("Failed initializing UI Automator: ", err)
		}
		defer d.Close(ctx)

		isPlayStoreOpen(ctx, s, d, tconn)
	}()

	cr, err := chrome.New(
		ctx,
		chrome.KeepState(),
		chrome.GAIALogin(creds),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)
	defer dumpUIOnErr(ctx, a)

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	s.Log("Opening the Play Store after restart")
	reopenPlayStore(ctx, s, a, d, tconn)
	isPlayStoreOpen(ctx, s, d, tconn)

	appsToUpdate := []string{
		"com.google.android.gms",
		"com.android.vending",
	}

	for _, app := range appsToUpdate {
		updateApp(ctx, s, a, d, app)
		reopenPlayStore(ctx, s, a, d, tconn)
		isPlayStoreOpen(ctx, s, d, tconn)
	}
}
