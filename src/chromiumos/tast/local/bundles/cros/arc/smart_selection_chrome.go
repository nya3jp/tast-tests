// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SmartSelectionChrome,
		Desc:         "Test ARC's smart selections show up in Chrome's right click menu",
		Contacts:     []string{"chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Vars:         []string{"arc.SmartSelectionChrome.username", "arc.SmartSelectionChrome.password"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func SmartSelectionChrome(ctx context.Context, s *testing.State) {
	username := s.RequiredVar("arc.SmartSelectionChrome.username")
	password := s.RequiredVar("arc.SmartSelectionChrome.password")

	cr, err := chrome.New(ctx, chrome.GAIALogin(), chrome.Auth(username, password, "gaia-id"), chrome.ARCSupported())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC by user policy: ", err)
	}
	defer a.Close(ctx)
	if err := a.WaitIntentHelper(ctx); err != nil {
		s.Fatal("Failed to wait for ARC Intent Helper: ", err)
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Open page with an address on it.
	if _, err := cr.NewConn(ctx, "https://google.com/search?q=1600+amphitheatre+parkway"); err != nil {
		s.Fatal("Failed to create new Chrome connection: ", err)
	}

	// Wait for the address to appear.
	node, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeStaticText, Name: "1600 amphitheatre parkway"}, 30*time.Second)
	if err != nil {
		s.Fatal("Failed to wait for address to load: ", err)
	}
	defer node.Release(ctx)

	// Select the address.
	watcher, err := ui.NewRootWatcher(ctx, tconn, ui.EventTypeTextSelectionChanged)
	if err != nil {
		s.Fatal("Failed to create selection watcher: ", err)
	}
	defer watcher.Release(ctx)
	if err := ui.Select(ctx, node, 0, node, 25); err != nil {
		s.Fatal("Failed to select address: ", err)
	}
	if _, err := watcher.WaitForEvent(ctx, 20*time.Second); err != nil {
		s.Fatal("Failed to wait for the address to be selected: ", err)
	}

	// Right click the selected address.
	if err := node.RightClick(ctx); err != nil {
		s.Fatal("Failed to right click address: ", err)
	}

	// Ensure the smart selection map option is available.
	if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{Role: ui.RoleTypeMenuItem, Name: "Map"}, 30*time.Second); err != nil {
		s.Fatal("Failed to show map option: ", err)
	}
}
