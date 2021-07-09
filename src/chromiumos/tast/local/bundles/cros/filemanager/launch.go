// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Launch,
		Desc: "Verify Files app opens as a single window",
		Contacts: []string{
			"benreich@chromium.org",
			"chromeos-files-syd@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

func Launch(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test API Connection: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "multiple_windows")

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	if err := launcher.SearchAndWaitForAppOpen(tconn, kb, apps.Files)(ctx); err != nil {
		s.Fatal("Failed to open Files app: ", err)
	}

	// Verify only one Files app window was opened.
	if _, err := ash.FindOnlyWindow(ctx, tconn, func(w *ash.Window) bool {
		return w.Title == "Files"
	}); errors.Is(err, ash.ErrMultipleWindowsFound) {
		s.Fatal("Failed due to multiple Files app windows: ", err)
	} else if err != nil {
		s.Fatal("Failed to find only Files app window: ", err)
	}
}
