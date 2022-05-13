// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package personalization

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/personalization"
	"chromiumos/tast/local/wallpaper"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SetDLThemeQuickSettings,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test setting dark light theme from quick settings",
		Contacts: []string{
			"thuongphan@google.com",
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Fixture:      "personalizationWithDarkLightMode",
	})
}

func SetDLThemeQuickSettings(ctx context.Context, s *testing.State) {
	const (
		dlCollection = "Element"
		dImage       = "Wind Dark Digital Art by Rutger Paulusse"
		lImage       = "Wind Light Digital Art by Rutger Paulusse"
	)

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
	defer cleanup(cleanupCtx)

	// The test has a dependency of network speed, so we give uiauto.Context ample
	// time to wait for nodes to load.
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	if err := uiauto.Combine("Enable light mode",
		personalization.OpenPersonalizationHub(ui),
		personalization.ToggleLightMode(ui))(ctx); err != nil {
		s.Fatal("Failed to enable light mode: ", err)
	}

	if err := uiauto.Combine(fmt.Sprintf("Change to light mode wallpaper %s %s", dlCollection, lImage),
		personalization.OpenWallpaperSubpage(ui),
		wallpaper.SelectCollection(ui, dlCollection),
		wallpaper.SelectImage(ui, lImage),
		ui.WaitUntilExists(nodewith.Name(fmt.Sprintf("Currently set %v", lImage)).Role(role.Heading)),
		wallpaper.CloseWallpaperPicker())(ctx); err != nil {
		s.Fatalf("Failed to validate selected wallpaper %s %s: %v", dlCollection, lImage, err)
	}

	if err := toggleDarkThemeFromQuickSettings(ctx, tconn, ui); err != nil {
		s.Fatal("Failed to turn on dark theme in Quick Settings: ", err)
	}

	if err := validateDLWallpaper(ui, dImage)(ctx); err != nil {
		s.Fatalf("Failed to change to dark mode wallpaper %v: %v", dImage, err)
	}

	if err := toggleDarkThemeFromQuickSettings(ctx, tconn, ui); err != nil {
		s.Fatal("Failed to turn off dark theme in Quick Settings: ", err)
	}

	if err := validateDLWallpaper(ui, lImage)(ctx); err != nil {
		s.Fatalf("Failed to change to light mode wallpaper %v: %v", lImage, err)
	}
}

func toggleDarkThemeFromQuickSettings(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context) error {
	if err := quicksettings.Expand(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to expand quick settings")
	}

	darkThemePodIconButton := quicksettings.PodIconButton(quicksettings.SettingPodDarkTheme)
	if err := ui.WaitUntilExists(darkThemePodIconButton)(ctx); err != nil {
		return errors.Wrap(err, "dark theme pod icon button is not found")
	}

	pageIndicators := nodewith.Role(role.Button).ClassName("PageIndicatorView")
	pages, err := ui.NodesInfo(ctx, pageIndicators)
	if err != nil {
		return errors.Wrap(err, "failed to get page indicator")
	}

	// If there is no page indicator (which means only one page of pod icons in Quick Settings),
	// try to click on Dark theme pod icon button.
	if len(pages) == 0 {
		if err := ui.LeftClick(darkThemePodIconButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to toggle Dark theme")
		}
		return nil
	}

	// Although Dark theme pod icon button is available in Quick Settings, we don't know the exact page it resides.
	// If we click on the Dark theme button in a wrong page, it takes no effects.
	// So try to click on the Dark theme button in all the pages.
	for _, page := range pages {
		if shown, err := quicksettings.Shown(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to check quick settings visibility status")
		} else if !shown {
			if err := quicksettings.Expand(ctx, tconn); err != nil {
				return errors.Wrap(err, "failed to expand quick settings")
			}
		}
		if err := uiauto.Combine("Toggle Dark theme",
			ui.LeftClick(nodewith.Role(page.Role).ClassName(page.ClassName).Name(page.Name)),
			ui.LeftClick(darkThemePodIconButton))(ctx); err != nil {
			return errors.Wrap(err, "failed to toggle Dark theme")
		}
	}
	return nil
}

func validateDLWallpaper(ui *uiauto.Context, image string) uiauto.Action {
	return uiauto.Combine("Validate that wallpaper has changed",
		wallpaper.OpenWallpaperPicker(ui),
		ui.WaitUntilExists(nodewith.Name(fmt.Sprintf("Currently set %v", image)).Role(role.Heading)),
		wallpaper.CloseWallpaperPicker())
}
