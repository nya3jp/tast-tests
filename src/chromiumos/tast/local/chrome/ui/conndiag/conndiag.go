// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package conndiag provides tools and library calls to create and manage an
// instance of the Connectivity Diagnostics App.
package conndiag

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

// App represents an instance of the Connectivity Diagnostics App.
type App struct {
	tconn *chrome.TestConn
	Root  *ui.Node
}

const appURL = "chrome://connectivity-diagnostics/"

var pollOpts = testing.PollOptions{
	Interval: 100 * time.Millisecond,
	Timeout:  5 * time.Second,
}

var appRootParams = ui.FindParams{
	Name: "Connectivity Diagnostics",
	Role: ui.RoleTypeWindow,
}

var appTitleParams = ui.FindParams{
	Name: "Connectivity Diagnostics",
	Role: ui.RoleTypeInlineTextBox,
}

// Launch launches the Connectivity Diagnostics app and returns the instance of
// it. An error is returned if the app fails to launch.
func Launch(ctx context.Context, tconn *chrome.TestConn) (*App, error) {
	if err := apps.Launch(ctx, tconn, apps.ConnectivityDiagnostics.ID); err != nil {
		return nil, errors.Wrap(err, "failed to launch connectivity diagnostics app")
	}

	// Get the Connectivity Diagnostics app root node.
	appRoot, err := ui.FindWithTimeout(ctx, tconn, appRootParams, time.Minute)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find app root")
	}

	app := App{tconn: tconn, Root: appRoot}

	if err := app.waitForTitle(ctx); err != nil {
		return nil, err
	}

	return &app, nil
}

// ChromeConn returns a Chrome connection to the Connectivity Diagnostics app.
func (a *App) ChromeConn(ctx context.Context, cr *chrome.Chrome) (*chrome.Conn, error) {
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(appURL))
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (a *App) waitForTitle(ctx context.Context) error {
	// Poll the root node for the title node.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := a.Root.DescendantWithTimeout(ctx, appTitleParams, time.Minute); err != nil {
			return errors.Wrap(err, "failed to find app title")
		}
		return nil
	}, &pollOpts); err != nil {
		return errors.Wrap(err, "timed out waiting for app title")
	}
	return nil
}
