// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package familylink

import (
	"context"
	"time"

	"chromiumos/tast/errors"
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
		VarDeps:      []string{"unicorn.syncWallpaperCategory", "unicorn.syncWallpaperName", "unicorn.changeWallpaperCategory", "unicorn.changeWallpaperName"},
		Fixture:      "familyLinkUnicornLogin",
	})
}

func DefaultChildWallpaper(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*familylink.FixtData).TestConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	success := false
	finalError := errors.New("failed to sync wallpaper")

	defer func() {
		if !success {
			s.Fatal("Failed to sync wallpaper for Unicorn user: ", finalError)
		}
	}()

	syncWallpaperCategory := s.RequiredVar("unicorn.syncWallpaperCategory")
	syncWallpaperName := s.RequiredVar("unicorn.syncWallpaperName")
	// The wallpaper can take a while to sync so loop checking
	// for the wallpaper.
	s.Logf("Waiting for %s %s wallpaper to sync", syncWallpaperCategory, syncWallpaperName)
	for attempts := 1; attempts <= 20; attempts++ {
		// We need to keep closing and re-opening the wallpaper picker to detect when the text changes.
		if err := wallpaper.OpenWallpaper(ctx, tconn); err != nil {
			s.Fatal("Failed to open the wallpaper picker: ", err)
		}
		// Wait until the wallpaper turns to "Deep Purple" through Chrome sync.
		if err := wallpaper.CheckWallpaper(ctx, tconn, syncWallpaperName); err != nil {
			s.Logf("%d attempts to sync wallpaper failed, trying again", attempts)
			finalError = errors.Wrapf(err, "failed to sync wallpaper after %d attempts", attempts)
		} else {
			s.Logf("Successfully synced %s %s wallpaper", syncWallpaperCategory, syncWallpaperName)
			success = true
			break
		}
		if err := wallpaper.CloseWallpaper(ctx, tconn); err != nil {
			s.Fatal("Failed to close the wallpaper picker: ", err)
		}
	}

	changeWallpaperCategory := s.RequiredVar("unicorn.changeWallpaperCategory")
	changeWallpaperName := s.RequiredVar("unicorn.changeWallpaperName")
	s.Logf("Changing wallpaper to %s %s", changeWallpaperCategory, changeWallpaperName)
	if err := wallpaper.ChangeWallpaper(ctx, tconn, changeWallpaperCategory, changeWallpaperName); err != nil {
		s.Fatalf("Failed to change the wallpaper to %s %s: %v", changeWallpaperCategory, changeWallpaperName, err)
	}

	s.Logf("Changing wallpaper back to %s %s for the next test", syncWallpaperCategory, syncWallpaperName)
	if err := wallpaper.ChangeWallpaper(ctx, tconn, syncWallpaperCategory, syncWallpaperName); err != nil {
		s.Fatalf("Failed to change the wallpaper to %s %s: %v", syncWallpaperCategory, syncWallpaperName, err)
	}
}
