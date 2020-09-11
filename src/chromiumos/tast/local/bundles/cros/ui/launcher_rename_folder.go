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
	"chromiumos/tast/local/chrome/ui/launcher"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/testing"
)

// init adds the test LauncherRenameFolder.
func init() {
	testing.AddTest(&testing.Test{
		Func: LauncherRenameFolder,
		Desc: "Renaming Folder In Launcher",
		Contacts: []string{
			"seewaifu@chromium.org",
			"kyleshima@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

// LauncherRenameFolder tests if pinning application onto shelf behaves correctly.
func LauncherRenameFolder(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Open the Launcher and go to Apps list page.
	if err := launcher.OpenExpandedView(ctx, tconn); err != nil {
		s.Fatal("Failed to open Expanded Application list view: ", err)
	}

	// Create a Folder at launcher by dragging one app on top of another.
	params := ui.FindParams{ClassName: "ui/app_list/AppListItemView"}
	icons, err := ui.FindAll(ctx, tconn, params)
	if err != nil {
		s.Fatal("Failed to find all in expanded launcher: ", err)
	}
	defer icons.Release(ctx)
	if len(icons) < 2 {
		s.Fatal("Not enough icons in expanded launcher to perform test")
	}
	point0 := icons[0].Location.CenterPoint()
	point1 := icons[1].Location.CenterPoint()
	// Drag one app on top of another.
	if err := mouse.Drag(ctx, tconn, point0, point1, 10*time.Second); err != nil {
		s.Fatalf("Failed to Drag app %q: %v", icons[0].Name, err)
	}
	ui.WaitForLocationChangeCompleted(ctx, tconn)

	if err := launcher.RenameFolder(ctx, tconn, "Unnamed", "NewName"); err != nil {
		s.Fatal("Failed to rename folder to NewName: ", err)
	}

}
