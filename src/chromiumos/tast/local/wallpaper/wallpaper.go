// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wallpaper supports interaction with ChromeOS wallpaper app.
package wallpaper

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// OpenWallpaperPicker opens the wallpaper app.
func OpenWallpaperPicker(ctx context.Context, ui *uiauto.Context) error {
	wallpaperView := nodewith.HasClass("WallpaperView")
	// Wait for the wallpaper to be visible on the screen.
	if err := ui.WaitUntilExists(wallpaperView)(ctx); err != nil {
		return errors.Wrap(err, "failed to render wallpaper view")
	}
	// Right click to display the option to set wallpaper.
	if err := ui.RightClick(wallpaperView)(ctx); err != nil {
		return errors.Wrap(err, "failed to right click on wallpaper view")
	}
	setWallpaperMenu := nodewith.Name("Set wallpaper").Role(role.MenuItem)
	// This button takes a bit before it is clickable.
	// Keep clicking it until the click is received and the menu closes.
	return ui.WithInterval(500*time.Millisecond).LeftClickUntil(setWallpaperMenu, ui.Gone(setWallpaperMenu))(ctx)
}

// SelectCollection selects the collection with the given |collection| name.
func SelectCollection(ctx context.Context, ui *uiauto.Context, collection string) error {
	collections := nodewith.Role(role.Button).HasClass("photo-inner-container")
	// We should at least wait for a few collections to be loaded.
	if err := ui.WaitUntilExists(collections.Nth(5))(ctx); err != nil {
		return errors.Wrap(err, "failed to render collection view")
	}
	return ui.LeftClick(nodewith.NameContaining(collection).Role(role.Button))(ctx)
}

// SelectImage selects the image with the given |image| title.
func SelectImage(ctx context.Context, ui *uiauto.Context, image string) error {
	images := nodewith.Role(role.ListBoxOption).HasClass("photo-inner-container")
	if err := ui.WaitUntilExists(images.First())(ctx); err != nil {
		errors.Wrap(err, "failed to render images view")
	}
	return ui.LeftClick(nodewith.Name(image).Role(role.ListBoxOption))(ctx)
}
