// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uig"
)

const terminalAppID = "fhicihalidkgcimdmhpohldehjmcabcf"

const uiTimeout = 30 * time.Second

// Terminal represents the crostini terminal app.
type Terminal struct {
	tconn *chrome.TestConn
}

// LaunchTerminal launches the crostini terminal app.
func LaunchTerminal(ctx context.Context, tconn *chrome.TestConn) (*Terminal, error) {
	err := apps.Launch(ctx, tconn, terminalAppID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch Crostini Terminal")
	}
	terminal := &Terminal{tconn: tconn}
	return terminal, terminal.waitForPrompt(ctx)
}

func (t *Terminal) waitForPrompt(ctx context.Context) error {
	waitForPrompt := uig.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeRootWebArea, Name: "testuser@penguin: ~"}, uiTimeout).
		FindWithTimeout(ui.FindParams{Role: ui.RoleTypeStaticText, Name: "$ "}, 90*time.Second).
		WithNamef("Terminal.waitForPrompt()")
	return uig.Do(ctx, t.tconn, waitForPrompt)
}

// ShutdownCrostini shuts down crostini by right clicking on the terminal app shelf icon.
func (t *Terminal) ShutdownCrostini(ctx context.Context) error {
	revert, err := ash.EnsureTabletModeEnabled(ctx, t.tconn, false)
	if err != nil {
		return errors.Wrap(err, "Unable to switch out of tablet mode")
	}
	defer revert(ctx)

	shutdown := uig.Steps(
		uig.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeButton, Name: "Terminal"}, uiTimeout).RightClick(),
		uig.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeMenuItem, Name: "Shut down Linux (Beta)"}, uiTimeout).LeftClick(),
	).WithNamef("Terminal.ShutdownCrostini()")
	return uig.Do(ctx, t.tconn, shutdown)
}
