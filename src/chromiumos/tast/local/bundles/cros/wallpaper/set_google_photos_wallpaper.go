// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wallpaper

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/media/imgcmp"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/wallpaper"
	"chromiumos/tast/local/wallpaper/constants"
	"chromiumos/tast/testing"
)

type setGooglePhotosWallpaperParams struct {
	album string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         SetGooglePhotosWallpaper,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test setting Google Photos wallpapers in the wallpaper app",
		Contacts: []string{
			"xiaohuic@google.com",
			"assistive-eng@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps: []string{
			"wallpaper.googlePhotosAccountPool",
		},
		Timeout: 5 * time.Minute,
		Params: []testing.Param{{
			Name: "from_album",
			Val: setGooglePhotosWallpaperParams{
				album: constants.GooglePhotosWallpaperAlbum,
			},
		}, {
			Name: "from_photos",
			Val: setGooglePhotosWallpaperParams{
				album: "",
			},
		}},
	})
}

func SetGooglePhotosWallpaper(ctx context.Context, s *testing.State) {
	// Setting Google Photos wallpapers requires that Chrome be logged in with
	// a user from an account pool which has been preconditioned to have a
	// Google Photos library with specific photos/albums present.
	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("wallpaper.googlePhotosAccountPool")),
		chrome.EnableFeatures("WallpaperGooglePhotosIntegration"))
	if err != nil {
		s.Fatal("Failed to log in to Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Force Chrome to be in clamshell mode to make sure the wallpaper view is
	// clearly visible for us to compare it with an expected RGBA color.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure DUT is not in tablet mode: ", err)
	}
	defer cleanup(ctx)

	// The test has a dependency on network speed, so we give `uiauto.Context`
	// ample time to wait for nodes to load.
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	album := s.Param().(setGooglePhotosWallpaperParams).album

	if err := uiauto.Combine("Set a new wallpaper and minimize wallpaper picker",
		wallpaper.OpenWallpaperPicker(ui),
		wallpaper.SelectCollection(ui, constants.GooglePhotosWallpaperCollection),
		func(ctx context.Context) error {
			if len(album) == 0 {
				return nil
			}
			return uiauto.Combine("Select album",
				ui.LeftClick(nodewith.Name("Albums").Role(role.ToggleButton)),
				wallpaper.SelectGooglePhotosAlbum(ui, album),
			)(ctx)
		},
		wallpaper.SelectGooglePhotosPhoto(ui, constants.GooglePhotosWallpaperPhoto),
		wallpaper.MinimizeWallpaperPicker(ui),
	)(ctx); err != nil {
		s.Fatal("Failed to set new wallpaper: ", err)
	}

	// The expected percentage takes into account that the center cropped image is
	// similar to the filled one.
	const expectedPercent = 70
	if err := wallpaper.ValidateBackground(cr,
		constants.GooglePhotosWallpaperColor, expectedPercent)(ctx); err != nil {
		s.Error("Failed to validate wallpaper background: ", err)
	}

	// Take a screenshot of the current wallpaper.
	screenshot1, err := screenshot.GrabScreenshot(ctx, cr)
	if err != nil {
		s.Fatal("Failed to grab screenshot: ", err)
	}

	if err := uiauto.Combine("Choose new layout and minimize wallpaper picker",
		wallpaper.OpenWallpaperPicker(ui),
		wallpaper.SelectCollection(ui, constants.GooglePhotosWallpaperCollection),
		ui.LeftClick(nodewith.Name("Center").Role(role.ToggleButton)),
		wallpaper.MinimizeWallpaperPicker(ui),
	)(ctx); err != nil {
		s.Fatal("Failed to set new wallpaper: ", err)
	}

	// Take a screenshot of the wallpaper with new layout.
	screenshot2, err := screenshot.GrabScreenshot(ctx, cr)
	if err != nil {
		s.Fatal("Failed to grab screenshot: ", err)
	}

	// Verify that the wallpaper has indeed changed.
	if err = wallpaper.ValidateDiff(screenshot1, screenshot2, expectedPercent); err != nil {
		screenshot1Path := filepath.Join(s.OutDir(), "screenshot_1.png")
		screenshot2Path := filepath.Join(s.OutDir(), "screenshot_2.png")
		if err := imgcmp.DumpImageToPNG(ctx, &screenshot1, screenshot1Path); err != nil {
			s.Errorf("Failed to dump image to %s: %v", screenshot1Path, err)
		}
		if err := imgcmp.DumpImageToPNG(ctx, &screenshot2, screenshot2Path); err != nil {
			s.Errorf("Failed to dump image to %s: %v", screenshot2Path, err)
		}
		s.Fatal("Failed to validate wallpaper difference: ", err)
	}
}
