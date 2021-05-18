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
		Timeout:      7 * time.Minute,
	})
}

func FullRestoreFilesappReboot(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.RemoveNotification(false),
		chrome.EnableFeatures("FullRestore"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Open the Files app.
	_, err = filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch Files app: ", err)
	}

	// Need to sleep at least 2.5 seconds to make sure the opened app is captured by the full restore feature.
	testing.Sleep(ctx, 3*time.Second)

	cr, err = chrome.New(ctx,
		chrome.GAIALogin(cr.Creds()),
		chrome.RemoveNotification(false),
		chrome.EnableFeatures("FullRestore"),
		chrome.KeepState())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	tconn, err = cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	//alertDialog := nodewith.NameStartingWith("Restore apps ").Role(role.AlertDialog)
	restoreButton := nodewith.Name("RESTORE").Role(role.Button)
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
}
