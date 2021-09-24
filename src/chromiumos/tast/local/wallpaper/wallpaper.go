// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wallpaper supports interaction with ChromeOS wallpaper app.
package wallpaper

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/media/imgcmp"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

// OpenWallpaperPicker returns an action to open the wallpaper app.
func OpenWallpaperPicker(ui *uiauto.Context) uiauto.Action {
	setWallpaperMenu := nodewith.Name("Set wallpaper").Role(role.MenuItem)
	return uiauto.Combine("open wallpaper picker",
		ui.RightClick(nodewith.HasClass("WallpaperView")),
		ui.WithInterval(1*time.Second).LeftClickUntil(setWallpaperMenu, ui.Gone(setWallpaperMenu)),
		ui.WaitUntilExists(nodewith.NameContaining("Wallpaper").Role(role.Window).First()),
	)
}

// SelectCollection returns an action to select the collection with the given collection name.
func SelectCollection(ui *uiauto.Context, collection string) uiauto.Action {
	collections := nodewith.Role(role.Button).HasClass("photo-inner-container")
	return uiauto.Combine(fmt.Sprintf("select collection %q", collection),
		// We should at least wait for a few collections to be loaded.
		ui.WaitUntilExists(collections.Nth(5)),
		ui.LeftClick(nodewith.NameContaining(collection).Role(role.Button)),
	)
}

// SelectImage returns an action to select the image with the given image title.
func SelectImage(ui *uiauto.Context, image string) uiauto.Action {
	images := nodewith.Role(role.ListBoxOption).HasClass("photo-inner-container")
	return uiauto.Combine(fmt.Sprintf("select image %q", image),
		ui.WaitUntilExists(images.First()),
		ui.LeftClick(nodewith.Name(image).Role(role.ListBoxOption)),
	)
}

// MinimizeWallpaperPicker returns an action to minimize the wallpaper picker.
func MinimizeWallpaperPicker(ui *uiauto.Context) uiauto.Action {
	windowNode := nodewith.NameContaining("Wallpaper").Role(role.Window).First()
	minimizeBtn := nodewith.Name("Minimize").Role(role.Button).Ancestor(windowNode)
	// Minimize window to get the view of wallpaper image.
	return ui.LeftClickUntil(minimizeBtn, ui.Gone(minimizeBtn))
}

// ValidateBackground takes a screenshot and check the percentage of the clr in the image,
// returns error if it's less than expectedPercent%.
func ValidateBackground(ctx context.Context, cr *chrome.Chrome, clr color.Color, expectedPercent int) error {
	// Take a screenshot and check the clr pixels percentage.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		img, err := screenshot.GrabScreenshot(ctx, cr)
		if err != nil {
			return errors.Wrap(err, "failed to grab screenshot")
		}
		rect := img.Bounds()
		correctPixels := imgcmp.CountPixelsWithDiff(img, clr, 10)
		totalPixels := rect.Dx() * rect.Dy()
		percent := correctPixels * 100 / totalPixels
		if percent < expectedPercent {
			return errors.Errorf("unexpected pixels percentage: got %d / %d = %d%%; want at least %d%%", correctPixels, totalPixels, percent, expectedPercent)
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second, Interval: time.Second}); err != nil {
		return err
	}
	return nil
}

// ValidateDiff checks the diff percentage between 2 images and returns error if
// the percentage is less than expectedPercent.
func ValidateDiff(img1, img2 image.Image, expectedPercent int) error {
	diff, err := imgcmp.CountDiffPixels(img1, img2, 10)
	if err != nil {
		return errors.Wrap(err, "failed to count diff pixels")
	}

	bounds := img1.Bounds()
	total := bounds.Dx() * bounds.Dy()
	if percentage := diff * 100 / total; percentage < expectedPercent {
		return errors.Errorf("unexpected percentage: got %d%%; want at least %d%%", percentage, expectedPercent)
	}
	return nil
}
