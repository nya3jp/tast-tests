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
	"chromiumos/tast/local/chrome/uiauto/ossettings"
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
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{screenshot.GoldServiceAccountKeyVar},
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

func screenshotSettingsSearchBox(ctx context.Context, cr *chrome.Chrome, d screenshot.Differ) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}
	if _, err := ossettings.Launch(ctx, tconn); err != nil {
		return err
	}
	ui := uiauto.New(tconn)
	return uiauto.Combine("open about and take screenshot",
		// Ensure that the focus is no longer on the search box. Otherwise, the flashing mouse cursor could be a problem.
		ui.LeftClick(ossettings.AboutChromeOS),
		// By the time the new page has loaded, we can assume that the searchbox has lost focus.
		ui.WaitForLocation(ossettings.AboutChromeOS.Role(role.Heading)),
		d.Diff("settingssearchbox", ossettings.SearchBoxFinder))(ctx)
}

func ScreenDiff(ctx context.Context, s *testing.State) {
	// The defer in the SingleConfigDiffer needs to happen before the multiconfigdiffer starts.
	d, cr, err := screenshot.NewDiffer(ctx, s)
	if err != nil {
		s.Fatal("Failed to initialize differ: ", err)
	}

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

	if err := screenshotSettingsSearchBox(ctx, cr, d); err != nil {
		s.Fatal("Failed to screenshot with single config: ", err)
	}

	if err := expectError(
		screenshotSettingsSearchBox(ctx, cr, d),
		"screenshot has already been taken"); err != nil {
		s.Fatal("sending the same diff twice succeeded: ", err)
	}

	failedSingle := d.GetFailedDiffs()

	// Unfortunately, it's not possible to test that images fail on gold, because
	// gold would then comment on everyone's CLs saying that they failed this test.
	failedMulti := screenshot.DiffPerConfig(ctx, s, []screenshot.Config{{Region: "de"}, {Region: "en"}}, func(d screenshot.Differ, cr *chrome.Chrome) {
		if err := screenshotSettingsSearchBox(ctx, cr, d); err != nil {
			s.Fatal("Failed to take screenshot with multiple configs: ", err)
		}
	})

	if failedSingle != nil && failedMulti != nil {
		s.Fatalf("Failed both single and multi-config diffs: single-config %s AND multi-config %s", failedSingle, failedMulti)
	} else if failedSingle != nil {
		s.Fatal("Failed single-config diffs: ", failedSingle)
	} else if failedMulti != nil {
		s.Fatal("Failed multi-config diffs: ", failedMulti)
	}
}
