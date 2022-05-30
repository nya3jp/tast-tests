// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package personalization

import (
	"context"
	"regexp"
	"time"

	"go.chromium.org/chromiumos/tast/ctxutil"
	"go.chromium.org/chromiumos/tast-tests/local/chrome"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/ash"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/faillog"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/nodewith"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/role"
	"go.chromium.org/chromiumos/tast-tests/local/personalization"
	"go.chromium.org/chromiumos/tast-tests/local/wallpaper"
	"go.chromium.org/chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SetDailyRefreshDLWallpaper,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test setting D/L wallpapers daily refresh in the personalization hub app",
		Contacts: []string{
			"thuongphan@google.com",
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

func SetDailyRefreshDLWallpaper(ctx context.Context, s *testing.State) {
	const dlCollection = "Element"
	// Dark mode wallpaper title includes "Dark" word but light mode wallpaper title doesn't.
	darkRegex := regexp.MustCompile(`Currently set Daily Refresh .*\sDark\s.*`)

	clearUpCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx, chrome.EnableFeatures("PersonalizationHub", "DarkLightMode"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(clearUpCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(clearUpCtx, s.OutDir(), s.HasError, tconn)

	// Force Chrome to be in clamshell mode to make sure wallpaper preview is not
	// enabled.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure DUT is not in tablet mode: ", err)
	}
	defer cleanup(clearUpCtx)

	// The test has a dependency of network speed, so we give uiauto.Context ample
	// time to wait for nodes to load.
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	if err := uiauto.Combine("select Elements collection and enable daily refresh",
		personalization.OpenPersonalizationHub(ui),
		personalization.OpenWallpaperSubpage(ui),
		wallpaper.SelectCollection(ui, dlCollection),
		ui.LeftClick(nodewith.Name("Change wallpaper image daily").Role(role.ToggleButton)),
		ui.WaitUntilExists(nodewith.Name("Refresh the current wallpaper image").Role(role.Button)))(ctx); err != nil {
		s.Fatal("Failed to enable daily refresh: ", err)
	}

	darkWallpaper := nodewith.Role(role.Heading).HasClass("preview-text-container").NameRegex(darkRegex)
	if err := uiauto.Combine("enable dark mode and validate daily refresh wallpaper",
		personalization.NavigateHome(ui),
		personalization.ToggleDarkMode(ui),
		personalization.OpenWallpaperSubpage(ui),
		ui.WaitUntilExists(darkWallpaper))(ctx); err != nil {
		s.Fatal("Failed to validate daily refresh dark mode wallpaper: ", err)
	}

	dailyRefreshWallpaper := nodewith.Role(role.Heading).HasClass("preview-text-container").NameStartingWith("Currently set Daily Refresh")
	if err := uiauto.Combine("enable light mode and validate daily refresh wallpaper",
		personalization.NavigateHome(ui),
		personalization.ToggleLightMode(ui),
		personalization.OpenWallpaperSubpage(ui),
		ui.WaitUntilExists(dailyRefreshWallpaper))(ctx); err != nil {
		s.Fatal("Failed to validate daily refresh wallpaper: ", err)
	}

	darkWallpaperFound, err := ui.IsNodeFound(ctx, darkWallpaper)
	if err != nil {
		s.Fatal("Failed to search for darkWallpaper node: ", err)
	}
	if darkWallpaperFound {
		s.Fatal("Failed to validate daily refresh light mode wallpaper, daily refresh wallpaper is still in dark mode")
	}
}
