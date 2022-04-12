// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package personalization supports interaction with ChromeOS personalization app.
package personalization

import (
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
func OpenWallpaperSubpage(ui *uiauto.Context) uiauto.Action {
	changeWallpaper := nodewith.Role(role.Button).Name("Change wallpaper")
	return uiauto.Combine("click change wallpaper button",
		ui.WaitUntilExists(changeWallpaper),
		ui.LeftClick(changeWallpaper))
}

// OpenScreensaverSubpage returns an action to open the screensaver subpage.
func OpenScreensaverSubpage(ui *uiauto.Context) uiauto.Action {
	changeScreensaver := nodewith.Role(role.Button).Name("Change screensaver")
	return uiauto.Combine("click change screensaver button",
		ui.WaitUntilExists(changeScreensaver),
		ui.LeftClick(changeScreensaver))
}

// OpenAvatarSubpage returns an action to open the avatar subpage.
func OpenAvatarSubpage(ui *uiauto.Context) uiauto.Action {
	changeAvatar := nodewith.Role(role.Button).Name("Change avatar")
	return uiauto.Combine("click change avatar button",
		ui.WaitUntilExists(changeAvatar),
		ui.LeftClick(changeAvatar))
}
