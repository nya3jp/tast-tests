// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package terminalapp supports actions on Terminal on Chrome OS.
package terminalapp

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uig"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
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
	// Launch the Terminal App.
	if err := apps.Launch(ctx, tconn, apps.Terminal.ID); err != nil {
		return nil, err
	}
	return Find(ctx, tconn, userName, "~")
}

// Find finds an open Terminal App with the specified userName and current.
// working directory (cwd). An error is returned if terminal cannot be found.
func Find(ctx context.Context, tconn *chrome.TestConn, userName, cwd string) (*TerminalApp, error) {
	title := userName + "@penguin: " + cwd
	rootFindParams := ui.FindParams{
		Name:      title,
		Role:      ui.RoleTypeWindow,
		ClassName: "BrowserFrame",
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

	terminalApp := &TerminalApp{tconn: tconn, Root: app}
	return terminalApp, terminalApp.waitForPrompt(ctx, title)
}

func (ta *TerminalApp) waitForPrompt(ctx context.Context, title string) error {
	waitForPrompt := uig.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeRootWebArea, Name: title}, uiTimeout).
		FindWithTimeout(ui.FindParams{Role: ui.RoleTypeStaticText, Name: "$ "}, 90*time.Second).
		WithNamef("Terminal.waitForPrompt()")
	return uig.Do(ctx, ta.tconn, waitForPrompt)
}

// clickShelfMenuItem shuts down crostini by right clicking on the terminal app shelf icon.
func (ta *TerminalApp) clickShelfMenuItem(ctx context.Context, itemName string) error {
	revert, err := ash.EnsureTabletModeEnabled(ctx, ta.tconn, false)
	if err != nil {
		return errors.Wrap(err, "Unable to switch out of tablet mode")
	}
	defer revert(ctx)

	shutdown := uig.Steps(
		uig.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeButton, Name: "Terminal"}, uiTimeout).RightClick(),
		uig.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeMenuItem, Name: itemName}, uiTimeout).LeftClick(),
	).WithNamef("TerminalApp.clickShelfMenuItem()")
	return uig.Do(ctx, ta.tconn, shutdown)
}

// RestartCrostini shuts down Crostini and launch and exit the Terminal window.
func (ta *TerminalApp) RestartCrostini(ctx context.Context, keyboard *input.KeyboardEventWriter, cont *vm.Container, userName string) error {
	if err := ta.ShutdownCrostini(ctx, cont); err != nil {
		return errors.Wrap(err, "failed to shutdown crostini")
	}

	// Start the VM and container.
	ta, err := Launch(ctx, ta.tconn, strings.Split(userName, "@")[0])
	if err != nil {
		return errors.Wrap(err, "failed to lauch terminal")
	}

	if err := cont.Connect(ctx, userName); err != nil {
		return errors.Wrap(err, "failed to connect to restarted container")
	}

	if err := ta.clickShelfMenuItem(ctx, "Close"); err != nil {
		return errors.Wrap(err, "failed to close Terminal app")
	}

	return nil
}

// ShutdownCrostini shuts down Crostini.
func (ta *TerminalApp) ShutdownCrostini(ctx context.Context, cont *vm.Container) error {
	if err := ta.clickShelfMenuItem(ctx, "Shut down Linux (Beta)"); err != nil {
		return errors.Wrap(err, "failed to shutdown crostini")
	}

	err := testing.Poll(ctx, func(ctx context.Context) error {
		// While the VM is down, this command is expected to fail.
		if out, err := cont.Command(ctx, "pwd").Output(); err == nil {
			return errors.Errorf("expected command to fail while the container was shut down, but got: %q", string(out))
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
	if err != nil {
		return errors.Wrap(err, "VM failed to stop: ")
	}

	return nil
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

// Exit closes the Terminal App through entering exit in the Terminal window.
func (ta *TerminalApp) Exit(ctx context.Context, keyboard *input.KeyboardEventWriter) error {
	defer ta.Root.Release(ctx)

	if err := ta.RunCommand(ctx, keyboard, "exit"); err != nil {
		return errors.Wrap(err, "failed to exit Terminal window")
	}

	// Wait for window to close.
	return ui.WaitUntilGone(ctx, ta.tconn, ui.FindParams{Name: ta.Root.Name, Role: ta.Root.Role, ClassName: ta.Root.ClassName}, time.Minute)
}

// Close closes the Terminal App through clicking Close on shelf context menu.
func (ta *TerminalApp) Close(ctx context.Context) error {
	defer ta.Root.Release(ctx)

	if err := ta.clickShelfMenuItem(ctx, "Close"); err != nil {
		return errors.Wrap(err, "failed to close crostini")
	}

	// Wait for window to close.
	return ui.WaitUntilGone(ctx, ta.tconn, ui.FindParams{Name: ta.Root.Name, Role: ta.Root.Role, ClassName: ta.Root.ClassName}, time.Minute)
}
