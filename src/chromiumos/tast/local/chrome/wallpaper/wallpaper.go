// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wallpaper is used to test changing the wallpaper. This is for the deprecated wallpaper picker extension and not the new wallpaper SWA.
package wallpaper

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// OpenWallpaperDeprecated opens the wallpaper picker.
func OpenWallpaperDeprecated(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)
	setWallpaperMenu := nodewith.Name("Set wallpaper").Role(role.MenuItem)
	if err := uiauto.Combine("open wallpaper",
		ui.RightClick(nodewith.ClassName("WallpaperView")),
		// This button takes a bit before it is clickable.
		// Keep clicking it until the click is received and the menu closes.
		ui.WithInterval(500*time.Millisecond).LeftClickUntil(setWallpaperMenu, ui.Gone(setWallpaperMenu)))(ctx); err != nil {
		return errors.Wrap(err, "failed to open wallpaper picker")
	}
	return nil
}

// ChangeWallpaperDeprecated changes the wallpaper to the name under the specified category.
func ChangeWallpaperDeprecated(ctx context.Context, tconn *chrome.TestConn, category, name string) error {
	ui := uiauto.New(tconn)
	categoryMenu := nodewith.Name(category).Role(role.StaticText)
	if err := uiauto.Combine("change wallpaper",
		ui.WaitUntilExists(categoryMenu),
		ui.MakeVisible(categoryMenu),
		ui.LeftClick(categoryMenu),
		ui.LeftClick(nodewith.Name(name).Role(role.ListItem)))(ctx); err != nil {
		return errors.Wrapf(err, "failed to change the wallpaper to %s %s", category, name)
	}
	if err := CheckWallpaperDeprecated(ctx, tconn, name); err != nil {
		return errors.Wrapf(err, "could not verify changing the wallpaper to %s %s", category, name)
	}
	return nil
}

// CheckWallpaperDeprecated verifies that the wallpaper changed to the name.
func CheckWallpaperDeprecated(ctx context.Context, tconn *chrome.TestConn, name string) error {
	ui := uiauto.New(tconn)
	// The UI displays the name of the currently set wallpaper.
	if err := ui.WaitUntilExists(nodewith.Name(name).Role(role.StaticText))(ctx); err != nil {
		return errors.Wrapf(err, "failed to find wallpaper with name %s", name)
	}
	return nil
}

// CloseWallpaperDeprecated closes the wallpaper picker.
func CloseWallpaperDeprecated(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)
	closeButton := nodewith.Name("Close").Role(role.MenuItem)
	if err := uiauto.Combine("close wallpaper",
		ui.RightClick(nodewith.Name("Wallpaper Picker").Role(role.Button)),
		ui.WaitUntilExists(closeButton),
		ui.LeftClick(closeButton))(ctx); err != nil {
		return errors.Wrap(err, "failed to close wallpaper picker")
	}
	return nil
}
