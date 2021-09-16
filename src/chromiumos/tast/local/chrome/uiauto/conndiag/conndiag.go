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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// App represents an instance of the Connectivity Diagnostics App.
type App struct {
	cr    *chrome.Chrome
	Tconn *chrome.TestConn
	ui    *uiauto.Context
}

const appURL = "chrome://connectivity-diagnostics/"

var windowFinder *nodewith.Finder = nodewith.Name("Connectivity Diagnostics").ClassName("BrowserFrame").Role(role.Window)

var titleFinder *nodewith.Finder = nodewith.Name("Connectivity Diagnostics").Role(role.InlineTextBox)

// Launch launches the Connectivity Diagnostics app and returns the instance of
// it. An error is returned if the app fails to launch.
func Launch(ctx context.Context, cr *chrome.Chrome) (*App, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect Test API")
	}

	if err := apps.Launch(ctx, tconn, apps.ConnectivityDiagnostics.ID); err != nil {
		return nil, errors.Wrap(err, "failed to launch connectivity diagnostics app")
	}

	ui := uiauto.New(tconn)

	// Get the Connectivity Diagnostics app window.
	if err := ui.WithTimeout(time.Minute).WaitUntilExists(windowFinder)(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to find app window")
	}

	// Ensure the title of the app is visible
	if err := ui.WithTimeout(30 * time.Second).WaitUntilExists(titleFinder)(ctx); err != nil {
		return nil, errors.Wrap(err, "unable to get app title")
	}

	app := App{cr: cr, Tconn: tconn, ui: ui}
	return &app, nil
}

// ChromeConn returns a Chrome connection to the Connectivity Diagnostics app if
// already launched.
func (a *App) ChromeConn(ctx context.Context) (*chrome.Conn, error) {
	conn, err := a.cr.NewConnForTarget(ctx, chrome.MatchTargetURL(appURL))
	if err != nil {
		return nil, err
	}
	return conn, nil
}
