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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
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

func takeScreenshots(ctx context.Context, d screenshot.Differ) error {
	_, err := filesapp.Launch(ctx, d.Tconn())
	if err != nil {
		return err
	}
	ui := uiauto.New(d.Tconn())
	filesAppOptions := screenshot.DiffTestOptions{RemoveElements: []*nodewith.Finder{nodewith.ClassName("date")}}
	// We take various screenshots to test various different things:
	// * System UI elements,
	// * Icons with no text
	// * Standalone text
	// * Text with icons
	// * Elements that may or may not have a fixed size
	// * Elements with dynamic content inside them
	// This should not be done by other users of the screen diff library.
	// We only do this to attempt to determine how screenshots of different types
	// of elements are affected by device-specific configuration.
	return uiauto.Combine("open files app and take screenshots",
		d.DiffWithOptions("minMaxClose",
			nodewith.ClassName("FrameCaptionButtonContainerView").Ancestor(filesapp.WindowFinder),
			screenshot.DiffTestOptions{IgnoredBorderThickness: 1}),
		d.Diff("searchButton", nodewith.Name("Search").Role(role.Button).Ancestor(filesapp.WindowFinder)),
		d.Diff("recentText", nodewith.Name("Recent").Role(role.StaticText).Ancestor(filesapp.WindowFinder)),
		d.Diff("recentItem", nodewith.Name("Recent").Role(role.TreeItem).Ancestor(filesapp.WindowFinder)),
		d.Diff("tree", nodewith.Role(role.Tree).Ancestor(filesapp.WindowFinder)),
		ui.WaitUntilGone(nodewith.Role(role.ProgressIndicator).Ancestor(filesapp.WindowFinder)),
		d.Diff("welcomeMessage", nodewith.ClassName("holding-space-welcome").Ancestor(filesapp.WindowFinder)),
		d.Diff("tableHeader", nodewith.ClassName("table-header").Ancestor(filesapp.WindowFinder)),
		d.DiffWithOptions("tableRow", nodewith.ClassName("table-row directory").Ancestor(filesapp.WindowFinder), filesAppOptions),
		d.DiffWithOptions("filesApp", filesapp.WindowFinder, filesAppOptions))(ctx)
}

func ScreenDiff(ctx context.Context, s *testing.State) {
	screenDiffConfig := screenshot.Config{OutputUITrees: true}
	// Normally the next line would be "defer d.DieOnFailedDiffs()"
	// However, in our case, we want to run both this and DiffPerConfig.
	d, err := screenshot.NewDifferFromConfig(ctx, s, screenDiffConfig)
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

	if err := takeScreenshots(ctx, d); err != nil {
		s.Fatal("Failed to screenshot with single config: ", err)
	}

	if err := expectError(
		takeScreenshots(ctx, d),
		"screenshot has already been taken"); err != nil {
		s.Fatal("sending the same diff twice succeeded: ", err)
	}

	failedSingle := d.GetFailedDiffs()

	// Unfortunately, it's not possible to test that images fail on gold, because
	// gold would then comment on everyone's CLs saying that they failed this test.
	// TODO(crbug.com/1173812): Once ThoroughConfigs has more than one element, switch to ThoroughConfigs()
	failedMulti := screenshot.DiffPerConfig(ctx, s, screenshot.WithBase(screenDiffConfig, []screenshot.Config{}), func(d screenshot.Differ) {
		if err := takeScreenshots(ctx, d); err != nil {
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
