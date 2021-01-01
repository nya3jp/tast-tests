// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package connectivitydiagnosticsapp provides support for controlling and
// interacting with the Chrome OS Connectivity Diagnostics app.
package connectivitydiagnosticsapp

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

const uiTimeout = 15 * time.Second

var defaultStablePollOpts = testing.PollOptions{Interval: 100 * time.Millisecond, Timeout: uiTimeout}

var appRootParams = ui.FindParams{
	Name: apps.ConnectivityDiagnostics.Name,
	Role: ui.RoleTypeWindow,
}

var rerunRoutinesButtonParams = ui.FindParams{
	Name: "Rerun Routines",
	Role: ui.RoleTypeButton,
}

// ConnectivityDiagnosticsApp represents an instance of the Connectivity
// Diagnostics App.
type ConnectivityDiagnosticsApp {
	tconn				*chrome.TestConn
	Root				*ui.Node
	stablePollOptions	*testing.PollOptions
}

// waitForRerunRoutinesButtonEnabled waits until the "Rerun Routines" button is enabled.
func (c *ConnectivityDiagnosticsApp) waitForRerunRoutinesButtonEnabled(ctx context.Context) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		rerunRoutinesButton, err := c.Root.DescendantWithTimeout(ctx, rerunRoutinesButtonParams, uiTimeout)
		if err != nil {
			return errors.Wrap(err, "failed to find Rerun Routines button")
		}
		defer rerunRoutinesButton.Release(ctx)

		if rerunRoutinesButton.Restriction == ui.RestrictionDisabled {
			return errors.New("Rerun Routines button is disabled")
		}

		return nil
	}, c.stablePollOpts); err != nil {
		return errors.Wrap(err, "failed to wait for Rerun Routines button")
	}

	return nil
}

// Launch launches the Connectivity Diagnostics app and returns it. An error is
// returned if the app fails to launch.
func Launch(ctx context.Context, tconn *chrome.TestConn) (*ConnectivityDiagnosticsApp, error) {
	// Launch the Connectivity Diagnostics app.
	if err := apps.Launch(ctx, tconn, apps.ConnectivityDiagnostics.ID); err != nil {
		return nil, err
	}

	// Get the Connectivity Diagnostics app root node.
	appRoot, err := ui.FindWithTimeout(ctx, tconn, appRootParams, time.Minute)
	if err != nil {
		return nil, err
	}

	app := ConnectivityDiagnosticsApp{tconn: tconn, Root: appRoot, stablePollOpts: &defaultStablePollOpts}

	// Wait until the "Rerun Routines" button is enabled to verify the app is loaded.
	if err := app.waitForRerunRoutinesButtonEnabled(ctx); err != nil {
		return nil, err
	}

	return &app, nil
}