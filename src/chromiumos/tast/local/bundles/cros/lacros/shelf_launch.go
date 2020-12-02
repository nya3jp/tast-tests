// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShelfLaunch,
		Desc:         "Tests basic lacros startup",
		Contacts:     []string{"lacros-team@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Params: []testing.Param{
			{
				Pre:       launcher.StartedByDataUI(),
				ExtraData: []string{launcher.DataArtifact},
			},
			{
				Name:              "omaha",
				Pre:               launcher.StartedByOmaha(),
				ExtraHardwareDeps: hwdep.D(hwdep.Model("enguarde", "samus", "sparky")),
			}},
	})
}

func ShelfLaunch(ctx context.Context, s *testing.State) {
	tconn, err := s.PreValue().(launcher.PreData).Chrome.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// When launched by Omaha we need to wait several seconds after signing in for lacros to be launchable.
	// It is ready when the image loader path is created with the chrome executable.
	if s.PreValue().(launcher.PreData).Mode == launcher.Omaha {
		testing.ContextLog(ctx, "Waiting for Lacros to initialize")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			var matches []string
			var err error
			if matches, err = filepath.Glob("/run/imageloader/lacros-fishfood/*/chrome"); err != nil {
				return errors.Wrap(err, "binaryPath does not exist yet")
			}
			if len(matches) == 0 {
				return errors.New("BinaryPath does not exist yet")
			}
			return nil
		}, &testing.PollOptions{Timeout: 2 * time.Minute, Interval: 5 * time.Second}); err != nil {
			s.Fatal("Failed to find lacros binary: ", err)
		}
	}

	s.Log("Checking that Lacros is included in installed apps")
	appItems, err := ash.ChromeApps(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get installed apps: ", err)
	}
	found := false
	for _, appItem := range appItems {
		if appItem.AppID == apps.Lacros.ID && appItem.Name == apps.Lacros.Name && appItem.Type == ash.Lacros {
			found = true
			break
		}
	}
	if !found {
		s.Fatal("Lacros was not included in the list of installed applications: ", err)
	}

	s.Log("Check that Lacros is a pinned app in the shelf")
	shelfItems, err := ash.ShelfItems(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get shelf items: ", err)
	}
	found = false
	for _, shelfItem := range shelfItems {
		if shelfItem.AppID == apps.Lacros.ID && shelfItem.Title == apps.Lacros.Name && shelfItem.Type == ash.ShelfItemTypePinnedApp {
			found = true
			break
		}
	}
	if !found {
		s.Fatal("Lacros was not found in the list of shelf items: ", err)
	}

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get all open windows: ", err)
	}
	for _, w := range ws {
		if err := w.CloseWindow(ctx, tconn); err != nil {
			s.Logf("Warning: Failed to close window (%+v): %v", w, err)
		}
	}

	if err = ash.LaunchAppFromShelf(ctx, tconn, apps.Lacros.Name, apps.Lacros.ID); err != nil {
		s.Fatal("Failed to launch Lacros: ", err)
	}

	s.Log("Checking that Lacros window is visible")
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.IsVisible && strings.HasPrefix(w.Title, "Welcome to Chrome") && strings.HasPrefix(w.Name, "ExoShellSurface")
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed waiting for Lacros window to be visible: ", err)
	}
}
