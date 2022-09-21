// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AppInfoWebStore,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test Web Store app info from the context menu on app list",
		Contacts: []string{
			"jinrongwu@google.com",
			"chromeos-apps-foundation-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

func AppInfoWebStore(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	const (
		newTab          = "New tab"
		appInfo         = "App info"
		webStoreSubpage = "Web Store subpage back button"
		pinToShelf      = "Pin to shelf"
		permissions     = "Permissions"
		storage         = "Identify and eject storage devices"
		manage          = "Manage your apps, extensions, and themes"
	)

	newWindowMenu := nodewith.Name(newTab).Role(role.MenuItem)
	pinToShelfMenu := nodewith.Name(pinToShelf).Role(role.MenuItem)
	appInfoMenu := nodewith.Name(appInfo).Role(role.MenuItem)

	settings := nodewith.Name("Settings").Role(role.Window).First()
	webStoreSubpageButton := nodewith.Name(webStoreSubpage).Role(role.Button).Ancestor(settings)
	toggleButton := nodewith.Name(pinToShelf).Role(role.ToggleButton).Ancestor(settings)
	permissionTxt := nodewith.Name(permissions).Role(role.StaticText).Ancestor(settings)
	storageTxt := nodewith.Name(storage).Role(role.StaticText).Ancestor(settings)
	manageTxt := nodewith.Name(manage).Role(role.StaticText).Ancestor(settings)
	launcherApp := launcher.AppItemViewFinder(apps.WebStore.Name).First()

	ui := uiauto.New(tconn)
	checkMenuAndSettings := uiauto.Combine("check app context menu and settings",
		ui.WaitUntilExists(newWindowMenu),
		ui.WaitUntilExists(pinToShelfMenu),
		ui.LeftClick(appInfoMenu),
		ui.WaitUntilExists(webStoreSubpageButton),
		ui.WaitUntilExists(toggleButton),
		ui.WaitUntilExists(permissionTxt),
		ui.WaitUntilExists(storageTxt),
		ui.WaitUntilExists(manageTxt))

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := uiauto.Combine("check context menu of Web Store app on app list",
		launcher.Open(tconn),
		ui.RightClick(launcherApp),
		checkMenuAndSettings)(ctx); err != nil {
		s.Fatal("Failed to check app info for Web Store app: ", err)
	}
}
