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

	// The test has a dependency of network speed, so we give uiauto.Context ample time to wait for nodes to load.
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)
	setWallpaperMenu := nodewith.Name("Set wallpaper").Role(role.MenuItem)
	if err := uiauto.Combine("change the wallpaper",
		// Wait for the wallpaper to be visible on the screen.
		ui.RightClick(nodewith.ClassName("WallpaperView")),
		// This button takes a bit before it is clickable.
		// Keep clicking it until the click is received and the menu closes.
		ui.WithInterval(500*time.Millisecond).LeftClickUntil(setWallpaperMenu, ui.Gone(setWallpaperMenu)),

		// Click on the collection.
		ui.LeftClick(nodewith.NameContaining("Solid colors").Role(role.Button)),
		// Set the online wallpaper image.
		ui.LeftClick(nodewith.Name("Deep Purple").Role(role.ListBoxOption)),
		// Ensure that "Deep Purple" text is displayed.
		// The UI displays the name of the currently set wallpaper.
		ui.WaitUntilExists(nodewith.Name("Currently set Deep Purple").Role(role.Heading)),

		// Navigate back to collection view by clicking on the back arrow in breadcrumb.
		ui.LeftClick(nodewith.Name("Back to Wallpaper").ClassName("icon-arrow-back").Role(role.Button)),
		// Click on another collection.
		ui.LeftClick(nodewith.NameContaining("Colors").Role(role.Button)),
		// Set another online wallpaper image.
		ui.LeftClick(nodewith.Name("Bubbly").Role(role.ListBoxOption)),
		// Ensure that "Bubbly" text is displayed.
		// The UI displays the name of the currently set wallpaper.
		ui.WaitUntilExists(nodewith.Name("Currently set Bubbly").Role(role.Heading)),
	)(ctx); err != nil {
		s.Fatal("Failed to change the wallpaper: ", err)
	}
}
