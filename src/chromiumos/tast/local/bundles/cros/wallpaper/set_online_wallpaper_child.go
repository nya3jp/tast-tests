// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wallpaper

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/wallpaper"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SetOnlineWallpaperChild,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test setting online wallpapers in the new wallpaper app for a child user",
		Contacts: []string{
			"tobyhuang@chromium.org", "cros-families-eng+test@google.com", "chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"unicorn.wallpaperCategory", "unicorn.wallpaperName"},
		Fixture:      "familyLinkUnicornLogin",
		Timeout:      5 * time.Minute,
	})
}

func SetOnlineWallpaperChild(ctx context.Context, s *testing.State) {
	collection := s.RequiredVar("unicorn.wallpaperCategory")
	image := s.RequiredVar("unicorn.wallpaperName")

	tconn := s.FixtValue().(*familylink.FixtData).TestConn

	// Force Chrome to be in clamshell mode to make sure wallpaper preview is not
	// enabled.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure DUT is not in tablet mode: ", err)
	}
	defer cleanup(ctx)

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// The test has a dependency of network speed, so we give uiauto.Context ample
	// time to wait for nodes to load.
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	// Sleep for 10 seconds for family link notification to disappear.
	// The notification will intefere with the test if we start the test at the same time.
	// TODO: remove Sleep action if there is a better solution to handle this case.
	if err := action.Sleep(10 * time.Second)(ctx); err != nil {
		s.Fatal("Failed to sleep for 10 seconds: ", err)
	}

	if err := uiauto.Combine(fmt.Sprintf("Change the wallpaper to %s %s", collection, image),
		wallpaper.OpenWallpaperPicker(ui),
		wallpaper.SelectCollection(ui, collection),
		wallpaper.SelectImage(ui, image),
		ui.WaitUntilExists(wallpaper.CurrentWallpaperWithSpecificNameFinder(image)),
	)(ctx); err != nil {
		s.Fatalf("Failed to validate selected wallpaper %s %s: %v", collection, image, err)
	}
}
