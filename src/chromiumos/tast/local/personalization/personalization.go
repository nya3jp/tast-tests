// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package personalization supports interaction with ChromeOS personalization app.
package personalization

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
)

// OpenPersonalizationHub returns an action to open the personalization app.
func OpenPersonalizationHub(ui *uiauto.Context) uiauto.Action {
	setPersonalizationMenu := nodewith.Name("Personalize").Role(role.MenuItem)
	return ui.RetryUntil(uiauto.Combine("open personalization hub",
		ui.RightClick(nodewith.HasClass("WallpaperView")),
		ui.WithInterval(300*time.Millisecond).LeftClickUntil(setPersonalizationMenu, ui.Gone(setPersonalizationMenu))),
		ui.Exists(nodewith.NameContaining("Personalization").Role(role.Window).First()))
}

// OpenWallpaperSubpage returns an action to open the wallpaper subpage.
func OpenWallpaperSubpage(ui *uiauto.Context) uiauto.Action {
	return OpenSubpage("Change wallpaper", ui)
}

// OpenScreensaverSubpage returns an action to open the screensaver subpage.
func OpenScreensaverSubpage(ui *uiauto.Context) uiauto.Action {
	return OpenSubpage("Change screensaver", ui)
}

// OpenAvatarSubpage returns an action to open the avatar subpage.
func OpenAvatarSubpage(ui *uiauto.Context) uiauto.Action {
	return OpenSubpage("Change avatar", ui)
}

// OpenSubpage returns an action to open a subpage from personalization hub main page.
func OpenSubpage(subpageButton string, ui *uiauto.Context) uiauto.Action {
	changeSubpageButton := nodewith.Role(role.Button).Name(subpageButton)
	return uiauto.Combine(fmt.Sprintf("click subpage button - %s", subpageButton),
		ui.WaitUntilExists(changeSubpageButton),
		ui.LeftClick(changeSubpageButton))
}

// ToggleLightMode returns an action to enable light color mode.
func ToggleLightMode(ui *uiauto.Context) uiauto.Action {
	return ToggleThemeButton("Enable light color mode", ui)
}

// ToggleDarkMode returns an action to enable dark color mode.
func ToggleDarkMode(ui *uiauto.Context) uiauto.Action {
	return ToggleThemeButton("Enable dark color mode", ui)
}

// ToggleThemeButton returns an action to toggle a theme button.
func ToggleThemeButton(themeButton string, ui *uiauto.Context) uiauto.Action {
	toggleThemeButton := nodewith.Role(role.ToggleButton).Name(themeButton)
	return uiauto.Combine(fmt.Sprintf("toggle theme button - %s", themeButton),
		ui.WaitUntilExists(toggleThemeButton),
		ui.LeftClick(toggleThemeButton))
}

// NavigateHome returns an action to navigate Personalization Hub Main page.
func NavigateHome(ui *uiauto.Context) uiauto.Action {
	homeButton := nodewith.Role(role.Button).Name("Home")
	return uiauto.Combine("click home button",
		ui.WaitUntilExists(homeButton),
		ui.LeftClick(homeButton),
		ui.Exists(nodewith.NameContaining("Personalization").Role(role.Window).First()))
}

// NavigateBreadcrumb returns an action to navigate to a desired page using breadcrumb
func NavigateBreadcrumb(breadcrumb string, ui *uiauto.Context) uiauto.Action {
	breadcrumbButton := nodewith.Role(role.Button).Name(breadcrumb).HasClass("breadcrumb")
	return uiauto.Combine(fmt.Sprintf("click breadcrumb button - %s", breadcrumb),
		ui.WaitUntilExists(breadcrumbButton),
		ui.LeftClick(breadcrumbButton))
}

// ClosePersonalizationHub returns an action to close the personalization hub via the Ctrl+W shortcut.
func ClosePersonalizationHub() uiauto.Action {
	return func(ctx context.Context) error {
		kb, err := input.VirtualKeyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get virtual keyboard")
		}
		defer kb.Close()
		return kb.Accel(ctx, "Ctrl+W")
	}
}
