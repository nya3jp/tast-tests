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
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/imgcmp"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

// OpenWallpaperPicker returns an action to open the wallpaper app.
func OpenWallpaperPicker(ui *uiauto.Context) uiauto.Action {
	setWallpaperMenu := nodewith.Name("Set wallpaper").Role(role.MenuItem)
	return ui.RetryUntil(uiauto.Combine("open wallpaper picker",
		ui.RightClick(nodewith.HasClass("WallpaperView")),
		ui.WithInterval(300*time.Millisecond).LeftClickUntil(setWallpaperMenu, ui.Gone(setWallpaperMenu))),
		ui.Exists(nodewith.NameContaining("Wallpaper").Role(role.Window).First()))
}

// SelectCollection returns an action to select the collection with the given collection name.
func SelectCollection(ui *uiauto.Context, collection string) uiauto.Action {
	// Collections that are fully loaded will have a name like "Name 10 Images".
	loadedCollections := nodewith.Role(role.Button).HasClass("photo-inner-container").NameRegex(regexp.MustCompile(`.*\d+\s[iI]mages`))
	desiredCollection := loadedCollections.NameStartingWith(collection)
	return uiauto.Combine(fmt.Sprintf("select collection %q", collection),
		// We should at least wait for a few collections to be loaded.
		ui.WaitUntilExists(loadedCollections.Nth(5)),
		ui.WaitUntilExists(desiredCollection),
		ui.MakeVisible(desiredCollection),
		ui.LeftClick(desiredCollection),
	)
}

// SelectImage returns an action to select the image with the given image title.
func SelectImage(ui *uiauto.Context, image string) uiauto.Action {
	imageNode := nodewith.Role(role.ListBoxOption).HasClass("photo-inner-container").Name(image)
	return uiauto.Combine(fmt.Sprintf("select image %q", image),
		ui.WaitUntilExists(imageNode),
		ui.MakeVisible(imageNode),
		ui.LeftClick(imageNode))
}

// Back presses the back button in the wallpaper app. Used to navigate from an individual collection to the collections list.
func Back(ui *uiauto.Context) uiauto.Action {
	back := nodewith.Role(role.Button).Name("Back to Wallpaper").HasClass("icon-arrow-back")
	return uiauto.Combine("click wallpaper app back button",
		ui.WaitUntilExists(back),
		ui.LeftClick(back))
}

// MinimizeWallpaperPicker returns an action to minimize the wallpaper picker.
func MinimizeWallpaperPicker(ui *uiauto.Context) uiauto.Action {
	windowNode := nodewith.NameContaining("Wallpaper").Role(role.Window).First()
	minimizeBtn := nodewith.Name("Minimize").Role(role.Button).Ancestor(windowNode)
	// Minimize window to get the view of wallpaper image.
	return ui.LeftClickUntil(minimizeBtn, ui.Gone(minimizeBtn))
}

// CloseWallpaperPicker returns an action to close the wallpaper picker via the Ctrl+W shortcut.
func CloseWallpaperPicker() uiauto.Action {
	return func(ctx context.Context) error {
		kb, err := input.VirtualKeyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get virtual keyboard")
		}
		defer kb.Close()
		return kb.Accel(ctx, "Ctrl+W")
	}
}

// WaitForWallpaperWithName checks that a text node exists inside the wallpaper app with the given name.
// Requires the wallpaper app to be open.
func WaitForWallpaperWithName(ui *uiauto.Context, name string) uiauto.Action {
	windowNode := nodewith.NameContaining("Wallpaper").Role(role.Window).First()
	wallpaperNameNode := nodewith.Name(fmt.Sprintf("Currently set %v", name)).Role(role.Heading).Ancestor(windowNode)
	return ui.WaitUntilExists(wallpaperNameNode)
}

// ConfirmFullscreenPreview presses the "Set as wallpaper" button while in fullscreen preview mode.
func ConfirmFullscreenPreview(ui *uiauto.Context) uiauto.Action {
	windowNode := nodewith.NameContaining("Wallpaper").Role(role.Window).First()
	selectButton := nodewith.Name("Set as wallpaper").Ancestor(windowNode).Role(role.Button)
	return uiauto.Combine("Confirm full screen preview",
		ui.WaitUntilExists(selectButton),
		ui.LeftClick(selectButton),
		ui.WaitUntilGone(selectButton))
}

// CancelFullscreenPreview presses the "Exit wallpaper preview" button while in fullscreen preview mode.
func CancelFullscreenPreview(ui *uiauto.Context) uiauto.Action {
	windowNode := nodewith.NameContaining("Wallpaper").Role(role.Window).First()
	cancelButton := nodewith.Name("Exit wallpaper preview").Ancestor(windowNode).Role(role.Button)
	return uiauto.Combine("Cancel full screen preview",
		ui.WaitUntilExists(cancelButton),
		ui.LeftClick(cancelButton),
		ui.WaitUntilGone(cancelButton))
}

// ValidateBackground takes a screenshot and check the percentage of the clr in the image,
// returns error if it's less than expectedPercent%.
func ValidateBackground(cr *chrome.Chrome, clr color.Color, expectedPercent int) uiauto.Action {
	return func(ctx context.Context) error {
		// Take a screenshot and check the clr pixels percentage.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			img, err := screenshot.GrabScreenshot(ctx, cr)
			if err != nil {
				return errors.Wrap(err, "failed to grab screenshot")
			}
			rect := img.Bounds()
			correctPixels := imgcmp.CountPixelsWithDiff(img, clr, 60)
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
