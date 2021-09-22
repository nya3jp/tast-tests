// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cws provides a utility to install apps from the Chrome Web Store.
package cws

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

// InstallationTimeout defines the maximum time duration to install a cws app from the Chrome Web Store.
const InstallationTimeout = 5 * time.Minute

// App contains info about a Chrome Web Store app. All fields are required.
type App struct {
	Name         string // Name of the Chrome app.
	URL          string // URL to install the app from.
	InstalledTxt string // Button text after the app is installed.
	AddTxt       string // Button text when the app is available to be added.
	ConfirmTxt   string // Button text to confirm the installation.
}

// pollOpts is the polling interval and timeout to be used on the Chrome Web Store.
var pollOpts = &testing.PollOptions{Interval: time.Second, Timeout: InstallationTimeout}

// InstallApp installs the specified Chrome app from the Chrome Web Store.
func InstallApp(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, app App) error {
	cws, err := cr.NewConn(ctx, app.URL)
	if err != nil {
		return err
	}
	defer cws.Close()
	defer cws.CloseTarget(ctx)

	ui := uiauto.New(tconn)
	installed := nodewith.Name(app.InstalledTxt).Role(role.Button).First()
	add := nodewith.Name(app.AddTxt).Role(role.Button).First()
	confirm := nodewith.Name(app.ConfirmTxt).Role(role.Button)

	// Click the add button at most once to prevent triggering
	// weird UI behaviors in Chrome Web Store.
	addClicked := false
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Check if the app is installed.
		if err := ui.Exists(installed)(ctx); err == nil {
			return nil
		}

		if !addClicked {
			// If the app is not installed, install it now.
			// Click on the add button, if it exists.
			if err := ui.Exists(add)(ctx); err == nil {
				if err := ui.LeftClick(add)(ctx); err != nil {
					return testing.PollBreak(err)
				}
				addClicked = true
			}
		}

		// Click on the confirm button, if it exists.
		if err := ui.IfSuccessThen(ui.Exists(confirm), ui.LeftClick(confirm))(ctx); err != nil {
			return testing.PollBreak(err)
		}
		return errors.Errorf("%s still installing", app.Name)
	}, pollOpts); err != nil {
		return errors.Wrapf(err, "failed to install %s", app.Name)
	}
	return nil
}
