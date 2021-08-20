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
		Func: ChildWallpaperSync,
		Desc: "Verifies Unicorn users can sync the wallpaper",
		Contacts: []string{
			"tobyhuang@chromium.org",
			"cros-families-eng+test@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		VarDeps:      []string{"unicorn.wallpaperCategory", "unicorn.wallpaperName"},
		Fixture:      "familyLinkUnicornLogin",
	})
}

func ChildWallpaperSync(outerCtx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*familylink.FixtData).TestConn

	defer faillog.DumpUITreeOnError(outerCtx, s.OutDir(), s.HasError, tconn)

	wallpaperCategory := s.RequiredVar("unicorn.wallpaperCategory")
	wallpaperName := s.RequiredVar("unicorn.wallpaperName")
	// The wallpaper can take a while to sync so loop checking for the wallpaper.
	if err := testing.Poll(outerCtx, func(innerCtx context.Context) error {
		s.Logf("Waiting for %s %s wallpaper to sync", wallpaperCategory, wallpaperName)
		// We need to keep closing and re-opening the wallpaper picker to detect when the text changes.
		if err := wallpaper.OpenWallpaper(innerCtx, tconn); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to open the wallpaper picker"))
		}
		defer func() {
			if err := wallpaper.CloseWallpaper(outerCtx, tconn); err != nil {
				s.Fatal("Failed to close the wallpaper picker: ", err)
			}
		}()
		// Wait until the wallpaper turns to "Imaginary Next Level!" through Chrome sync.
		if err := wallpaper.CheckWallpaper(innerCtx, tconn, wallpaperName); err != nil {
			return errors.Wrapf(err, "failed to sync %s %s wallpaper for Unicorn user", wallpaperCategory, wallpaperName)
		}
		s.Logf("Successfully synced %s %s wallpaper", wallpaperCategory, wallpaperName)
		return nil
	}, &testing.PollOptions{Timeout: 4 * time.Minute}); err != nil {
		s.Fatal("Failed to sync wallpaper for Unicorn user: ", err)
	}
}
