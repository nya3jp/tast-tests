// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wallpaper supports interaction with ChromeOS personalization hub
// wallpaper subpage.
package wallpaper

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"regexp"
	"time"

	"go.chromium.org/chromiumos/tast/errors"
	"go.chromium.org/chromiumos/tast-tests/local/chrome"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/nodewith"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/role"
	"go.chromium.org/chromiumos/tast-tests/local/input"
	"go.chromium.org/chromiumos/tast-tests/local/media/imgcmp"
	"go.chromium.org/chromiumos/tast-tests/local/personalization"
	"go.chromium.org/chromiumos/tast-tests/local/screenshot"
	"go.chromium.org/chromiumos/tast/testing"
)

// Wallpaper human readable strings
const (
	Personalization = "Wallpaper & style"
)

// OpenWallpaperPicker returns an action to open the personalization hub and navigate to wallpaper subpage.
func OpenWallpaperPicker(ui *uiauto.Context) uiauto.Action {
	return uiauto.Combine("open wallpaper picker from personalization hub",
		personalization.OpenPersonalizationHub(ui),
		personalization.OpenWallpaperSubpage(ui))
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

// SelectGooglePhotosAlbum returns an action to select the Google Photos album with the given name.
func SelectGooglePhotosAlbum(ui *uiauto.Context, name string) uiauto.Action {
	albumNode := nodewith.HasClass("album").Name(name)
	return uiauto.Combine(fmt.Sprintf("select Google Photos album %q", name),
		ui.WaitUntilExists(albumNode),
		ui.MakeVisible(albumNode),
		ui.LeftClick(albumNode))
}

// SelectGooglePhotosPhoto returns an action to select the Google Photos photo with the given name.
func SelectGooglePhotosPhoto(ui *uiauto.Context, name string) uiauto.Action {
	photoNode := nodewith.HasClass("photo").Name(name)
	return uiauto.Combine(fmt.Sprintf("select Google Photos photo %q", name),
		ui.WaitUntilExists(photoNode),
		ui.MakeVisible(photoNode),
		ui.LeftClick(photoNode))
}

// SelectImage returns an action to select the image with the given image title.
func SelectImage(ui *uiauto.Context, image string) uiauto.Action {
	imageNode := nodewith.Role(role.ListBoxOption).HasClass("photo-inner-container").Name(image)
	return uiauto.Combine(fmt.Sprintf("select image %q", image),
		ui.WaitUntilExists(imageNode),
		ui.MakeVisible(imageNode),
		ui.LeftClick(imageNode))
}

// BackToWallpaper presses the wallpaper tag from the breadcrumb.
// Used to navigate from an individual collection to the collections list.
func BackToWallpaper(ui *uiauto.Context) uiauto.Action {
	return personalization.NavigateBreadcrumb("Wallpaper", ui)
}

// MinimizeWallpaperPicker returns an action to minimize the personalization hub.
func MinimizeWallpaperPicker(ui *uiauto.Context) uiauto.Action {
	windowNode := nodewith.NameContaining(Personalization).Role(role.Window).First()
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
	windowNode := nodewith.NameContaining(Personalization).Role(role.Window).First()
	wallpaperNameNode := nodewith.Name(fmt.Sprintf("Currently set %v", name)).Role(role.Heading).Ancestor(windowNode)
	return ui.WaitUntilExists(wallpaperNameNode)
}

// ConfirmFullscreenPreview presses the "Set as wallpaper" button while in fullscreen preview mode.
func ConfirmFullscreenPreview(ui *uiauto.Context) uiauto.Action {
	windowNode := nodewith.NameContaining(Personalization).Role(role.Window).First()
	selectButton := nodewith.Name("Set as wallpaper").Ancestor(windowNode).Role(role.Button)
	return uiauto.Combine("Confirm full screen preview",
		ui.WaitUntilExists(selectButton),
		ui.LeftClick(selectButton),
		ui.WaitUntilGone(selectButton))
}

// CancelFullscreenPreview presses the "Exit wallpaper preview" button while in fullscreen preview mode.
func CancelFullscreenPreview(ui *uiauto.Context) uiauto.Action {
	windowNode := nodewith.NameContaining(Personalization).Role(role.Window).First()
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
