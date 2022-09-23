// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/personalization"
	"chromiumos/tast/local/wallpaper"
	"chromiumos/tast/local/wallpaper/constants"
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
		Fixture:      "personalizationWithClamshell",
	})
}

func SetDLThemeQuickSettings(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Force Chrome to be in clamshell mode to make sure wallpaper preview is not
	// enabled.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure DUT is not in tablet mode: ", err)
	}
	defer cleanup(cleanupCtx)

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// The test has a dependency of network speed, so we give uiauto.Context ample
	// time to wait for nodes to load.
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	// By default after log in, dark light mode is set as Auto in Personlization Hub.
	// Switch to Light Mode for the test. ToggleLightMode() won't fail even if Light
	// mode is already enabled.
	if err := uiauto.Combine("Enable light mode",
		personalization.OpenPersonalizationHub(ui),
		personalization.ToggleLightMode(ui),
	)(ctx); err != nil {
		s.Fatal("Failed to enable light mode: ", err)
	}

	if err := uiauto.Combine(fmt.Sprintf("Change to light mode wallpaper %s %s", constants.ElementCollection, constants.LightElementImage),
		personalization.OpenWallpaperSubpage(ui),
		wallpaper.SelectCollection(ui, constants.ElementCollection),
		wallpaper.SelectImage(ui, constants.LightElementImage),
		ui.WaitUntilExists(wallpaper.CurrentWallpaperWithSpecificNameFinder(constants.LightElementImage)),
		wallpaper.CloseWallpaperPicker(),
	)(ctx); err != nil {
		s.Fatalf("Failed to validate selected wallpaper %s %s: %v", constants.ElementCollection, constants.LightElementImage, err)
	}

	if err := toggleDarkThemeFromQuickSettings(ctx, tconn, ui); err != nil {
		s.Fatal("Failed to turn on dark theme in Quick Settings: ", err)
	}

	if err := validateDLWallpaper(ctx, ui, constants.DarkElementImage); err != nil {
		s.Fatalf("Failed to change to dark mode wallpaper %v: %v", constants.DarkElementImage, err)
	}

	if err := toggleDarkThemeFromQuickSettings(ctx, tconn, ui); err != nil {
		s.Fatal("Failed to turn off dark theme in Quick Settings: ", err)
	}

	if err := validateDLWallpaper(ctx, ui, constants.LightElementImage); err != nil {
		s.Fatalf("Failed to change to light mode wallpaper %v: %v", constants.LightElementImage, err)
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

	// Although Dark theme pod icon button is available in Quick Settings, we don't know the
	// exact page it resides. If we click on the Dark theme button in a wrong page, it would
	// close Quick Settings bubble. Hence, we need to reopen the bubble in case it closes and
	// try to click on the Dark theme button in all the pages.
	// TODO: update the tast test when Quick Settings adds new infrastructure to not close
	// the bubble accidentally.
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
			ui.LeftClick(darkThemePodIconButton),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to toggle Dark theme")
		}
	}
	return nil
}

func validateDLWallpaper(ctx context.Context, ui *uiauto.Context, image string) error {
	if err := wallpaper.OpenWallpaperPicker(ui)(ctx); err != nil {
		return errors.Wrap(err, "failed to open wallpaper picker")
	}

	currentWallpaper, err := wallpaper.CurrentWallpaper(ctx, ui)
	if err != nil {
		return err
	}
	if !strings.Contains(currentWallpaper, image) {
		return errors.Errorf("current wallpaper - %v is not as expected - %v", currentWallpaper, image)
	}

	if err := wallpaper.CloseWallpaperPicker()(ctx); err != nil {
		return errors.Wrap(err, "failed to close wallpaper picker")
	}

	return nil
}
