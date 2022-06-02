// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package personalization

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
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
		Func:         SetAutoDarkLightWallpaper,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test setting auto theme D/L wallpapers in the personalization hub app",
		Contacts: []string{
			"thuongphan@google.com",
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
		Fixture:      "personalizationWithDarkLightMode",
		Params: []testing.Param{
			{
				Name: "dark",
				Val:  true,
			},
			{
				Name: "light",
				Val:  false,
			},
		},
	})
}

func SetAutoDarkLightWallpaper(ctx context.Context, s *testing.State) {
	const dlCollection = "Element"

	isDarkModeOn := s.Param().(bool)
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

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
		personalization.ToggleAutoMode(ui),
	)(ctx); err != nil {
		s.Fatal("Failed to enable auto mode: ", err)
	}

	if err := tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.forceAutoThemeMode)`, isDarkModeOn); err != nil {
		s.Fatalf("Failed to force auto theme mode for testing, dark mode - %v: %v", isDarkModeOn, err)
	}

	// Open Elements collection.
	if err := uiauto.Combine(fmt.Sprintf("Select wallpaper from %v collection", dlCollection),
		personalization.OpenWallpaperSubpage(ui),
		wallpaper.SelectCollection(ui, dlCollection),
		ui.WaitUntilExists(nodewith.Role(role.Button).NameContaining(dlCollection).HasClass("breadcrumb")),
	)(ctx); err != nil {
		s.Fatalf("Failed to select collection %v : %v", dlCollection, err)
	}

	if err := selectDLWallpaper(ctx, ui, dlCollection); err != nil {
		s.Fatalf("Failed to select an image in %v collection as wallpaper: %v", dlCollection, err)
	}

	currentWallpaper, err := wallpaper.CurrentlySetWallpaper(ctx, ui)
	if err != nil {
		s.Fatal("Failed to get current wallpaper name: ", err)
	}

	if isDarkModeOn && !strings.Contains(currentWallpaper, "Dark") {
		s.Fatal("Failed to set wallpaper to dark mode, current wallpaper: ", currentWallpaper)
	}

	if !isDarkModeOn && !strings.Contains(currentWallpaper, "Light") {
		s.Fatal("Failed to set wallpaper to light mode, current wallpaper: ", currentWallpaper)
	}

	// var isNowSunset bool
	// if err := tconn.Call(ctx, &isNowSunset, `tast.promisify(chrome.autotestPrivate.isNowWithinSunsetSunrise)`); err != nil {
	// 	s.Fatal("failed to check if current time is sunset or sunrise: ", err)
	// }
	// if isNowSunset && !strings.Contains(currentWallpaper, "Dark") {
	// 	s.Fatal("current wallpaper should be in dark mode, sunset: %v", isNowSunset)
	// }
	// if !isNowSunset && !strings.Contains(currentWallpaper, "Light") {
	// 	s.Fatal("current wallpaper should be in light mode, sunset: %v", isNowSunset)
	// }
}

func selectDLWallpaper(ctx context.Context, ui *uiauto.Context, collection string) error {
	// Select second image as wallpaper.
	imagesFinder := nodewith.Role(role.ListBoxOption).HasClass("photo-inner-container")

	images, err := ui.NodesInfo(ctx, imagesFinder)
	if err != nil {
		return errors.Wrapf(err, "failed to find images in %v collection", collection)
	}
	if len(images) < 8 {
		return errors.Errorf("at least 8 image options for %v collection expected", collection)
	}

	// Select 3rd image as wallpaper.
	for i, image := range images {
		if i == 2 {
			if err := uiauto.Combine("set D/L wallpaper",
				ui.MouseClickAtLocation(0, image.Location.CenterPoint()),
				ui.WaitUntilExists(nodewith.Role(role.Heading).NameContaining(image.Name)),
			)(ctx); err != nil {
				return errors.Wrapf(err, "failed to select Element image %v", image.Name)
			}
			break
		}
	}
	return nil
}
