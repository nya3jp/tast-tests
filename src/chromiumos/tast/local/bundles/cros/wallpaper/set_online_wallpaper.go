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
	"chromiumos/tast/local/wallpaper"
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

	if err := wallpaper.OpenWallpaperPicker(ctx, ui); err != nil {
		s.Fatal("Failed to open wallpaper picker: ", err)
	}
	if err := wallpaper.SelectCollection(ctx, ui, "Solid colors"); err != nil {
		s.Fatal("Failed to select collection: ", err)
	}
	if err := wallpaper.SelectImage(ctx, ui, "Light Blue"); err != nil {
		s.Fatal("Failed to select image: ", err)
	}

	// Navigate back to collection view by clicking on the back arrow in breadcrumb.
	if err := ui.LeftClick(nodewith.Name("Back to Wallpaper").ClassName("icon-arrow-back").Role(role.Button))(ctx); err != nil {
		s.Fatal("Failed to navigate back to collection view: ", err)
	}
	if err := wallpaper.SelectCollection(ctx, ui, "Colors"); err != nil {
		s.Fatal("Failed to select collection: ", err)
	}
	if err := wallpaper.SelectImage(ctx, ui, "Bubbly"); err != nil {
		s.Fatal("Failed to select image: ", err)
	}
	if err := ui.WaitUntilExists(nodewith.Name("Currently set Bubbly").Role(role.Heading))(ctx); err != nil {
		s.Fatal("Failed to validate selected wallpaper: ", err)
	}
}
