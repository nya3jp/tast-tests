// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
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
	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ExtraArgs(`--feature-flags=["full-restore@1"]`))
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

	// cr, err = chrome.New(ctx,
	// 	chrome.GAIALogin(cr.Creds()),
	// 	chrome.KeepState())
	// if err != nil {
	// 	s.Fatal("Failed to start Chrome: ", err)
	// }

	// tconn, err = cr.TestAPIConn(ctx)
	// if err != nil {
	// 	s.Fatal("Failed to connect Test API: ", err)
	// }

	// // Check Files app is restored.
	// _, err = filesapp.App(ctx, tconn)
	// if err != nil {
	// 	s.Fatal("Failed to find Files app after reboot with full restore: ", err)
	// }
}
