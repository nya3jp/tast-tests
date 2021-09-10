// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wallpaper

import (
	"context"
	"image/color"
	"time"

	"chromiumos/tast/errors"
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
		Func: SwitchOnlineWallpapers,
		Desc: "Test setting online wallpapers in the new wallpaper app",
		Contacts: []string{
			"jasontt@google.com",
			"croissant-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// SwitchOnlineWallpapers tests the flow of rapidly switching online wallpapers from the same collection and make sure the correct one is displayed.
func SwitchOnlineWallpapers(ctx context.Context, s *testing.State) {
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

	if err := wallpaper.OpenWallpaperPicker(ctx, ui); err != nil {
		s.Fatal("Failed to open wallpaper picker: ", err)
	}
	if err := wallpaper.SelectCollection(ctx, ui, "Solid colors"); err != nil {
		s.Fatal("Failed to select collection: ", err)
	}
	if err := wallpaper.SelectImage(ctx, ui, "Light Blue"); err != nil {
		s.Fatal("Failed to select image: ", err)
	}
	if err := wallpaper.SelectImage(ctx, ui, "Google Green"); err != nil {
		s.Fatal("Failed to select image: ", err)
	}
	if err := wallpaper.SelectImage(ctx, ui, "Google Yellow"); err != nil {
		s.Fatal("Failed to select image: ", err)
	}
	// Make sure yellow is last selected. We will be comparing the blurred background from lock screen with the given rgba color.
	if err := wallpaper.SelectImage(ctx, ui, "Yellow"); err != nil {
		s.Fatal("Failed to select image: ", err)
	}

	windowNode := nodewith.NameContaining("Wallpaper").Role(role.Window).First()
	minimizeBtn := nodewith.Name("Minimize").Role(role.Button).Ancestor(windowNode)
	if err := uiauto.Combine("find minimize button under window and click it",
		ui.LeftClick(minimizeBtn),
	)(ctx); err != nil {
		s.Fatal("Failed to minimize window: ", err)
	}

	yellowClr := color.RGBA{255, 227, 53, 255}
	expectedPercent := 90
	if err := validateBackground(ctx, cr, yellowClr, expectedPercent); err != nil {
		s.Error("Failed to validate wallpaper background: ", err)
	}
}

// validateBackground takes a screenshot and check the percentage of the clr in the image, returns error if it's less than expectedPercent%.
func validateBackground(ctx context.Context, cr *chrome.Chrome, clr color.Color, expectedPercent int) error {
	// Take a screenshot and check the clr pixels percentage.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		img, err := screenshot.GrabScreenshot(ctx, cr)
		if err != nil {
			return errors.Wrap(err, "failed to grab screenshot")
		}
		rect := img.Bounds()
		clrPixels := imgcmp.CountPixelsWithDiff(img, clr, 10)
		totalPixels := (rect.Max.Y - rect.Min.Y) * (rect.Max.X - rect.Min.X)
		percent := clrPixels * 100 / totalPixels
		if percent < expectedPercent {
			return errors.Errorf("unexpected red pixels percentage: got %d / %d = %d%%; want at least %d%%", clrPixels, totalPixels, percent, expectedPercent)
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second, Interval: time.Second}); err != nil {
		return err
	}
	return nil
}
