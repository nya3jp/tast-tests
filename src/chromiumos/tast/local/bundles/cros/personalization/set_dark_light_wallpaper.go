// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package personalization

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
	"chromiumos/tast/local/personalization"
	"chromiumos/tast/local/wallpaper"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SetDarkLightWallpaper,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test setting online wallpapers in the new wallpaper app",
		Contacts: []string{
			"thuongphan@google.com",
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

func SetDarkLightWallpaper(ctx context.Context, s *testing.State) {
	const (
		dlCollection = "Element"
		dImage       = "Wind Dark Digital Art by Rutger Paulusse"
		lImage       = "Wind Light Digital Art by Rutger Paulusse"
	)

	cr, err := chrome.New(ctx, chrome.EnableFeatures("PersonalizationHub", "DarkLightMode"))

	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}

	defer cr.Close(ctx)

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

	if err := uiauto.Combine("Enable dark mode",
		personalization.OpenPersonalizationHub(ui),
		personalization.ToggleDarkMode(ui))(ctx); err != nil {
		s.Fatal("Failed to enable dark mode: ", err)
	}

	if err := uiauto.Combine(fmt.Sprintf("Change the wallpaper to %s %s", dlCollection, dImage),
		personalization.OpenWallpaperSubpage(ui),
		wallpaper.SelectCollection(ui, dlCollection),
		wallpaper.SelectImage(ui, dImage),
		ui.WaitUntilExists(nodewith.Name(fmt.Sprintf("Currently set %v", dImage)).Role(role.Heading)))(ctx); err != nil {
		s.Fatalf("Failed to validate selected wallpaper %s %s: %v", dlCollection, dImage, err)
	}

	if err := uiauto.Combine("Enable light mode",
		personalization.NavigateHome(ui),
		personalization.ToggleLightMode(ui))(ctx); err != nil {
		s.Fatal("Failed to enable light mode: ", err)
	}

	if err := uiauto.Combine(fmt.Sprintf("Validate currently set wallpaper changed to %s %s", dlCollection, lImage),
		personalization.OpenWallpaperSubpage(ui),
		ui.WaitUntilExists(nodewith.Name(fmt.Sprintf("Currently set %v", lImage)).Role(role.Heading)))(ctx); err != nil {
		s.Fatalf("Failed to validate selected wallpaper %s %s: %v", dlCollection, lImage, err)
	}
}
