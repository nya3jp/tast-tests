// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package screenshot contains code to test the screenshot library.
package screenshot

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DiffLib,
		Desc:         "Test to confirm that the screen diffing library works as intended",
		Contacts:     []string{"msta@google.com", "chrome-engprod@google.com"},
		SoftwareDeps: []string{"chrome"},
	})
}

func DiffLib(ctx context.Context, s *testing.State) {
	d, _, err := screenshot.NewDiffer(ctx, s)
	if err != nil {
		s.Fatal("Failed to initialize differ: ", err)
	}

	launcher := nodewith.ClassName("ash/HomeButton")
	searchBox := nodewith.ClassName("SearchBoxView").Role(role.Group)

	if err := d.Diff("nomatches", nodewith.ClassName("MissingClassName")); err == nil {
		s.Fatal("Expected no matches")
	}
	if err := d.Diff("multiplematches", nodewith.ClassName("FrameCaptionButton")); err == nil {
		s.Fatal("Expected multiple matches")
	}

	if err := d.Diff("repeat", launcher); err != nil {
		s.Fatal("Failed to send diff: ", err)
	}
	if err := d.Diff("repeat", launcher); err == nil {
		s.Fatal("Expected sending the same diff twice to fail")
	}

	screenshot.DiffPerConfigOrDie(ctx, s, []screenshot.Config{
		{Region: "us"},
		{Region: "au"},
		{Region: "jp"},
	}, func(d screenshot.Differ, cr *chrome.Chrome) {
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to create tconn: ", err)
		}
		ui2 := uiauto.New(tconn).WithTimeout(time.Second * 3)
		if err := uiauto.Run(ctx,
			d.DiffAction("launcher", launcher),
			ui2.LeftClick(launcher),
			ui2.WaitUntilExists(searchBox),
			func(ctx context.Context) error { return ui.WaitForLocationChangeCompleted(ctx, tconn) },
			d.DiffAction("searchbox", searchBox),
		); err != nil {
			s.Fatal("Failed to screenshot searchbox: ", err)
		}
	})
}
