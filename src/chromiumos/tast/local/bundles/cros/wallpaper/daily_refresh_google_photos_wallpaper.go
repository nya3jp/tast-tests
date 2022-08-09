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
	"chromiumos/tast/local/media/imgcmp"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/wallpaper"
	"chromiumos/tast/local/wallpaper/constants"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DailyRefreshGooglePhotosWallpaper,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test setting Google Photos wallpapers in the wallpaper app",
		Contacts: []string{
			"xiaohuic@google.com",
			"assistive-eng@google.com",
			"chromeos-sw-engprod@google.com",
		},
		// Disabled due to <1% pass rate over 30 days. See b/241943381
		//Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps: []string{
			"wallpaper.googlePhotosAccountPool",
		},
		Timeout: 5 * time.Minute,
	})
}

func DailyRefreshGooglePhotosWallpaper(ctx context.Context, s *testing.State) {
	// Setting Google Photos wallpapers requires that Chrome be logged in with
	// a user from an account pool which has been preconditioned to have a
	// Google Photos library with specific photos/albums present. Note that sync
	// is disabled to prevent flakiness caused by wallpaper cross device sync.
	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("wallpaper.googlePhotosAccountPool")),
		chrome.EnableFeatures("WallpaperGooglePhotosIntegration", "PersonalizationHub"),
		chrome.ExtraArgs("--disable-sync"))
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

	// Take a screenshot of the current wallpaper.
	screenshot1, err := screenshot.GrabScreenshot(ctx, cr)
	if err != nil {
		s.Fatal("Failed to grab screenshot: ", err)
	}

	if err := uiauto.Combine("Enable daily refresh and minimize wallpaper picker",
		wallpaper.OpenWallpaperPicker(ui),
		wallpaper.SelectCollection(ui, constants.GooglePhotosWallpaperCollection),
		ui.LeftClick(constants.GooglePhotosWallpaperAlbumsButton),
		wallpaper.SelectGooglePhotosAlbum(ui, constants.GooglePhotosWallpaperAlbum),
		ui.LeftClick(constants.ChangeDailyButton),
		ui.WaitUntilExists(constants.RefreshButton),
		wallpaper.MinimizeWallpaperPicker(ui),
	)(ctx); err != nil {
		s.Fatal("Failed to enable daily refresh: ", err)
	}

	// Take a screenshot of the current wallpaper.
	screenshot2, err := screenshot.GrabScreenshot(ctx, cr)
	if err != nil {
		s.Fatal("Failed to grab screenshot: ", err)
	}

	// Verify that the wallpaper has indeed changed.
	const expectedPercent = 90
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

	if err := uiauto.Combine("Manually refresh and minimize wallpaper picker",
		wallpaper.OpenWallpaperPicker(ui),
		wallpaper.SelectCollection(ui, constants.GooglePhotosWallpaperCollection),
		ui.LeftClick(constants.GooglePhotosWallpaperAlbumsButton),
		wallpaper.SelectGooglePhotosAlbum(ui, constants.GooglePhotosWallpaperAlbum),
		ui.LeftClick(constants.RefreshButton),

		// NOTE: The refresh button will be hidden while updating the wallpaper so
		// use its reappearance as a proxy to know when the wallpaper has finished
		// updating.
		ui.WaitUntilGone(constants.RefreshButton),
		ui.WaitUntilExists(constants.RefreshButton),

		wallpaper.MinimizeWallpaperPicker(ui),
	)(ctx); err != nil {
		s.Fatal("Failed to manually refresh: ", err)
	}

	// Take a screenshot of the current wallpaper.
	screenshot3, err := screenshot.GrabScreenshot(ctx, cr)
	if err != nil {
		s.Fatal("Failed to grab screenshot: ", err)
	}

	// Verify that the wallpaper has indeed changed.
	if err = wallpaper.ValidateDiff(screenshot2, screenshot3, expectedPercent); err != nil {
		screenshot3Path := filepath.Join(s.OutDir(), "screenshot_3.png")
		if err := imgcmp.DumpImageToPNG(ctx, &screenshot3, screenshot3Path); err != nil {
			s.Errorf("Failed to dump image to %s: %v", screenshot3Path, err)
		}
		s.Fatal("Failed to validate wallpaper difference: ", err)
	}
}
