// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wallpaper

import (
	"context"
	"image/color"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/wallpaper"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SwitchOnlineWallpapers,
		Desc: "Test quickly switching online wallpapers in the new wallpaper app",
		Contacts: []string{
			"jasontt@google.com",
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Fixture:      "chromeLoggedIn",
	})
}

// SwitchOnlineWallpapers tests the flow of rapidly switching online wallpapers from the same
// collection and make sure the correct one is displayed.
func SwitchOnlineWallpapers(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Force Chrome to be in clamshell mode to make sure wallpaper view is clearly
	// visible for us to compare it with the given rgba color.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure DUT is not in tablet mode: ", err)
	}
	defer cleanup(ctx)

	// The test has a dependency of network speed, so we give uiauto.Context ample time to
	// wait for nodes to load.
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	if err := wallpaper.OpenWallpaperPicker(ui)(ctx); err != nil {
		s.Fatal("Failed to open wallpaper picker: ", err)
	}

	const collection = "Solid colors"
	if err := wallpaper.SelectCollection(ui, collection)(ctx); err != nil {
		s.Fatalf("Failed to select collection %q: %v", collection, err)
	}

	// Make sure yellow is last in the slice. We will be comparing the background wallpaper
	// with the given rgba color.
	for _, image := range []string{"Light Blue", "Google Green", "Google Yellow", "Yellow"} {
		if err := wallpaper.SelectImage(ui, image)(ctx); err != nil {
			s.Fatalf("Failed to select image %q: %v", image, err)
		}
	}

	if err := wallpaper.MinimizeWallpaperPicker(ui)(ctx); err != nil {
		s.Fatal("Failed to minimize wallpaper picker: ", err)
	}

	yellow := color.RGBA{255, 235, 60, 255}
	const expectedPercent = 90
	if err := wallpaper.ValidateBackground(ctx, cr, yellow, expectedPercent); err != nil {
		s.Error("Failed to validate wallpaper background: ", err)
	}
}
