// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui2"
	"chromiumos/tast/local/chrome/ui2/nodewith"
	"chromiumos/tast/local/chrome/ui2/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChangeWallpaper,
		Desc: "Follows the user flow to change the wallpaper",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

func ChangeWallpaper(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ac := ui2.New(tconn)
	setWallpaper := nodewith.Name("Set wallpaper").Role(role.MenuItem)
	if err := ui2.Run(ctx,
		ac.RightClick(nodewith.ClassName("WallpaperView")),
		// This button takes a bit before it is clickable.
		// Keep clicking it until the click is received and the menu closes.
		ac.WithInterval(500*time.Millisecond).LeftClickUntil(ac.Gone(setWallpaper), setWallpaper),
		ac.LeftClick(nodewith.Name("Solid colors").Role(role.StaticText)),
		ac.LeftClick(nodewith.Name("Deep Purple").Role(role.ListItem)),
		// Ensure that "Deep Purple" text is displayed.
		// The UI displays the name of the currently set wallpaper.
		ac.WaitUntilExists(nodewith.Name("Deep Purple").Role(role.StaticText)),
	); err != nil {
		s.Fatal("Failed to chaneg the wallpaper: ", err)
	}
}
