// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wallpaper

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/wallpaper"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Change,
		Desc: "Follows the user flow to change the wallpaper",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

func Change(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := wallpaper.OpenWallpaper(ctx, tconn); err != nil {
		s.Fatal("Failed to open the wallpaper picker: ", err)
	}

	if err := wallpaper.ChangeWallpaper(ctx, tconn, "Solid colors", "Deep Purple"); err != nil {
		s.Fatal("Failed to change the wallpaper: ", err)
	}

	// Ensure that "Deep Purple" text is displayed.
	if err := wallpaper.CheckWallpaper(ctx, tconn, "Deep Purple"); err != nil {
		s.Fatal("Failed to verify the wallpaper: ", err)
	}
}
