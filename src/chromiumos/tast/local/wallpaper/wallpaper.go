// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wallpaper supports interaction with ChromeOS wallpaper app.
package wallpaper

import (
	"fmt"
	"time"

	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// OpenWallpaperPicker opens the wallpaper app.
func OpenWallpaperPicker(ui *uiauto.Context) uiauto.Action {
	setWallpaperMenu := nodewith.Name("Set wallpaper").Role(role.MenuItem)
	return uiauto.Combine("open wallpaper picker",
		ui.RightClick(nodewith.HasClass("WallpaperView")),
		ui.WithInterval(500*time.Millisecond).LeftClickUntil(setWallpaperMenu, ui.Gone(setWallpaperMenu)),
	)
}

// SelectCollection selects the collection with the given collection name.
func SelectCollection(ui *uiauto.Context, collection string) uiauto.Action {
	collections := nodewith.Role(role.Button).HasClass("photo-inner-container")
	return uiauto.Combine(fmt.Sprintf("select collection %q", collection),
		// We should at least wait for a few collections to be loaded.
		ui.WaitUntilExists(collections.Nth(5)),
		ui.LeftClick(nodewith.NameContaining(collection).Role(role.Button)),
	)
}

// SelectImage selects the image with the given image title.
func SelectImage(ui *uiauto.Context, image string) uiauto.Action {
	images := nodewith.Role(role.ListBoxOption).HasClass("photo-inner-container")
	return uiauto.Combine(fmt.Sprintf("select image %q", image),
		ui.WaitUntilExists(images.First()),
		ui.LeftClick(nodewith.Name(image).Role(role.ListBoxOption)),
	)
}
