// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SmartSelectionChrome,
		Desc:         "Test ARC's smart selections show up in Chrome's right click menu",
		Contacts:     []string{"chromeos-sw-engprod@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		VarDeps:      []string{"arc.SmartSelectionChrome.username", "arc.SmartSelectionChrome.password"},
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

	cr, err := chrome.New(ctx, chrome.GAIALogin(chrome.Creds{User: username, Pass: password}), chrome.ARCSupported())
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
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	// Open page with an address on it.
	if _, err := cr.NewConn(ctx, "https://google.com/search?q=1600+amphitheatre+parkway"); err != nil {
		s.Fatal("Failed to create new Chrome connection: ", err)
	}

	// Wait for the address to appear.
	address := nodewith.Name("1600 amphitheatre parkway").Role(role.StaticText)
	if err := ui.WaitUntilExists(address)(ctx); err != nil {
		s.Fatal("Failed to wait for address to load: ", err)
	}

	// Select the address and setup watcher to wait for text selection event
	if err := ui.WaitForEvent(nodewith.Root(),
		event.TextSelectionChanged,
		ui.Select(address, 0, address, 25))(ctx); err != nil {
		s.Fatal("Failed to select address: ", err)
	}

	// Right click the selected address and ensure the smart selection map option is available.
	mapOption := nodewith.Name("Map").Role(role.MenuItem)
	if err := uiauto.Combine("Show context menu",
		ui.RightClick(address),
		ui.WaitUntilExists(mapOption))(ctx); err != nil {
		s.Fatal("Failed to show map option: ", err)
	}
}
