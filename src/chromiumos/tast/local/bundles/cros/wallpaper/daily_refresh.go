// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wallpaper

import (
	"context"
	"image"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/imgcmp"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/wallpaper"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DailyRefresh,
		Desc: "Test enabling daily refresh in the new wallpaper app",
		Contacts: []string{
			"jasontt@google.com",
			"croissant-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// DailyRefresh tests enabling daily refresh and compares the new wallpaper with the old one.
func DailyRefresh(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.EnableFeatures("WallpaperWebUI"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Open a keyboard device.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open keyboard device: ", err)
	}
	defer kb.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// The test has a dependency of network speed, so we give uiauto.Context ample time to wait for nodes to load.
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	// Take a screenshot of the current wallpaper.
	firstScreenshot, err := screenshot.GrabScreenshot(ctx, cr)
	if err != nil {
		s.Fatal("Failed to grab screenshot: ", err)
	}

	if err := wallpaper.OpenWallpaperPicker(ui)(ctx); err != nil {
		s.Fatal("Failed to open wallpaper picker: ", err)
	}

	const collection = "Solid colors"
	if err := wallpaper.SelectCollection(ui, collection)(ctx); err != nil {
		s.Fatalf("Failed to select collection %q: %v", collection, err)
	}

	if err := uiauto.Combine("Enable daily refresh",
		ui.LeftClick(nodewith.Name("Change wallpaper image daily").Role(role.ToggleButton)),
		ui.WaitUntilExists(nodewith.Name("Refresh the current wallpaper image").Role(role.Button)),
	)(ctx); err != nil {
		s.Fatal("Failed to enable daily refresh: ", err)
	}

	windowNode := nodewith.NameContaining("Wallpaper").Role(role.Window).First()
	minimizeBtn := nodewith.Name("Minimize").Role(role.Button).Ancestor(windowNode)
	// Minimize window to get the view of wallpaper image.
	if err := uiauto.Combine("find minimize button under window and click it",
		ui.LeftClick(minimizeBtn),
	)(ctx); err != nil {
		s.Fatal("Failed to minimize window: ", err)
	}

	// Take a screenshot of the new wallpaper.
	secondScreenshot, err := screenshot.GrabScreenshot(ctx, cr)
	if err != nil {
		s.Fatal("Failed to grab screenshot: ", err)
	}

	// Verify that the wallpaper has indeed changed.
	sim, err := getSimilarityPercentage(firstScreenshot, secondScreenshot)
	if err != nil {
		s.Fatal("Failed to count images similarity percentage: ", err)
	}
	if sim > 50 {
		s.Fatalf("Wallpaper is not updated. Similarity percentage: %d", sim)
	}
}

func getSimilarityPercentage(img1, img2 image.Image) (int, error) {
	diff, err := imgcmp.CountDiffPixels(img1, img2, 10)
	if err != nil {
		return -1, err
	}

	bounds := img1.Bounds()
	total := bounds.Dx() * bounds.Dy()
	percentage := 100 - diff*100/total
	return percentage, nil
}
