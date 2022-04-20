// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package personalization supports interaction with ChromeOS personalization app.
package personalization

import (
	"fmt"
	"time"

	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
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
// Reference: aria-label="$i18n{ariaLabelChangeWallpaper}"
// ash/webui/personalization_app/resources/trusted/wallpaper/wallpaper_preview_element.html
func OpenWallpaperSubpage(ui *uiauto.Context) uiauto.Action {
	return openSubpage("Change wallpaper", ui)
}

// OpenScreensaverSubpage returns an action to open the screensaver subpage.
// Reference: aria-label="$i18n{ariaLabelChangeScreensaver}"
// ash/webui/personalization_app/resources/trusted/personalization_main_element.html
func OpenScreensaverSubpage(ui *uiauto.Context) uiauto.Action {
	return openSubpage("Change screensaver", ui)
}

// OpenAvatarSubpage returns an action to open the avatar subpage.
// Reference: aria-label="$i18n{ariaLabelChangeAvatar}"
// ash/webui/personalization_app/resources/trusted/user/user_preview_element.html
func OpenAvatarSubpage(ui *uiauto.Context) uiauto.Action {
	return openSubpage("Change avatar", ui)
}

// openSubpage returns an action to open a subpage from personalization hub main page.
func openSubpage(subpageButton string, ui *uiauto.Context) uiauto.Action {
	changeSubpageButton := nodewith.Role(role.Button).Name(subpageButton)
	return uiauto.Combine(fmt.Sprintf("click subpage button - %s", subpageButton),
		ui.WaitUntilExists(changeSubpageButton),
		ui.LeftClick(changeSubpageButton))
}

// ToggleLightMode returns an action to enable light color mode.
// Reference: aria-label="$i18n{ariaLabelEnableLightColorMode}"
// ash/webui/personalization_app/resources/trusted/personalization_theme_element.html
func ToggleLightMode(ui *uiauto.Context) uiauto.Action {
	return toggleThemeButton("Enable light color mode", ui)
}

// ToggleDarkMode returns an action to enable dark color mode.
// Reference: aria-label="$i18n{ariaLabelEnableDarkColorMode}"
// ash/webui/personalization_app/resources/trusted/personalization_theme_element.html
func ToggleDarkMode(ui *uiauto.Context) uiauto.Action {
	return toggleThemeButton("Enable dark color mode", ui)
}

// toggleThemeButton returns an action to toggle a theme button.
func toggleThemeButton(themeButton string, ui *uiauto.Context) uiauto.Action {
	toggleThemeButton := nodewith.Role(role.ToggleButton).Name(themeButton)
	return uiauto.Combine(fmt.Sprintf("toggle theme button - %s", themeButton),
		ui.WaitUntilExists(toggleThemeButton),
		ui.LeftClick(toggleThemeButton))
}

// EnableAmbientMode returns an action to enable ambient mode.
func EnableAmbientMode(ui *uiauto.Context) uiauto.Action {
	return toggleAmbientMode("Off", ui)
}

// toggleAmbientMode returns an action to toggle ambient mode.
func toggleAmbientMode(currentMode string, ui *uiauto.Context) uiauto.Action {
	toggleAmbientButton := nodewith.Role(role.ToggleButton).Name(currentMode)
	return uiauto.Combine(fmt.Sprintf("toggle ambient mode - %s", currentMode),
		ui.WaitUntilExists(toggleAmbientButton),
		ui.LeftClick(toggleAmbientButton))
}

// NavigateHome returns an action to navigate Personalization Hub Main page.
func NavigateHome(ui *uiauto.Context) uiauto.Action {
	homeButton := nodewith.Role(role.Button).Name("Home")
	return uiauto.Combine("click home button",
		ui.WaitUntilExists(homeButton),
		ui.LeftClick(homeButton),
		ui.Exists(nodewith.NameContaining("Personalization").Role(role.Window).First()))
}
