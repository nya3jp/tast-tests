// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wallpaper

import (
	"context"
	"image/color"
	"path/filepath"
	"time"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/media/imgcmp"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/wallpaper"
	"chromiumos/tast/testing"
)

const filename = "set_local_wallpaper_light_pink_20210929.jpg"

func init() {
	testing.AddTest(&testing.Test{
		Func: SetLocalWallpaper,
		Desc: "Test setting local wallpapers in the new wallpaper app",
		Contacts: []string{
			"jasontt@google.com",
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{filename},
		SoftwareDeps: []string{"chrome"},
	})
}

func SetLocalWallpaper(ctx context.Context, s *testing.State) {
	const collection = "My Images"
	filePath := filepath.Join(filesapp.DownloadPath, filename)

	cr, err := chrome.New(ctx, chrome.EnableFeatures("WallpaperWebUI"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	if err := fsutil.CopyFile(s.DataPath(filename), filePath); err != nil {
		s.Fatalf("Could not copy %s to %s: %v", filename, filePath, err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// The test has a dependency of network speed, so we give uiauto.Context ample
	// time to wait for nodes to load.
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	if err := uiauto.Combine("Set a new custom wallpaper and minimize wallpaper picker",
		wallpaper.OpenWallpaperPicker(ui),
		wallpaper.SelectCollection(ui, collection),
		wallpaper.SelectImage(ui, filename),
		wallpaper.MinimizeWallpaperPicker(ui),
	)(ctx); err != nil {
		s.Fatal("Failed to set new wallpaper: ", err)
	}

	pink := color.RGBA{255, 203, 198, 255}
	// percentage takes into account the center cropped image is similar to the filled
	// one.
	const expectedPercent = 85
	if err := wallpaper.ValidateBackground(ctx, cr, pink, expectedPercent); err != nil {
		s.Error("Failed to validate wallpaper background: ", err)
	}

	// Take a screenshot of the current wallpaper.
	firstScreenshot, err := screenshot.GrabScreenshot(ctx, cr)
	if err != nil {
		s.Fatal("Failed to grab screenshot: ", err)
	}

	if err := uiauto.Combine("Set a new custom wallpaper, choose new layout and minimize wallpaper picker",
		wallpaper.OpenWallpaperPicker(ui),
		wallpaper.SelectCollection(ui, collection),
		ui.LeftClick(nodewith.Name("Center").Role(role.Button)),
		wallpaper.MinimizeWallpaperPicker(ui),
	)(ctx); err != nil {
		s.Fatal("Failed to set new wallpaper: ", err)
	}

	// Take a screenshot of the same wallpaper with new layout.
	secondScreenshot, err := screenshot.GrabScreenshot(ctx, cr)
	if err != nil {
		s.Fatal("Failed to grab screenshot: ", err)
	}

	// Verify that the wallpaper has indeed changed.
	if err = wallpaper.ValidateDiff(firstScreenshot, secondScreenshot, expectedPercent); err != nil {
		firstScreenshotPath := filepath.Join(s.OutDir(), "screenshot_1.png")
		secondScreenshotPath := filepath.Join(s.OutDir(), "screenshot_2.png")
		if err := imgcmp.DumpImageToPNG(ctx, &firstScreenshot, firstScreenshotPath); err != nil {
			s.Errorf("Failed to dump image to %s: %v", firstScreenshotPath, err)
		}
		if err := imgcmp.DumpImageToPNG(ctx, &secondScreenshot, secondScreenshotPath); err != nil {
			s.Errorf("Failed to dump image to %s: %v", secondScreenshotPath, err)
		}
		s.Fatal("Failed to validate wallpaper difference: ", err)
	}
}
