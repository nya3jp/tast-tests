// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package screenshot contains code to test the screenshot library.
package screenshot

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
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
	defer d.DieOnFailedDiffs()

	launcher := nodewith.ClassName("ash/HomeButton")
	searchBox := nodewith.ClassName("SearchBoxView").Role(role.Group)

	if err := d.DiffWithOptions("nomatches", nodewith.ClassName("MissingClassName"), screenshot.DiffTestOptions{Timeout: 500 * time.Millisecond})(ctx); err == nil {
		s.Fatal("Expected no matches")
	}
	if err := d.DiffWithOptions("multiplematches", nodewith.ClassName("FrameCaptionButton"), screenshot.DiffTestOptions{Timeout: 500 * time.Millisecond})(ctx); err == nil {
		s.Fatal("Expected multiple matches")
	}

	if err := d.Diff("repeat", launcher)(ctx); err != nil {
		s.Fatal("Failed to send diff: ", err)
	}
	if err := d.Diff("repeat", launcher)(ctx); err == nil {
		s.Fatal("Expected sending the same diff twice to fail")
	}

	testing.Sleep(ctx, 5*time.Second)

	screenshot.DiffPerConfigOrDie(ctx, s, []screenshot.Config{
		{Region: "us"},
		{Region: "au"},
		{Region: "jp"},
	}, func(d screenshot.Differ, cr *chrome.Chrome) {
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to create tconn: ", err)
		}
		ui := uiauto.New(tconn)
		if err := uiauto.Combine("Take screenshots",
			d.Diff("launcher", launcher),
			ui.LeftClick(launcher),
			ui.WaitForLocation(searchBox),
			d.Diff("searchbox", searchBox),
		)(ctx); err != nil {
			s.Fatal("Failed to screenshot searchbox: ", err)
		}
	})
}
