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
		Vars:         screenshot.ScreenDiffVars,
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

	noRetries := screenshot.Timeout(500 * time.Millisecond)
	if err := expectError(
		d.Diff(ctx, "nomatches", nodewith.ClassName("MissingClassName"), noRetries)(ctx),
		"failed to find node"); err != nil {
		return errors.Wrap(err, "diffing with no matching elements succeeded")
	}
	if err := expectError(
		d.Diff(ctx, "nomatchesinwindow", nodewith.ClassName("UnifiedSystemTray"), noRetries)(ctx),
		"failed to find node"); err != nil {
		return errors.Wrap(err, "diffing with the matching element outside of the window succeeded")
	}
	if err := expectError(
		d.Diff(ctx, "multiplematches", nodewith.Name("My Files"), noRetries)(ctx),
		"failed to find node"); err != nil {
		return errors.Wrap(err, "diffing with multiple matching elements succeeded")
	}

	ui := uiauto.New(d.Tconn())
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
	ejectButton := nodewith.Name("Eject device").Role(role.Button).First()
	if err := uiauto.Combine("take screenshots of files app",
		// Device ejection is reset upon chrome start. The next test will still have the device.
		ui.IfSuccessThen(ui.Exists(ejectButton),
			uiauto.Combine("Eject device", ui.LeftClick(ejectButton), ui.WaitUntilGone(ejectButton))),
		d.Diff(ctx, "minMaxClose",
			nodewith.ClassName("FrameCaptionButtonContainerView")),
		d.Diff(ctx, "searchButton", nodewith.Name("Search").Role(role.Button)),
		d.Diff(ctx, "recentText", nodewith.Name("Recent").Role(role.StaticText)),
		d.Diff(ctx, "recentItem", nodewith.Name("Recent").Role(role.TreeItem)),
		d.Diff(ctx, "tree", nodewith.Role(role.Tree)),
		ui.WaitUntilGone(nodewith.Role(role.ProgressIndicator)),
		d.Diff(ctx, "welcomeMessage", nodewith.ClassName("holding-space-welcome")),
		d.Diff(ctx, "tableHeader", nodewith.ClassName("table-header")),
		d.Diff(ctx, "tableRow", nodewith.ClassName("table-row directory")),
		d.DiffWindow(ctx, "filesApp"))(ctx); err != nil {
		return err
	}

	if err := expectError(
		d.Diff(ctx, "filesApp", nodewith.First())(ctx),
		"screenshot has already been taken"); err != nil {
		return errors.Wrap(err, "sending the same diff twice succeeded: ")
	}
	return nil
}

func ScreenDiff(ctx context.Context, s *testing.State) {
	screenDiffConfig := screenshot.Config{
		DefaultOptions: screenshot.Options{
			WindowWidthDP:  1000,
			WindowHeightDP: 632,
			RemoveElements: []*nodewith.Finder{nodewith.ClassName("date")}},
		NameSuffix: "V2"}

	// Normally the next line would be "defer d.DieOnFailedDiffs()"
	// However, in our case, we want to run both this and DiffPerConfig.
	d, err := screenshot.NewDiffer(ctx, s, screenDiffConfig)
	if err != nil {
		s.Fatal("Failed to initialize differ: ", err)
	}

	if err := expectError(
		d.Diff(ctx, "nowindowopen", nodewith.ClassName("FrameCaptionButton"), screenshot.Timeout(500*time.Millisecond))(ctx),
		"unable to find focused window"); err != nil {
		s.Fatal("Diffing with no window open succeeded: ", err)
	}

	if err := takeScreenshots(ctx, d); err != nil {
		s.Fatal("Failed to screenshot with single config: ", err)
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
