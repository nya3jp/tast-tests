// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wallpaper

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/wallpaper"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SetOnlineWallpaper,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test setting online wallpapers in the new wallpaper app",
		Contacts: []string{
			"jasontt@google.com",
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Fixture:      "chromeLoggedIn",
	})
}

func SetOnlineWallpaper(ctx context.Context, s *testing.State) {
	const (
		firstCollection  = "Cityscapes"
		firstImage       = "J. Paul Getty Museum, Los Angeles Photo by Victor Cheng"
		secondCollection = "Imaginary"
		secondImage      = "The Savanna's Band Digital Art by Leo Natsume"
	)

	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Force Chrome to be in clamshell mode to make sure wallpaper preview is not
	// enabled.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure DUT is not in tablet mode: ", err)
	}
	defer cleanup(ctx)

	// The test has a dependency of network speed, so we give uiauto.Context ample
	// time to wait for nodes to load.
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	if err := uiauto.Combine(fmt.Sprintf("Change the wallpaper to %s %s", firstCollection, firstImage),
		wallpaper.OpenWallpaperPicker(ui),
		wallpaper.SelectCollection(ui, firstCollection),
		wallpaper.SelectImage(ui, firstImage),
		ui.WaitUntilExists(nodewith.Name(fmt.Sprintf("Currently set %v", firstImage)).Role(role.Heading)))(ctx); err != nil {
		s.Fatalf("Failed to validate selected wallpaper %s %s: %v", firstCollection, firstImage, err)
	}

	// Navigate back to collection view by clicking on the back arrow in breadcrumb.
	if err := uiauto.Combine(fmt.Sprintf("Change the wallpaper to %s %s", secondCollection, secondImage),
		wallpaper.BackToWallpaper(ui),
		wallpaper.SelectCollection(ui, secondCollection),
		wallpaper.SelectImage(ui, secondImage),
		ui.WaitUntilExists(nodewith.Name(fmt.Sprintf("Currently set %v", secondImage)).Role(role.Heading)))(ctx); err != nil {
		s.Fatalf("Failed to validate selected wallpaper %s %s: %v", secondCollection, secondImage, err)
	}
}
