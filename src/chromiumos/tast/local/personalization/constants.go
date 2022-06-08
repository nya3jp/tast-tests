// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package personalization

import (
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// PersonalizationHubWindow is the finder to find the Personalization Hub window in the UI.
var PersonalizationHubWindow = nodewith.NameContaining("Wallpaper & style").Role(role.Window).First()

// SetPersonalizationMenu is the finder to find the Set Wallpaper & Style menu item after
// right click from desktop.
var SetPersonalizationMenu = nodewith.NameContaining("Set wallpaper").Role(role.MenuItem)

const (
	// Personalization Hub Name
	Personalization = "Personalization"

	// SettingsAppName is the name of settings app.
	SettingsAppName = "Settings, Installed App"
	// SettingsSetWallpaper is an option title in Settings app to open Personalization hub.
	SettingsSetWallpaper = "Set your wallpaper"

	// WallpaperSubpageName is the name of wallpaper subpage.
	WallpaperSubpageName = "Wallpaper"
	// ScreensaverSubpageName is the name of screen saver subpage.
	ScreensaverSubpageName = "Screen saver"
	// AvatarSubpageName is the name of avatar subpage.
	AvatarSubpageName = "Avatar"

	// ChangeWallpaper is the name of navigation button to open wallpaper subpage.
	ChangeWallpaper = "Change wallpaper"
	// ChangeScreensaver is the name of navigation button to open screensaver subpage.
	ChangeScreensaver = "Change screen saver"
	// ChangeAvatar is the name of navigation button to open avatar subpage.
	ChangeAvatar = "Change avatar"

	// DarkModeName is the name of dark color mode.
	DarkModeName = "Dark"
	// LightModeName is the name of light color mode.
	LightModeName = "Light"
	// AutoModeName is the name of auto color mode.
	AutoModeName = "Auto"

	// WallpaperSearchTerm is a sample search term to search for Wallpaper subpage.
	WallpaperSearchTerm = "change wallpaper"
	// PersonalizationSearchTerm is a sample search term to search for Personalization app.
	PersonalizationSearchTerm = "personalization hub"
	// SettingsSearchTerm is a sample search term to search for Settings app.
	SettingsSearchTerm = "settings"
)
