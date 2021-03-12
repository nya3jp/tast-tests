// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package screenshot contains code to test the screenshot library.
package screenshot

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScreenDiff,
		Desc:         "Test to confirm that the screen diffing library works as intended",
		Contacts:     []string{"msta@google.com", "chrome-engprod@google.com"},
		SoftwareDeps: []string{"chrome"},
	})
}

// expectError returns an error if the error returned doesn't match the expectation.
func expectError(err error, expectation string) error {
	if err == nil {
		return errors.New("expected an error but didn't get it")
	}
	if !strings.Contains(err.Error(), expectation) {
		return errors.Wrapf(err, "expected an error containing the string %s, but got the error: ", expectation)
	}
	return nil
}

func ScreenDiff(ctx context.Context, s *testing.State) {
	launcher := nodewith.ClassName("ash/HomeButton")
	searchBox := nodewith.ClassName("SearchBoxView").Role(role.Group)

	// The defer in the SingleConfigDiffer needs to happen before the multiconfigdiffer starts.
	func() {
		d, _, err := screenshot.NewDiffer(ctx, s)
		if err != nil {
			s.Fatal("Failed to initialize differ: ", err)
		}
		defer d.DieOnFailedDiffs()

		if err := expectError(
			d.DiffWithOptions("nomatches", nodewith.ClassName("MissingClassName"), screenshot.DiffTestOptions{Timeout: 500 * time.Millisecond})(ctx),
			"failed to find node"); err != nil {
			s.Fatal("diffing with no matching elements succeeded: ", err)
		}
		if err := expectError(
			d.DiffWithOptions("multiplematches", nodewith.ClassName("FrameCaptionButton"), screenshot.DiffTestOptions{Timeout: 500 * time.Millisecond})(ctx),
			"failed to find node"); err != nil {
			s.Fatal("diffing with multiple matching elements succeeded: ", err)
		}

		if err := d.Diff("repeat", launcher)(ctx); err != nil {
			s.Fatal("Failed to send diff: ", err)
		}

		if err := expectError(
			d.Diff("repeat", launcher)(ctx),
			"screenshot has already been taken"); err != nil {
			s.Fatal("sending the same diff twice succeeded: ", err)
		}
	}()

	if err := screenshot.DiffPerConfig(ctx, s, []screenshot.Config{
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
	}); err != nil {
		s.Fatal("Taking screenshots of launcher and searchbox failed: ", err)
	}
}
