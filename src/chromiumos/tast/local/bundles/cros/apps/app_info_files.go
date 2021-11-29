// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AppInfoFiles,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test Files app info from the context menu on shelf and app list",
		Contacts: []string{
			"jinrongwu@google.com",
			"chromeos-apps-foundation-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

func AppInfoFiles(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	const (
		newWindow      = "New window"
		unpin          = "Unpin"
		unpinFromShelf = "Unpin from shelf"
		appInfo        = "App info"
		filesSubpage   = "Files subpage back button"
		pinToShelf     = "Pin to shelf"
		permissions    = "Permissions"
		note           = "Display notifications"
		read           = "Read and modify data you copy and paste"
		store          = "Store data in your Google Drive account"
		wallpaper      = "Change your wallpaper"
	)

	newWindowMenu := nodewith.Name(newWindow).Role(role.MenuItem)
	unpinMenu := nodewith.Name(unpin).Role(role.MenuItem)
	appInfoMenu := nodewith.Name(appInfo).Role(role.MenuItem)

	settings := nodewith.Name("Settings").Role(role.Window).First()
	filesSubpageButton := nodewith.Name(filesSubpage).Role(role.Button).Ancestor(settings)
	toggleButton := nodewith.Name(pinToShelf).Role(role.ToggleButton).Ancestor(settings)
	permissionTxt := nodewith.Name(permissions).Role(role.StaticText).Ancestor(settings)
	noteTxt := nodewith.Name(note).Role(role.StaticText).Ancestor(settings)
	readTxt := nodewith.Name(read).Role(role.StaticText).Ancestor(settings)
	storeTxt := nodewith.Name(store).Role(role.StaticText).Ancestor(settings)
	wallpaperTxt := nodewith.Name(wallpaper).Role(role.StaticText).Ancestor(settings)

	ui := uiauto.New(tconn)
	checkMenuAndSettings := func() uiauto.Action {
		return uiauto.Combine("check app context menu and settings",
			ui.WaitUntilExists(newWindowMenu),
			ui.WaitUntilExists(unpinMenu),
			ui.LeftClick(appInfoMenu),
			ui.WaitUntilExists(filesSubpageButton),
			ui.WaitUntilExists(toggleButton),
			ui.WaitUntilExists(permissionTxt),
			ui.WaitUntilExists(noteTxt),
			ui.WaitUntilExists(readTxt),
			ui.WaitUntilExists(storeTxt),
			ui.WaitUntilExists(wallpaperTxt))
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Run(ctx, "check_files_appinfo_on_shelf", func(ctx context.Context, s *testing.State) {
		if err := uiauto.Combine("check context menu of Files app on the shelf",
			ash.RightClickApp(tconn, apps.Files.Name),
			checkMenuAndSettings())(ctx); err != nil {
			s.Fatal("Failed to check app info for Files app: ", err)
		}
		if err := apps.Close(ctx, tconn, apps.Settings.ID); err != nil {
			s.Error("Failed to close settings: ", err)
		}
	})

	unpinMenu = unpinMenu.Name(unpinFromShelf)

	s.Run(ctx, "check_files_appinfo_on_app_list", func(ctx context.Context, s *testing.State) {
		kb, err := input.Keyboard(ctx)
		if err != nil {
			s.Fatal("Failed to find keyboard: ", err)
		}
		defer kb.Close()
		if err := uiauto.Combine("check context menu of Files app on app list",
			launcher.SearchAndRightClick(tconn, kb, apps.Files.Name, apps.Files.Name),
			checkMenuAndSettings())(ctx); err != nil {
			s.Fatal("Failed to check app info for Files app: ", err)
		}
	})
}
