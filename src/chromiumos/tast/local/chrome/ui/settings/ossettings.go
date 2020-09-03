// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package settings supports controlling the Settings App on Chrome OS.
package settings

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

const uiTimeout = 15 * time.Second

// Sidebar items / subpage names.
var (
	AboutChromeOS = ui.FindParams{
		Name: "About Chrome OS",
		Role: ui.RoleTypeLink,
	}
)

// Launch launches the Settings app.
// An error is returned if the app fails to launch.
func Launch(ctx context.Context, tconn *chrome.TestConn) error {
	app := apps.Settings
	if err := apps.Launch(ctx, tconn, app.ID); err != nil {
		return errors.Wrapf(err, "failed to launch %s", app.Name)
	}

	testing.ContextLog(ctx, "Wait for settings app shown in shelf")
	if err := ash.WaitForApp(ctx, tconn, app.ID); err != nil {
		return errors.Wrapf(err, "%s did not appear in shelf after launch", app.Name)
	}
	return nil
}

// LaunchAtPage launches the Settings app at a particular page.
// An error is returned if the app fails to launch.
func LaunchAtPage(ctx context.Context, tconn *chrome.TestConn, subpage ui.FindParams) error {
	// Launch Settings App.
	err := Launch(ctx, tconn)
	if err != nil {
		return err
	}

	// Find and click About Chrome OS.
	if err := ui.FindAndClick(ctx, tconn, subpage, uiTimeout); err != nil {
		return errors.Wrapf(err, "failed to find subpage with %v", subpage)
	}
	return nil
}
