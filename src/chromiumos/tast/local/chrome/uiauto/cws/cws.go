// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cws provides a utility to install apps from the Chrome Web Store.
package cws

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

// InstallationTimeout defines the maximum time duration to install an app from the Chrome Web Store.
const InstallationTimeout = 5 * time.Minute

// App contains info about a Chrome Web Store app. All fields are required.
type App struct {
	Name string // Name of the Chrome app.
	URL  string // URL to install the app from.
}

// pollOpts is the polling interval and timeout to be used on the Chrome Web Store.
var pollOpts = &testing.PollOptions{Interval: time.Second, Timeout: InstallationTimeout}

// InstallApp installs the specified Chrome app from the Chrome Web Store. This works for both ash-chrome and lacros-chrome browsers.
func InstallApp(ctx context.Context, br *browser.Browser, tconn *chrome.TestConn, app App) error {
	cws, err := br.NewConn(ctx, app.URL)
	if err != nil {
		return err
	}
	defer cws.Close()
	defer cws.CloseTarget(ctx)

	var (
		installed = nodewith.Role(role.Button).NameRegex(regexp.MustCompile(`(Remove from Chrome|Launch app)`)).First()
		add       = nodewith.Role(role.Button).Name(`Add to Chrome`).First()
		confirm   = nodewith.Role(role.Button).NameRegex(regexp.MustCompile(`Add (app|extension)`))

		ui = uiauto.New(tconn)
	)

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
