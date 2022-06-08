// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wallpaper

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
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
		Func:         DailyRefresh,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test enabling daily refresh in the new wallpaper app",
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

// DailyRefresh tests enabling daily refresh and compares the new wallpaper with the old one.
func DailyRefresh(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Force Chrome to be in clamshell mode to make sure wallpaper view is clearly
	// visible for us to compare it with the given rgba color.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure DUT is not in tablet mode: ", err)
	}
	defer cleanup(cleanupCtx)

	// The test has a dependency of network speed, so we give uiauto.Context ample time to wait for nodes to load.
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	// Take a screenshot of the current wallpaper.
	firstScreenshot, err := screenshot.GrabScreenshot(ctx, cr)
	if err != nil {
		s.Fatal("Failed to grab screenshot: ", err)
	}

	if err := uiauto.Combine("Enable daily refresh",
		wallpaper.OpenWallpaperPicker(ui),
		wallpaper.SelectCollection(ui, constants.ElementCollection),
		ui.LeftClick(constants.ChangeDailyButton),
		ui.WaitUntilExists(constants.RefreshButton),
		wallpaper.MinimizeWallpaperPicker(ui),
	)(ctx); err != nil {
		s.Fatal("Failed to enable daily refresh: ", err)
	}

	// Take a screenshot of the new wallpaper.
	secondScreenshot, err := screenshot.GrabScreenshot(ctx, cr)
	if err != nil {
		s.Fatal("Failed to grab screenshot: ", err)
	}

	// Verify that the wallpaper has indeed changed.
	const expectedPercent = 90
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
