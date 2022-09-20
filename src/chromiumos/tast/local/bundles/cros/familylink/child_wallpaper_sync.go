// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package familylink

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/wallpaper"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChildWallpaperSync,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies Unicorn users can sync the wallpaper",
		Contacts: []string{
			"tobyhuang@chromium.org",
			"cros-families-eng+test@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		VarDeps:      []string{"unicorn.wallpaperName"},
		Fixture:      "familyLinkUnicornLogin",
	})
}

func ChildWallpaperSync(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(familylink.HasTestConn).TestConn()

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	wallpaperName := s.RequiredVar("unicorn.wallpaperName")
	ui := uiauto.New(tconn)

	if err := wallpaper.OpenWallpaperPicker(ui)(ctx); err != nil {
		s.Fatal("Failed to open the wallpaper picker: ", err)
	}
	defer func(ctx context.Context) {
		if err := wallpaper.CloseWallpaperPicker()(ctx); err != nil {
			s.Fatal("Failed to close the wallpaper picker: ", err)
		}
	}(ctx)

	s.Logf("Waiting for wallpaper %q to sync", wallpaperName)

	// The wallpaper can take a while to sync so wait until it changes to the expected name.
	if err := wallpaper.WaitForWallpaperWithName(ui.WithPollOpts(testing.PollOptions{Timeout: 9 * time.Minute, Interval: 10 * time.Second}), wallpaperName)(ctx); err != nil {
		s.Fatal("Failed to sync wallpaper for Unicorn user: ", err)
	}
}
