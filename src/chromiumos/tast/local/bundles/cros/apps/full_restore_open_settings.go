// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/browser"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FullRestoreOpenSettings,
		Desc: "Test full restore notification and browser",
		Contacts: []string{
			"nancylingwang@google.com",
			"chromeos-apps-foundation-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"ui.gaiaPoolDefault"},
		SoftwareDeps: []string{"chrome"},
	})
}

func FullRestoreOpenSettings(ctx context.Context, s *testing.State) {
	func() {
		cr, err := chrome.New(ctx, chrome.EnableFeatures("FullRestore"))
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
		defer cr.Close(ctx)

		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to connect Test API: ", err)
		}

		// Open browser.
		// The opened browser is not closed before reboot so that it could be restored after reboot.
		_, err = browser.Launch(ctx, tconn, cr, "https://abc.xyz")
		if err != nil {
			s.Fatal("Failed to launch browser: ", err)
		}

		// According to the PRD of Full Restore go/chrome-os-full-restore-dd,
		// it uses a throttle of 2.5s to save the app launching and window statue information to the backend.
		// Therefore, sleep 3 seconds here.
		testing.Sleep(ctx, 3*time.Second)

	}()

	func() {
		cr, err := chrome.New(ctx,
			// Set not to clear the notification after restore.
			// By default, On startup is set to ask every time after reboot
			// and there is an alertdialog asking the user to select whether to restore or not.
			chrome.RemoveNotification(false),
			chrome.EnableFeatures("FullRestore"),
			chrome.EnableRestoreTabs(),
			chrome.KeepState())
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
		defer cr.Close(ctx)

		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to connect Test API: ", err)
		}

		alertDialog := nodewith.NameStartingWith("Restore apps?").Role(role.AlertDialog)
		settingsButton := nodewith.Name("SETTINGS").Role(role.Button).Ancestor(alertDialog)

		ui := uiauto.New(tconn)
		defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

		if err := ui.LeftClick(settingsButton)(ctx); err != nil {
			s.Fatal("Failed to click notification SETTINGS button: ", err)
		}

		if err := ui.WaitUntilGone(alertDialog)(ctx); err != nil {
			s.Fatal("Failed to wait for notification to disappear: ", err)
		}

		if err := ash.WaitForApp(ctx, tconn, apps.Settings.ID, time.Minute); err != nil {
			s.Fatal("Settings app did not appear in the shelf: ", err)
		}

		// Confirm that the Settings apps page is open.
		appsPage := nodewith.Name("Apps").Role(role.Heading).Ancestor(ossettings.WindowFinder)
		if err := uiauto.New(tconn).WaitUntilExists(appsPage)(ctx); err != nil {
			s.Fatal("Failed to wait for Settings apps page: ", err)
		}

		// After clicking the SETTINGS button, the notification is hidden in the system tray.
		// Show the quick settings to find the notification.
		if err := quicksettings.Show(ctx, tconn); err != nil {
			s.Fatal("Failed to wait for quick setting: ", err)
		}

		restoreButton := nodewith.Name("RESTORE").Role(role.Button)
		if err := ui.LeftClick(restoreButton)(ctx); err != nil {
			s.Fatal("Failed to click notification RESTORE button: ", err)
		}

		if err := ui.WaitUntilGone(restoreButton)(ctx); err != nil {
			s.Fatal("Failed to wait for notification to close: ", err)
		}

		if _, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool { return w.WindowType == ash.WindowTypeBrowser }); err != nil {
			s.Fatal("Failed to restore browser: ", err)
		}
	}()
}
