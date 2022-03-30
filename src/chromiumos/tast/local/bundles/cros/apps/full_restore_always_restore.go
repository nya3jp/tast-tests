// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FullRestoreAlwaysRestore,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Test full restore always restore setting",
		Contacts: []string{
			"nancylingwang@google.com",
			"chromeos-apps-foundation-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"ui.gaiaPoolDefault"},
		SoftwareDeps: []string{"chrome"},
	})
}

func FullRestoreAlwaysRestore(ctx context.Context, s *testing.State) {
	func() {
		bt := browser.TypeAsh
		cr, br, _, err := browserfixt.SetUpWithNewChrome(ctx,
			bt,
			nil,
			chrome.EnableFeatures("FullRestore"))
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
		conn, err := br.NewConn(ctx, "https://abc.xyz")
		if err != nil {
			s.Fatalf("Failed to connect to the restore URL: %v ", err)
		}
		defer conn.Close()

		// Open OS settings to set the 'Always restore' setting.
		if _, err = ossettings.LaunchAtPage(ctx, tconn, nodewith.Name("Apps").Role(role.Link)); err != nil {
			s.Fatal("Failed to launch Apps Settings: ", err)
		}

		if err := uiauto.Combine("set 'Always restore' Settings",
			uiauto.New(tconn).LeftClick(nodewith.Name("Restore apps on startup").Role(role.PopUpButton)),
			uiauto.New(tconn).LeftClick(nodewith.Name("Always restore").Role(role.ListBoxOption)))(ctx); err != nil {
			s.Fatal("Failed to set 'Always restore' Settings: ", err)
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

		defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

		// Confirm that the browser is restored.
		if _, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool { return w.WindowType == ash.WindowTypeBrowser }); err != nil {
			s.Fatal("Failed to restore browser: ", err)
		}

		// Confirm that the Settings app is restored.
		if err := uiauto.New(tconn).WaitUntilExists(ossettings.SearchBoxFinder)(ctx); err != nil {
			s.Fatal("Failed to restore the Settings app: ", err)
		}
	}()
}
