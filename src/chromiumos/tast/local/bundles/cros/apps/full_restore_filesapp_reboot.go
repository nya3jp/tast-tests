// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FullRestoreFilesappReboot,
		Desc: "Test full restore files app",
		Contacts: []string{
			"jinrongwu@google.com",
			"chromeos-apps-foundation-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"ui.gaiaPoolDefault"},
		SoftwareDeps: []string{"chrome"},
	})
}

func FullRestoreFilesappReboot(ctx context.Context, s *testing.State) {
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

		// Open the Files app.
		// The opened Files app is not closed before reboot so that it could be restored after reboot.
		_, err = filesapp.Launch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to launch Files app: ", err)
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
		restoreButton := nodewith.Name("RESTORE").Role(role.Button).Ancestor(alertDialog)
		downloads := nodewith.Name(filesapp.Downloads).Role(role.TreeItem).Ancestor(filesapp.WindowFinder)

		ui := uiauto.New(tconn)
		defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

		if err := uiauto.Combine("restore files app",
			// Click Restore on the restore alert.
			ui.LeftClick(restoreButton),

			// Check Files app is restored.
			ui.WaitUntilExists(downloads))(ctx); err != nil {
			s.Fatal("Failed to restore Files app: ", err)
		}
	}()
}
