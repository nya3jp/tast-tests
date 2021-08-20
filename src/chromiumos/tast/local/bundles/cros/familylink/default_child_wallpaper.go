// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package familylink

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DefaultChildWallpaper,
		Desc: "Verifies Unicorn users can change the wallpaper and sync",
		Contacts: []string{
			"tobyhuang@chromium.org",
			"cros-families-eng+test@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Fixture:      "familyLinkUnicornLogin",
	})
}

func DefaultChildWallpaper(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*familylink.FixtData).TestConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)
	setWallpaperMenu := nodewith.Name("Set wallpaper").Role(role.MenuItem)
	closeButton := nodewith.Name("Close").Role(role.MenuItem)
	s.Log("Waiting for Deep Purple wallpaper to sync")
	for attempts := 1; ; attempts++ {
		if err := uiauto.Combine("open wallpaper picker",
			ui.RightClick(nodewith.ClassName("WallpaperView")),
			// This button takes a bit before it is clickable.
			// Keep clicking it until the click is received and the menu closes.
			ui.WithInterval(500*time.Millisecond).LeftClickUntil(setWallpaperMenu, ui.Gone(setWallpaperMenu)),
			// Wait until the wallpaper turns to "Deep Purple" through Chrome sync.
			// The UI displays the name of the currently set wallpaper.
			ui.WaitUntilExists(nodewith.Name("Deep Purple").Role(role.StaticText)))(ctx); err == nil {
			s.Log("Successfully synced Deep Purple wallpaper")
			break
		}
		s.Logf("%d attempts to sync wallpaper failed, trying again", attempts)
		if err := uiauto.Combine("close wallpaper picker",
			ui.RightClick(nodewith.Name("Wallpaper Picker").Role(role.Button)),
			ui.WaitUntilExists(closeButton),
			ui.LeftClick(closeButton))(ctx); err != nil {
			s.Fatal("Failed to close the wallpaper picker: ", err)
		}
	}

	s.Log("Changing wallpaper to Imaginary")
	imaginaryMenu := nodewith.Name("Imaginary").Role(role.StaticText)
	if err := uiauto.Combine("change the wallpaper",
		ui.WaitUntilExists(imaginaryMenu),
		ui.MakeVisible(imaginaryMenu),
		ui.LeftClick(imaginaryMenu),
		ui.LeftClick(nodewith.Name("Next Level!").Role(role.ListItem)),
		// Ensure that "Next Level!" text is displayed.
		// The UI displays the name of the currently set wallpaper.
		ui.WaitUntilExists(nodewith.Name("Next Level!").Role(role.StaticText)),
	)(ctx); err != nil {
		s.Fatal("Failed to change the wallpaper: ", err)
	}

	s.Log("Changing wallpaper back to Deep Purple for the next test")
	solidColorsMenu := nodewith.Name("Solid colors").Role(role.StaticText)
	if err := uiauto.Combine("change the wallpaper",
		ui.WaitUntilExists(solidColorsMenu),
		ui.MakeVisible(solidColorsMenu),
		ui.LeftClick(solidColorsMenu),
		ui.LeftClick(nodewith.Name("Deep Purple").Role(role.ListItem)),
		// Ensure that "Deep Purple" text is displayed.
		// The UI displays the name of the currently set wallpaper.
		ui.WaitUntilExists(nodewith.Name("Deep Purple").Role(role.StaticText)),
	)(ctx); err != nil {
		s.Fatal("Failed to change the wallpaper: ", err)
	}
}
