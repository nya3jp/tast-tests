// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wallpaper

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SetOnlineWallpaper,
		Desc: "Test setting online wallpapers in the new wallpaper app",
		Contacts: []string{
			"jasontt@google.com",
			"croissant-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

func SetOnlineWallpaper(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.EnableFeatures("WallpaperWebUI"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)
	wallpaperView := nodewith.ClassName("WallpaperView")
	setWallpaperMenu := nodewith.Name("Set wallpaper").Role(role.MenuItem)
	solidColorsCollection := nodewith.NameContaining("Solid colors").Role(role.Button)
	deepPurpleImage := nodewith.Name("Deep Purple").Role(role.ListBoxOption)
	deepPurpleLabel := nodewith.Name("Currently set Deep Purple").Role(role.Heading)
	colorsCollection := nodewith.NameContaining("Colors").Role(role.Button)
	bubblyImage := nodewith.Name("Bubbly").Role(role.ListBoxOption)
	bubblyLabel := nodewith.Name("Currently set Bubbly").Role(role.Heading)
	if err := uiauto.Combine("change the wallpaper",
		// Wait for the wallpaper to be visible on the screen.
		ui.WaitUntilExists(wallpaperView),
		ui.RightClick(wallpaperView),
		// This button takes a bit before it is clickable.
		// Keep clicking it until the click is received and the menu closes.
		ui.WithInterval(500*time.Millisecond).LeftClickUntil(setWallpaperMenu, ui.Gone(setWallpaperMenu)),
		// Wait till collection is accessible.
		ui.WaitUntilExists(solidColorsCollection),
		// Click on the collection.
		ui.LeftClick(solidColorsCollection),
		// Wait till the image is accessible.
		ui.WaitUntilExists(deepPurpleImage),
		// Set the online wallpaper image.
		ui.LeftClick(deepPurpleImage),
		// Ensure that "Deep Purple" text is displayed.
		// The UI displays the name of the currently set wallpaper.
		ui.WaitUntilExists(deepPurpleLabel),

		// Navigate back to collection view by clicking on the back arrow in breadcrumb.
		ui.LeftClick(nodewith.Name("Back to Wallpaper").ClassName("icon-arrow-back").Role(role.Button)),
		// Wait till another collection is accessible.
		ui.WaitUntilExists(colorsCollection),
		// Click on another collection.
		ui.LeftClick(colorsCollection),
		// Wait till another image is accessible.
		ui.WaitUntilExists(bubblyImage),
		// Set another online wallpaper image.
		ui.LeftClick(bubblyImage),
		// Ensure that "Bubbly" text is displayed.
		// The UI displays the name of the currently set wallpaper.
		ui.WaitUntilExists(bubblyLabel),
	)(ctx); err != nil {
		s.Fatal("Failed to change the wallpaper: ", err)
	}
}
