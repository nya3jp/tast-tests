// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LauncherSearchApps,
		Desc: "Launches an app of various types through the launcher",
		Contacts: []string{
			"bhansknecht@chromium.org",
			"dhaddock@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:              "arc",
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.Booted(),
			Val:               "Play Store",
			Timeout:           5 * time.Minute,
		}, {
			Name:              "arc_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBooted(),
			Val:               "Play Store",
			Timeout:           5 * time.Minute,
		}, {
			Name:              "crostini",
			ExtraSoftwareDeps: []string{"vm_host"},
			Pre:               crostini.StartedByArtifact(),
			ExtraData:         []string{crostini.ImageArtifact},
			Val:               "Terminal",
			Timeout:           7 * time.Minute,
		}, {
			Name:    "native",
			Pre:     chrome.LoggedIn(),
			Val:     "Settings",
			Timeout: 2 * time.Minute,
		}},
	})
}

func LauncherSearchApps(ctx context.Context, s *testing.State) {
	const defaultTimeout = 15 * time.Second
	appName := s.Param().(string)

	var cr *chrome.Chrome
	switch v := s.PreValue().(type) {
	case arc.PreData:
		cr = v.Chrome
		a := v.ARC
		if err := a.WaitIntentHelper(ctx); err != nil {
			s.Fatal("Failed to wait for ARC Intent Helper: ", err)
		}
	case *chrome.Chrome:
		cr = v
	case crostini.PreData:
		cr = v.Chrome
	default:
		s.Fatal("Unknown precondition type")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	root, err := ui.Root(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to UI root: ", err)
	}
	defer root.Release(ctx)

	// Open the launcher.
	params := ui.FindParams{
		Name: "Launcher",
		Role: ui.RoleTypeButton,
	}
	launcherButton, err := root.DescendantWithTimeout(ctx, params, defaultTimeout)
	if err != nil {
		s.Fatal("Failed to find launcher button: ", err)
	}
	defer launcherButton.Release(ctx)
	if err := launcherButton.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click launcher button: ", err)
	}

	// Click the search box.
	searchBox, err := root.DescendantWithTimeout(ctx, ui.FindParams{ClassName: "SearchBoxView"}, defaultTimeout)
	if err != nil {
		s.Fatal("Failed to wait for launcher searchbox: ", err)
	}
	defer searchBox.Release(ctx)
	if err := searchBox.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click launcher searchbox: ", err)
	}

	// Setup keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	// Search for the app.
	if err := kb.Type(ctx, appName); err != nil {
		s.Fatalf("Failed to type %q: %v", appName, err)
	}
	if err := kb.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Failed to type enter key: ", err)
	}

	// Look at the shelf to make sure the app launched.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		items, err := ash.ShelfItems(ctx, tconn)
		if err != nil {
			return testing.PollBreak(err)
		}
		for _, item := range items {
			if item.Title == appName {
				return nil
			}
		}
		return errors.New("app icon not found in the shelf")
	}, nil); err != nil {
		s.Fatalf("Failed to launch %q: %v", appName, err)
	}
}
