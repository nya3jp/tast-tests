// Copyright 2022 The ChromiumOS Authors
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

// OpenPersonalizationHub returns an action to open the personalization app by right clicking on the desktop.
func OpenPersonalizationHub(ui *uiauto.Context) uiauto.Action {
	return uiauto.Retry(3, uiauto.Combine("Open personalization hub from right click desktop",
		// Open the menu by right click on desktop random coord in upper left corner.
		ui.WithInterval(500*time.Millisecond).RetryUntil(ui.MouseClickAtLocation(1, coords.Point{X: rand.Intn(200), Y: rand.Intn(200)}), ui.Exists(SetPersonalizationMenu)),
		// Click the menu item to open Personalization.
		ui.WithTimeout(500*time.Millisecond).RetryUntil(ui.LeftClick(SetPersonalizationMenu), ui.Gone(SetPersonalizationMenu)),
		// Wait for Personalization window to appear.
		ui.WithTimeout(3*time.Second).WaitUntilExists(PersonalizationHubWindow),
	))
}

// OpenWallpaperSubpage returns an action to open the wallpaper subpage.
// Reference: aria-label="$i18n{ariaLabelChangeWallpaper}"
// ash/webui/personalization_app/resources/trusted/wallpaper/wallpaper_preview_element.html
func OpenWallpaperSubpage(ui *uiauto.Context) uiauto.Action {
	return openSubpage(ChangeWallpaper, ui)
}

// OpenScreensaverSubpage returns an action to open the screensaver subpage.
// Reference: aria-label="$i18n{ariaLabelChangeScreensaver}"
// ash/webui/personalization_app/resources/trusted/personalization_main_element.html
func OpenScreensaverSubpage(ui *uiauto.Context) uiauto.Action {
	return openSubpage(ChangeScreensaver, ui)
}

// OpenAvatarSubpage returns an action to open the avatar subpage.
// Reference: aria-label="$i18n{ariaLabelChangeAvatar}"
// ash/webui/personalization_app/resources/trusted/user/user_preview_element.html
func OpenAvatarSubpage(ui *uiauto.Context) uiauto.Action {
	return openSubpage(ChangeAvatar, ui)
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
// Reference: ash/webui/personalization_app/resources/trusted/personalization_theme_element.html
func ToggleLightMode(ui *uiauto.Context) uiauto.Action {
	return toggleThemeButton(LightModeName, ui)
}

// ToggleDarkMode returns an action to enable dark color mode.
// Reference: ash/webui/personalization_app/resources/trusted/personalization_theme_element.html
func ToggleDarkMode(ui *uiauto.Context) uiauto.Action {
	return toggleThemeButton(DarkModeName, ui)
}

// ToggleAutoMode returns an action to enable auto color mode.
func ToggleAutoMode(ui *uiauto.Context) uiauto.Action {
	return toggleThemeButton(AutoModeName, ui)
}

// toggleThemeButton returns an action to toggle a theme button.
func toggleThemeButton(themeButton string, ui *uiauto.Context) uiauto.Action {
	toggleThemeButton := nodewith.Role(role.ToggleButton).Name(themeButton)
	return uiauto.Combine(fmt.Sprintf("toggle theme button - %s", themeButton),
		ui.WaitUntilExists(toggleThemeButton),
		ui.LeftClick(toggleThemeButton),
		// Wait for a second as the system may take some time to update its UI.
		uiauto.Sleep(time.Second))
}

// NavigateHome returns an action to navigate Personalization Hub Main page.
func NavigateHome(ui *uiauto.Context) uiauto.Action {
	homeButton := nodewith.Role(role.Link).Name("Home")
	return uiauto.Combine("click home button",
		ui.WaitUntilExists(homeButton),
		ui.LeftClick(homeButton),
		ui.Exists(PersonalizationHubWindow))
}

// NavigateBreadcrumb returns an action to navigate to a desired page using breadcrumb.
func NavigateBreadcrumb(breadcrumb string, ui *uiauto.Context) uiauto.Action {
	return uiauto.Combine(fmt.Sprintf("click breadcrumb button - %s", breadcrumb),
		ui.LeftClick(BreadcrumbNodeFinder(breadcrumb)))
}

// BreadcrumbNodeFinder finds a specific breadcrumb link node.
func BreadcrumbNodeFinder(breadcrumb string) *nodewith.Finder {
	return nodewith.Role(role.Link).Name(breadcrumb).HasClass("breadcrumb")
}

// SearchForAppInLauncher returns an action to search and select result in launcher.
func SearchForAppInLauncher(query, result string, kb *input.KeyboardEventWriter, ui *uiauto.Context) uiauto.Action {
	searchResult := nodewith.Role("listBoxOption").NameContaining(result).HasClass("ui/app_list/SearchResultView").First()
	return ui.RetrySilently(2, uiauto.Combine("search and select result in launcher",
		kb.AccelAction("Search"),
		ui.WaitUntilExists(nodewith.Role(role.TextField).HasClass("Textfield")),
		kb.TypeAction(query),
		ui.LeftClick(searchResult),
	))
}
