// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package familylink

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/wallpaper"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DefaultChildWallpaper,
		Desc: "Verifies Unicorn users can change the wallpaper and sync",
		Contacts: []string{
			"tobyhuang@chromium.org",
			"cros-families-eng+test@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Fixture:      "familyLinkUnicornLogin",
	})
}

func DefaultChildWallpaper(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*familylink.FixtData).TestConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("Waiting for Deep Purple wallpaper to sync")
	for attempts := 1; ; attempts++ {
		if err := wallpaper.OpenWallpaper(ctx, tconn); err != nil {
			s.Fatal("Failed to open the wallpaper picker: ", err)
		}
		// Wait until the wallpaper turns to "Deep Purple" through Chrome sync.
		if err := wallpaper.CheckWallpaper(ctx, tconn, "Deep Purple"); err == nil {
			s.Log("Successfully synced Deep Purple wallpaper")
			break
		}
		s.Logf("%d attempts to sync wallpaper failed, trying again", attempts)
		if err := wallpaper.CloseWallpaper(ctx, tconn); err != nil {
			s.Fatal("Failed to close the wallpaper picker: ", err)
		}
	}

	s.Log("Changing wallpaper to Imaginary")
	if err := wallpaper.ChangeWallpaper(ctx, tconn, "Imaginary", "Next Level!"); err != nil {
		s.Fatal("Failed to change the wallpaper to Imaginary Next Level!: ", err)
	}
	if err := wallpaper.CheckWallpaper(ctx, tconn, "Next Level!"); err != nil {
		s.Fatal("Failed to verify the wallpaper changed to Imaginary Next Level!: ", err)
	}

	s.Log("Changing wallpaper back to Deep Purple for the next test")
	if err := wallpaper.ChangeWallpaper(ctx, tconn, "Solid colors", "Deep Purple"); err != nil {
		s.Fatal("Failed to change the wallpaper to Solid colors Deep Purple: ", err)
	}
	if err := wallpaper.CheckWallpaper(ctx, tconn, "Deep Purple"); err != nil {
		s.Fatal("Failed to verify the wallpaper changed to Solid colors Deep Purple: ", err)
	}
}
