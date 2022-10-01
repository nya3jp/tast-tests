// Copyright 2021 The ChromiumOS Authors
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
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/imgcmp"
	"chromiumos/tast/local/personalization"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
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
	loadedCollections := nodewith.Role(role.ListBoxOption).HasClass("photo-inner-container").NameRegex(regexp.MustCompile(`.*\d+\s[iI]mages`))
	desiredCollection := loadedCollections.NameStartingWith(collection)
	return uiauto.Combine(fmt.Sprintf("select collection %q", collection),
		// We should at least wait for a few collections to be loaded.
		ui.WaitUntilExists(loadedCollections.Nth(5)),
		selectCollectionNode(ui, desiredCollection),
	)
}

// SelectCollectionWithScrolling returns an action scroll through the collection and select the collection
// with the given collection name.
func SelectCollectionWithScrolling(ctx context.Context, ui *uiauto.Context, collection string) error {
	mew, err := input.Mouse(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to setup the mouse")
	}
	defer mew.Close()

	loadedCollections := nodewith.Role(role.ListBoxOption).HasClass("photo-inner-container").NameRegex(regexp.MustCompile(`.*\d+\s[iI]mages`))
	desiredCollection := loadedCollections.NameStartingWith(collection)

	// move mouse to the collections container so that we can scroll the mouse.
	if err := uiauto.Combine("wait for collections and move mouse to collections area",
		ui.WaitUntilExists(loadedCollections.Nth(5)),
		ui.MouseMoveTo(loadedCollections.Nth(5), 10*time.Millisecond),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to load or move to collections")
	}

	return scrollDownUntilSucceeds(ctx, selectCollectionNode(ui, desiredCollection), mew)
}

func selectCollectionNode(ui *uiauto.Context, collectionNode *nodewith.Finder) uiauto.Action {
	return uiauto.Combine("select collection node",
		ui.WaitUntilExists(collectionNode),
		ui.MakeVisible(collectionNode),
		ui.LeftClick(collectionNode),
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
	imageNode := nodewith.Role(role.ListBoxOption).Name(image)
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
	minimizeBtn := nodewith.Name("Minimize").Role(role.Button).Ancestor(personalization.PersonalizationHubWindow)
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
	wallpaperNameNode := nodewith.Name(fmt.Sprintf("Currently set %v", name)).Role(role.Heading).Ancestor(personalization.PersonalizationHubWindow)
	return ui.WaitUntilExists(wallpaperNameNode)
}

// ConfirmFullscreenPreview presses the "Set as wallpaper" button while in fullscreen preview mode.
func ConfirmFullscreenPreview(ui *uiauto.Context) uiauto.Action {
	selectButton := nodewith.Name("Set as wallpaper").Ancestor(personalization.PersonalizationHubWindow).Role(role.Button)
	return uiauto.Combine("Confirm full screen preview",
		ui.WaitUntilExists(selectButton),
		ui.LeftClick(selectButton),
		ui.WaitUntilGone(selectButton))
}

// CancelFullscreenPreview presses the "Exit wallpaper preview" button while in fullscreen preview mode.
func CancelFullscreenPreview(ui *uiauto.Context) uiauto.Action {
	cancelButton := nodewith.Name("Exit wallpaper preview").Ancestor(personalization.PersonalizationHubWindow).Role(role.Button)
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

// CurrentWallpaperFinder finds currently set wallpaper node.
func CurrentWallpaperFinder() *nodewith.Finder {
	return nodewith.Role(role.Heading).NameStartingWith("Currently set")
}

// CurrentWallpaperWithSpecificNameFinder find currently set wallpaper with an exact match name.
func CurrentWallpaperWithSpecificNameFinder(wallpaperName string) *nodewith.Finder {
	return nodewith.Role(role.Heading).NameContaining(wallpaperName)
}

// CurrentWallpaper gets the name of the current wallpaper.
func CurrentWallpaper(ctx context.Context, ui *uiauto.Context) (string, error) {
	currentWallpaperNode, err := ui.Info(ctx, CurrentWallpaperFinder())
	if err != nil {
		return "", errors.Wrap(err, "failed to find currently set wallpaper")
	}
	return currentWallpaperNode.Name, nil
}

// LocalImageDownloadPath gets the path of a local image in Downloads folder.
func LocalImageDownloadPath(ctx context.Context, user, image string) (string, error) {
	downloadsPath, err := cryptohome.DownloadsPath(ctx, user)
	if err != nil {
		return "", errors.Wrap(err, "failed to get users Download path")
	}
	filePath := filepath.Join(downloadsPath, image)
	return filePath, nil
}

// scrollDownUntilSucceeds scrolls the mouse down until an action is achieved.
func scrollDownUntilSucceeds(ctx context.Context, action uiauto.Action, mew *input.MouseEventWriter) error {
	const (
		maxNumSelectRetries = 4
		numScrolls          = 100
	)
	var actionErr error
	for i := 0; i < maxNumSelectRetries; i++ {
		if actionErr = action(ctx); actionErr == nil {
			return nil
		}
		for j := 0; j < numScrolls; j++ {
			if err := mew.ScrollDown(); err != nil {
				return errors.Wrap(err, "failed to scroll down")
			}
		}
	}
	return actionErr
}
