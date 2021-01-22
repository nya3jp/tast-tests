// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package sharesheet supports controlling the Sharesheet on Chrome OS.
package sharesheet

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

// ClickApp clicks the requested app on the sharesheet.
// The app must be available in the first 8 apps.
func ClickApp(ctx context.Context, tconn *chrome.TestConn, appShareLabel string) error {
	pollOpts := testing.PollOptions{Interval: 2 * time.Second, Timeout: 15 * time.Second}

	quickEditButton, err := waitForAppOnStableSharesheet(ctx, tconn, appShareLabel, &pollOpts)
	if err != nil {
		return errors.Wrap(err, "failed waiting for Sharesheet window to stabilize")
	}
	defer quickEditButton.Release(ctx)

	return quickEditButton.StableLeftClick(ctx, &pollOpts)
}

// waitForAppOnStableSharesheet waits for the Sharesheet to stabilize and returns the ARC apps node.
func waitForAppOnStableSharesheet(ctx context.Context, tconn *chrome.TestConn, appName string, pollOpts *testing.PollOptions) (*ui.Node, error) {
	// Get the Sharesheet View popup window.
	params := ui.FindParams{
		ClassName: "View",
		Name:      "Share",
	}
	sharesheetWindow, err := ui.FindWithTimeout(ctx, tconn, params, pollOpts.Timeout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find Sharesheet window")
	}
	defer sharesheetWindow.Release(ctx)

	// Setup a watcher to wait for the apps list in Sharesheet to stabilize.
	ew, err := ui.NewWatcher(ctx, sharesheetWindow, ui.EventTypeActiveDescendantChanged)
	if err != nil {
		return nil, errors.Wrap(err, "failed getting a watcher for the Sharesheet views window")
	}
	defer ew.Release(ctx)

	// Check the Sharesheet window for any Activedescendantchanged events occurring in a 2 second interval.
	// If any events are found continue polling until 10s is reached.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return ew.EnsureNoEvents(ctx, pollOpts.Interval)
	}, pollOpts); err != nil {
		return nil, errors.Wrapf(err, "failed waiting %v for Sharesheet window to stabilize", pollOpts.Timeout)
	}

	// Get the app button to click.
	params = ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: appName,
	}
	appButton, err := sharesheetWindow.DescendantWithTimeout(ctx, params, pollOpts.Timeout)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find app %q on Sharesheet window", appName)
	}

	return appButton, nil
}
