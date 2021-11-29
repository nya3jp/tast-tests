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
		Func:         Change,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Follows the user flow to change the wallpaper",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"wallpaper.category", "wallpaper.name"},
	})
}

func Change(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.EnableWallpaperSWA(false))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := wallpaper.OpenWallpaperDeprecated(ctx, tconn); err != nil {
		s.Fatal("Failed to open the wallpaper picker: ", err)
	}

	category := s.RequiredVar("wallpaper.category")
	name := s.RequiredVar("wallpaper.name")
	if err := wallpaper.ChangeWallpaperDeprecated(ctx, tconn, category, name); err != nil {
		s.Fatalf("Failed to change the wallpaper to %s %s: %v", category, name, err)
	}
}
