// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package screenshot contains code to test the screenshot library.
package screenshot

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScreenDiff,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test to confirm that the screen diffing library works as intended",
		Contacts:     []string{"msta@google.com", "chrome-engprod@google.com"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.Model("eve")),
		// Disabled due to <1% pass rate over 30 days. See b/241943743
		//Attr:         []string{"group:mainline", "informational"},
		Vars: screenshot.ScreenDiffVars,
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
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	screenDiffConfig := screenshot.Config{
		DefaultOptions: screenshot.Options{
			WindowWidthDP:  1000,
			WindowHeightDP: 632,
			RemoveElements: []*nodewith.Finder{nodewith.ClassName("date")}},
		NameSuffix: "V2"}

	d, err := screenshot.NewDiffer(ctx, s, screenDiffConfig)
	if err != nil {
		s.Fatal("Failed to initialize differ: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, d.Tconn())
	defer d.DieOnFailedDiffs()

	if err := expectError(
		d.Diff(ctx, "nowindowopen", nodewith.ClassName("FrameCaptionButton"), screenshot.Timeout(500*time.Millisecond))(ctx),
		"unable to find focused window"); err != nil {
		s.Fatal("Diffing with no window open succeeded: ", err)
	}

	_, err = filesapp.Launch(ctx, d.Tconn())
	if err != nil {
		s.Fatal("Failed to launch files app: ", err)
	}

	noRetries := screenshot.Timeout(500 * time.Millisecond)
	if err := expectError(
		d.Diff(ctx, "nomatches", nodewith.ClassName("MissingClassName"), noRetries)(ctx),
		"failed to find node"); err != nil {
		s.Fatal("Diffing with no matching elements succeeded: ", err)
	}
	if err := expectError(
		d.Diff(ctx, "nomatchesinwindow", nodewith.ClassName("UnifiedSystemTray"), noRetries)(ctx),
		"failed to find node"); err != nil {
		s.Fatal("Diffing with the matching element outside of the window succeeded: ", err)
	}
	if err := expectError(
		d.Diff(ctx, "multiplematches", nodewith.Name("My Files"), noRetries)(ctx),
		"failed to find node"); err != nil {
		s.Fatal("Diffing with multiple matching elements succeeded: ", err)
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
		uiauto.IfSuccessThen(ui.Exists(ejectButton),
			uiauto.Combine("Eject device", ui.LeftClick(ejectButton), ui.WaitUntilGone(ejectButton))),
		ui.WaitUntilGone(nodewith.Role(role.ProgressIndicator)),
		d.Diff(ctx, "recentText", nodewith.Name("Recent").Role(role.StaticText)),
		d.DiffWindow(ctx, "filesApp"),
		d.DiffWindow(ctx, "filesAppMaximized", screenshot.WindowState(ash.WindowStateMaximized)))(ctx); err != nil {
		s.Fatal("Failed to diff windows: ", err)
	}

	if err := expectError(
		d.Diff(ctx, "filesApp", nodewith.First())(ctx),
		"screenshot has already been taken"); err != nil {
		s.Fatal("Sending the same diff twice succeeded: ", err)
	}

	revert, err := ash.EnsureTabletModeEnabled(ctx, d.Tconn(), true)
	if err != nil {
		s.Fatal("Failed to enter tablet mode: ", err)
	}
	defer revert(cleanupCtx)

	if err := d.DiffWindow(ctx, "filesAppSplit", screenshot.WindowState(ash.WindowStateLeftSnapped), screenshot.Retries(4))(ctx); err != nil {
		s.Fatal("Failed to diff tablet window: ", err)
	}
}
