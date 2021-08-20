// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wallpaper

import (
	"context"

	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/wallpaper"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Child,
		Desc: "Follows the user flow to change the wallpaper for a child",
		Contacts: []string{
			"tobyhuang@chromium.org", "cros-families-eng+test@google.com", "chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"unicorn.wallpaperCategory", "unicorn.wallpaperName"},
		Fixture:      "familyLinkUnicornLogin",
	})
}

func Child(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*familylink.FixtData).TestConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := wallpaper.OpenWallpaper(ctx, tconn); err != nil {
		s.Fatal("Failed to open the wallpaper picker: ", err)
	}

	category := s.RequiredVar("unicorn.wallpaperCategory")
	name := s.RequiredVar("unicorn.wallpaperName")
	if err := wallpaper.ChangeWallpaper(ctx, tconn, category, name); err != nil {
		s.Fatalf("Failed to change the wallpaper to %s %s: %v", category, name, err)
	}
}
