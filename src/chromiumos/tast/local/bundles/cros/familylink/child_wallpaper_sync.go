// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package familylink

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/wallpaper"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChildWallpaperSync,
		Desc: "Verifies Unicorn users can sync the wallpaper",
		Contacts: []string{
			"tobyhuang@chromium.org",
			"cros-families-eng+test@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		VarDeps:      []string{"unicorn.wallpaperName"},
		Fixture:      "familyLinkUnicornLogin",
	})
}

func ChildWallpaperSync(outerCtx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*familylink.FixtData).TestConn

	defer faillog.DumpUITreeOnError(outerCtx, s.OutDir(), s.HasError, tconn)

	wallpaperName := s.RequiredVar("unicorn.wallpaperName")
	ui := uiauto.New(tconn)

	if err := wallpaper.OpenWallpaperPicker(ui)(outerCtx); err != nil {
		s.Fatal("Failed to open the wallpaper picker: ", err)
	}
	defer func() {
		if err := wallpaper.CloseWallpaperPicker(ui)(outerCtx); err != nil {
			s.Fatal("Failed to close the wallpaper picker: ", err)
		}
	}()

	// The wallpaper can take a while to sync so loop checking for the wallpaper.
	if err := testing.Poll(outerCtx, func(innerCtx context.Context) error {
		s.Logf("Waiting for %s wallpaper to sync", wallpaperName)
		// Wait until the wallpaper turns to "Imaginary Next Level!" through Chrome sync.
		if err := wallpaper.ValidateWallpaperName(ui, wallpaperName)(innerCtx); err != nil {
			return errors.Wrapf(err, "failed to sync %s wallpaper for Unicorn user", wallpaperName)
		}
		s.Logf("Successfully synced %s wallpaper", wallpaperName)
		return nil
	}, &testing.PollOptions{Timeout: 4 * time.Minute}); err != nil {
		s.Fatal("Failed to sync wallpaper for Unicorn user: ", err)
	}
}
