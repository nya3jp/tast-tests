// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RemoveAppsFromFolder,
		Desc:         "Test foldering actions in the launcher",
		Contacts:     []string{"mmourgos@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "chromeLoggedInWith100FakeApps",
	})
}

// RemoveAppsFromFolder tests that items can be removed from a folder.
func RemoveAppsFromFolder(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup(ctx)

	// Open the Launcher and go to Apps list page.
	ac := uiauto.New(tconn)
	if err := ac.Retry(2, launcher.OpenExpandedView(tconn))(ctx); err != nil {
		s.Fatal("Failed to open Expanded Application list view: ", err)
	}

	if err := ac.Retry(3, launcher.CreateFolder(ctx, tconn, ac))(ctx); err != nil {
		s.Fatal("Failed to create colder app: ", err)
	}

	// Add 5 app items to the folder.
	for i := 0; i < 5; i++ {
		launcher.DragIconToIcon(ctx, tconn, ac)(ctx)
	}

	// TODO: Check that the folder has 6 items.

	// Remove 3 items from the folder.
	for i := 0; i < 3; i++ {
		if err := launcher.RemoveIconFromFolder(tconn)(ctx); err != nil {
			s.Fatal("Failed to remove icon from folder: ", err)
		}
	}

	// TODO: Check that the folder has 3 items.

	// Remove the remaining 3 items from the folder.
	for i := 0; i < 3; i++ {
		if err := launcher.RemoveIconFromFolder(tconn)(ctx); err != nil {
			s.Fatal("Failed to remove icon from folder: ", err)
		}
	}

	// TODO: Check that there is no longer a folder.

	s.Log("Done adding items to max folder limit: ")
}
