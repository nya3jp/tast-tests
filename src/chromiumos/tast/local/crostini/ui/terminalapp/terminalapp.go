// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package terminalapp supports actions on Terminal on Chrome OS.
package terminalapp

import (
	"context"
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

// TerminalApp represents an instance of the Terminal App.
type TerminalApp struct {
	tconn *chrome.TestConn
	Root  *ui.Node
}

// Launch launches the Terminal App and returns it.
// An error is returned if the app fails to launch.
func Launch(ctx context.Context, tconn *chrome.TestConn, userName string) (*TerminalApp, error) {
	rootFindParams := ui.FindParams{
		Name:      userName + "@penguin: ~",
		Role:      ui.RoleTypeWindow,
		ClassName: "BrowserFrame",
	}

	// Launch the Terminal App.
	if err := apps.Launch(ctx, tconn, apps.Terminal.ID); err != nil {
		return nil, err
	}
	app, err := ui.FindWithTimeout(ctx, tconn, rootFindParams, time.Minute)
	if err != nil {
		return nil, err
	}

	// Check Terminal is on shelf.
	if err := ash.WaitForApp(ctx, tconn, apps.Terminal.ID); err != nil {
		app.Release(ctx)
		return nil, errors.Wrap(err, "failed to find Terminal icon on shelf")
	}

	// It takes a few seconds to start the Terminal, it is ready when the prefix of the command line is displayed.
	// The prefix is static text "username@penguin"
	params := ui.FindParams{
		Name: userName + "@penguin",
		Role: ui.RoleTypeStaticText,
	}
	if err := app.WaitUntilDescendantExists(ctx, params, uiTimeout); err != nil {
		app.Release(ctx)
		return nil, errors.Wrapf(err, "failed to find input area %s", userName)
	}

	return &TerminalApp{tconn: tconn, Root: app}, nil
}

// FocusMouseOnTerminalWindow gets focus on the Terminal window.
func (ta *TerminalApp) FocusMouseOnTerminalWindow(ctx context.Context) error {
	// Update node Root.
	if err := ta.Root.Update(ctx); err != nil {
		return errors.Wrap(err, "failed to update Rood node of Terminal app")
	}

	// Left click Terminal window to focus.
	if err := ta.Root.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to focus in Terminal app")
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
	defer ta.Root.Release(ctx)

	if err := ta.RunCommand(ctx, keyboard, "exit"); err != nil {
		return errors.Wrap(err, "failed to exit Terminal window")
	}

	// Wait for window to close.
	return ui.WaitUntilGone(ctx, ta.tconn, ui.FindParams{Name: ta.Root.Name, Role: ta.Root.Role, ClassName: ta.Root.ClassName}, time.Minute)
}

// CreateFileWithApp creates a file with an app command and types a string into it and save it in container.
func (ta *TerminalApp) CreateFileWithApp(ctx context.Context, keyboard *input.KeyboardEventWriter, tconn *chrome.TestConn, cmd, appName, testString, uiString string) error {
	// Open file through running the command of the app in Terminal.
	if err := ta.RunCommand(ctx, keyboard, cmd); err != nil {
		return errors.Wrapf(err, "failed to run command %q in Terminal window", cmd)
	}

	param := ui.FindParams{
		Name: uiString,
		Role: ui.RoleTypeWindow,
	}

	// Find the app window.
	appWindow, err := ui.FindWithTimeout(ctx, tconn, param, 15*time.Second)
	if err != nil {
		return errors.Wrapf(err, "failed to find the %s window", appName)
	}

	// Sometimes left click could not focus on the new window. Moving the mouse first to make sure the cursor goes to the new window.
	if err = mouse.Move(ctx, tconn, appWindow.Location.CenterPoint(), 5*time.Second); err != nil {
		return errors.Wrapf(err, "failed to move to the center of the %s window", appName)
	}

	// Left click the app window.
	if err = appWindow.LeftClick(ctx); err != nil {
		return errors.Wrapf(err, "failed left click on %s window", appName)
	}

	// Type test string into the new file.
	if err = keyboard.Type(ctx, testString); err != nil {
		return errors.Wrapf(err, "failed to type %q in %s window", testString, appName)
	}

	// Press ctrl+S to save the file.
	if err = keyboard.Accel(ctx, "ctrl+S"); err != nil {
		return errors.Wrapf(err, "failed to press ctrl+S in %s window", appName)
	}

	// Press ctrl+W twice to exit window.
	if err = keyboard.Accel(ctx, "ctrl+W"); err != nil {
		return errors.Wrapf(err, "failed to press ctrl+W in %s window", appName)
	}
	if err = keyboard.Accel(ctx, "ctrl+W"); err != nil {
		return errors.Wrapf(err, "failed to press ctrl+W in %s window", appName)
	}

	if err = ui.WaitUntilGone(ctx, tconn, param, 15*time.Second); err != nil {
		return errors.Wrapf(err, "failed to close %s window", appName)
	}
	return nil
}
