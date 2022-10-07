// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	arcpkg "chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/personalization"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DarkMode,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that ArcSystemUIService changes Settings.Secure",
		Contacts:     []string{"arc-app-dev@google.com, ttefera@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Fixture:      "arcBooted",
		Timeout:      chrome.GAIALoginTimeout + arcpkg.BootTimeout + 120*time.Second,
	})
}

func DarkMode(ctx context.Context, s *testing.State) {

	cr := s.FixtValue().(*arcpkg.PreData).Chrome
	arc := s.FixtValue().(*arcpkg.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

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

	if err := toggleDarkThemeFromQuickSettings(ctx, tconn, ui); err != nil {
		s.Fatal("Failed to turn on dark theme in Quick Settings: ", err)
	}

	cmd := arc.Command(ctx, "settings", "get", "secure", "ui_night_mode")
	output, err := cmd.Output()
	if err != nil {
		s.Fatal("Failed to get secure settings: ", err)
	}

	// The value of ui_night_mode enabled.
	const darkModeOn = "2"
	if strings.TrimSpace(string(output)) != darkModeOn {
		s.Fatalf("Night_mode wanted: 2 was %s", string(output))
	}
}

// toggleDarkThemeFromQuickSettings opens the Quick Settings and then toggles the Dark Theme.
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
