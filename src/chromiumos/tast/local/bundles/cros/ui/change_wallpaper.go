// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChangeWallpaper,
		Desc: "Follows the user flow to change the wallpaper",
		Contacts: []string{
			"bhansknecht@chromium.org",
			"kyleshima@chromium.org",
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

	// Right click the wallpaper.
	params := ui.FindParams{ClassName: "WallpaperView"}
	wallpaperView, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find the wallpaper view: ", err)
	}
	defer wallpaperView.Release(ctx)

	if err := wallpaperView.RightClick(ctx); err != nil {
		s.Fatal("Failed to right click the wallpaper view: ", err)
	}

	// Open wallpaper picker by clicking set wallpaper.
	params = ui.FindParams{Role: ui.RoleTypeMenuItem, Name: "Set wallpaper"}
	setWallpaper, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find the set wallpaper menu item: ", err)
	}
	defer setWallpaper.Release(ctx)

	// This button takes a bit before it is clickable.
	// Keep clicking it until the click is received and the menu closes.
	condition := func(ctx context.Context) (bool, error) {
		exists, err := ui.Exists(ctx, tconn, params)
		return !exists, err
	}
	opts := testing.PollOptions{Timeout: 10 * time.Second, Interval: 500 * time.Millisecond}
	if err := setWallpaper.LeftClickUntil(ctx, condition, &opts); err != nil {
		s.Fatal("Failed to open wallpaper picker: ", err)
	}

	// Click solid colors.
	params = ui.FindParams{Role: ui.RoleTypeStaticText, Name: "Solid colors"}
	solidColors, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find the solid colors button: ", err)
	}
	defer solidColors.Release(ctx)

	if err := solidColors.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click the solid colors button: ", err)
	}

	// Click deep purple wallpaper.
	params = ui.FindParams{Role: ui.RoleTypeListItem, Name: "Deep Purple"}
	deepPurple, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find the deep purple button: ", err)
	}
	defer deepPurple.Release(ctx)

	if err := deepPurple.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click the deep purple button: ", err)
	}

	// Ensure that "Deep Purple" text is displayed.
	// The UI displays the name of the currently set wallpaper.
	params = ui.FindParams{Role: ui.RoleTypeStaticText, Name: "Deep Purple"}
	deepPurpleText, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to set wallpaper, wallpaper name not changed: ", err)
	}
	defer deepPurpleText.Release(ctx)
}
