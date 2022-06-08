// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wallpaper

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Change,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Follows the user flow to change the wallpaper",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Fixture:      "chromeLoggedIn",
	})
}

func Change(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)
	personalizeMenu := nodewith.Name("Set wallpaper  style").Role(role.MenuItem)
	changeWallpaperButton := nodewith.Name("Change wallpaper").Role(role.Button)
	solidColorsMenu := nodewith.NameContaining("Element").Role(role.ListBoxOption).HasClass("photo-inner-container")
	if err := uiauto.Combine("change the wallpaper",
		ui.RightClick(nodewith.HasClass("WallpaperView")),
		// This button takes a bit before it is clickable.
		// Keep clicking it until the click is received and the menu closes.
		ui.WithInterval(time.Second).LeftClickUntil(personalizeMenu, ui.Gone(personalizeMenu)),
		ui.Exists(nodewith.NameContaining("Wallpaper & style").Role(role.Window).First()),
		ui.LeftClick(changeWallpaperButton),
		ui.FocusAndWait(solidColorsMenu),
		ui.MakeVisible(solidColorsMenu),
		ui.LeftClick(solidColorsMenu),
		ui.LeftClick(nodewith.NameContaining("Wind Light").Role(role.ListBoxOption)),
		// Ensure that "Wind Light" text is displayed.
		// The UI displays the name of the currently set wallpaper.
		ui.WaitUntilExists(nodewith.NameContaining("Wind Light").Role(role.Heading)),
	)(ctx); err != nil {
		s.Fatal("Failed to change the wallpaper: ", err)
	}
}
