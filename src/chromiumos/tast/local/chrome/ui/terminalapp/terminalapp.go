// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package terminalapp supports actions on Terminal on Chrome OS.
package terminalapp

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/input"
)

const uiTimeout = 15 * time.Second

var rootFindParams ui.FindParams = ui.FindParams{
	Name:      "", // The window name is username@penguin: ~, it should be set dynamically.
	Role:      ui.RoleTypeWindow,
	ClassName: "BrowserFrame",
}

// TerminalApp represents an instance of the Terminal App.
type TerminalApp struct {
	tconn    *chrome.TestConn
	Root     *ui.Node
	userName string
}

// Launch launches the Terminal App and returns it.
// An error is returned if the app fails to launch.
func Launch(ctx context.Context, tconn *chrome.TestConn, userName string) (*TerminalApp, error) {
	rootFindParams.Name = fmt.Sprintf("%s@penguin: ~", userName)

	// Get Terminal App root node.
	app, err := ui.FindWithTimeout(ctx, tconn, rootFindParams, time.Minute)
	if err != nil {
		// Launch the Terminal App.
		if err := apps.Launch(ctx, tconn, apps.Terminal.ID); err != nil {
			return nil, err
		}
		app, err = ui.FindWithTimeout(ctx, tconn, rootFindParams, time.Minute)
		if err != nil {
			return nil, err
		}
	}

	// Check Terminal is on shelf.
	if err := ash.WaitForApp(ctx, tconn, apps.Terminal.ID); err != nil {
		return nil, errors.Wrap(err, "failed to find Terminal icon on shelf")
	}

	// It takes a few seconds to start the Terminal, it is ready when the prefix of the command line is displayed.
	params := ui.FindParams{
		Name: rootFindParams.Name,
		Role: "rootWebArea",
	}
	if err := app.WaitUntilDescendantExists(ctx, params, uiTimeout); err != nil {
		return nil, err
	}

	return &TerminalApp{tconn: tconn, Root: app, userName: userName}, nil
}

// FocusMouseOnTerminalWindow gets focus on the Terminal window.
func (ta *TerminalApp) FocusMouseOnTerminalWindow(ctx context.Context) error {
	// Find the static text "username@penguin".
	params := ui.FindParams{
		Name: fmt.Sprintf("%s@penguin", ta.userName),
		Role: ui.RoleTypeStaticText,
	}
	textArea, err := ta.Root.DescendantWithTimeout(ctx, params, 15*time.Second)
	if err != nil {
		return errors.Wrapf(err, "failed to find input area %s", ta.userName)
	}
	defer textArea.Release(ctx)

	// Move mouse to the static text to focus on Terminal window.
	if err := mouse.Move(ctx, ta.tconn, textArea.Location.CenterPoint(), 15*time.Second); err != nil {
		return errors.Wrap(err, "failed to focus in input area")
	}
	return nil
}

// RunCommand runs command in Terminal windows.
func (ta *TerminalApp) RunCommand(ctx context.Context, keyboard *input.KeyboardEventWriter, cmd string) error {
	if err := ta.FocusMouseOnTerminalWindow(ctx); err != nil {
		return errors.Wrap(err, "failed to focus on Terminal window")
	}

	// Type command.
	if err := keyboard.Type(ctx, cmd); err != nil {
		return errors.Wrapf(err, "failed to type %s in Terminal", cmd)
	}

	// Press Enter.
	if err := keyboard.Accel(ctx, "Enter"); err != nil {
		return errors.Wrap(err, "failed to press Enter in Terminal")
	}
	return nil
}

// Close closes the Terminal App.
func (ta *TerminalApp) Close(ctx context.Context, keyboard *input.KeyboardEventWriter) error {
	if err := ta.RunCommand(ctx, keyboard, "exit"); err != nil {
		return errors.Wrap(err, "failed to exit Terminal window")
	}
	ta.Root.Release(ctx)

	// Wait for window to close.
	return ui.WaitUntilGone(ctx, ta.tconn, rootFindParams, time.Minute)
}
