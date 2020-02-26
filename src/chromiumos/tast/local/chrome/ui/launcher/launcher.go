// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package launcher is used for controlling the launcher directly through the UI.
package launcher

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
)

// SearchAndLaunch searches a query in the launcher and executes it.
func SearchAndLaunch(ctx context.Context, tconn *chrome.TestConn, query string) error {
	const defaultTimeout = 15 * time.Second
	root, err := ui.Root(ctx, tconn)
	if err != nil {
		return err
	}
	defer root.Release(ctx)

	// Open the launcher.
	params := ui.FindParams{
		Name: "Launcher",
		Role: ui.RoleTypeButton,
	}
	launcherButton, err := root.DescendantWithTimeout(ctx, params, defaultTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find launcher button")
	}
	defer launcherButton.Release(ctx)
	if err := launcherButton.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click launcher button")
	}

	// Click the search box.
	searchBox, err := root.DescendantWithTimeout(ctx, ui.FindParams{ClassName: "SearchBoxView"}, defaultTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to wait for launcher searchbox")
	}
	defer searchBox.Release(ctx)
	if err := searchBox.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click launcher searchbox")
	}

	// Set up keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard")
	}
	defer kb.Close()

	// Search for the app.
	if err := kb.Type(ctx, query); err != nil {
		return errors.Wrapf(err, "failed to type %q", query)
	}
	if err := kb.Accel(ctx, "Enter"); err != nil {
		return errors.Wrap(err, "failed to type Enter key")
	}
	return nil
}
