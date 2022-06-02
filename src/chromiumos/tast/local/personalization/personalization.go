// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package personalization supports interaction with ChromeOS personalization app.
package personalization

import (
	"fmt"
	"math/rand"
	"time"

	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
)

// PersonalizationHubWindow is the finder to find the Personalization Hub window in the UI.
var PersonalizationHubWindow = nodewith.NameContaining("Wallpaper & style").Role(role.Window).First()

// OpenPersonalizationHub returns an action to open the personalization app.
func OpenPersonalizationHub(ui *uiauto.Context) uiauto.Action {
	setPersonalizationMenu := nodewith.NameContaining("Set wallpaper").Role(role.MenuItem)
	return ui.RetryUntil(uiauto.Combine("open personalization hub",
		ui.MouseClickAtLocation(1, coords.Point{X: rand.Intn(200), Y: rand.Intn(200)}), // right click a random pixel
		ui.WithInterval(300*time.Millisecond).LeftClickUntil(setPersonalizationMenu, ui.Gone(setPersonalizationMenu))),
		ui.Exists(PersonalizationHubWindow))
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
	return openSubpage("Change screen saver", ui)
}

// OpenAvatarSubpage returns an action to open the avatar subpage.
// Reference: aria-label="$i18n{ariaLabelChangeAvatar}"
// ash/webui/personalization_app/resources/trusted/user/user_preview_element.html
func OpenAvatarSubpage(ui *uiauto.Context) uiauto.Action {
	return openSubpage("Change avatar", ui)
}

// openSubpage returns an action to open a subpage from personalization hub main page.
func openSubpage(subpageButton string, ui *uiauto.Context) uiauto.Action {
	changeSubpageButton := nodewith.Name(subpageButton).HasClass("tast-open-subpage")
	return uiauto.Combine(fmt.Sprintf("click subpage button - %s", subpageButton),
		ui.WaitUntilExists(changeSubpageButton),
		ui.LeftClick(changeSubpageButton))
}

// ClosePersonalizationHub returns an action to close the personalization hub by clicking on Close button.
func ClosePersonalizationHub(ui *uiauto.Context) uiauto.Action {
	closeButton := nodewith.Role(role.Button).Name("Close")
	return uiauto.Combine("close Personalization Hub",
		ui.LeftClick(closeButton),
		ui.WaitUntilGone(PersonalizationHubWindow))
}

// ToggleLightMode returns an action to enable light color mode.
// Reference: aria-label="$i18n{ariaLabelEnableLightColorMode}"
// ash/webui/personalization_app/resources/trusted/personalization_theme_element.html
func ToggleLightMode(ui *uiauto.Context) uiauto.Action {
	return toggleThemeButton("Light", ui)
}

// ToggleDarkMode returns an action to enable dark color mode.
// Reference: aria-label="$i18n{ariaLabelEnableDarkColorMode}"
// ash/webui/personalization_app/resources/trusted/personalization_theme_element.html
func ToggleDarkMode(ui *uiauto.Context) uiauto.Action {
	return toggleThemeButton("Dark", ui)
}

// ToggleAutoMode returns an action to enable auto color mode.
func ToggleAutoMode(ui *uiauto.Context) uiauto.Action {
	return toggleThemeButton("Auto", ui)
}

// toggleThemeButton returns an action to toggle a theme button.
func toggleThemeButton(themeButton string, ui *uiauto.Context) uiauto.Action {
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
		ui.Exists(PersonalizationHubWindow))
}

// NavigateBreadcrumb returns an action to navigate to a desired page using breadcrumb.
func NavigateBreadcrumb(breadcrumb string, ui *uiauto.Context) uiauto.Action {
	breadcrumbButton := nodewith.Role(role.Button).Name(breadcrumb).HasClass("breadcrumb")
	return uiauto.Combine(fmt.Sprintf("click breadcrumb button - %s", breadcrumb),
		ui.LeftClick(breadcrumbButton))
}

// SearchForAppInLauncher returns an action to search and select result in launcher.
func SearchForAppInLauncher(query, result string, kb *input.KeyboardEventWriter, ui *uiauto.Context) uiauto.Action {
	searchResult := nodewith.Role("listBoxOption").NameContaining(result).HasClass("ui/app_list/SearchResultView").First()
	return uiauto.Combine("search and select result in launcher",
		kb.AccelAction("Search"),
		ui.WaitUntilExists(nodewith.Role(role.TextField).HasClass("Textfield")),
		kb.TypeAction(query),
		ui.LeftClick(searchResult),
	)
}
