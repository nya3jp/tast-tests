// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package terminalapp supports actions on Terminal on Chrome OS.
package terminalapp

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui/launcher"
	"chromiumos/tast/local/chrome/ui/pointer"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

const uiTimeout = 15 * time.Second

var (
	rootWindow = nodewith.NameRegex(regexp.MustCompile(`\@penguin\: `)).Role(role.Window).ClassName("BrowserFrame")
)

// TerminalApp represents an instance of the Terminal App.
type TerminalApp struct {
	tconn *chrome.TestConn
}

// Launch launches the Terminal App and returns it.
// An error is returned if the app fails to launch.
func Launch(ctx context.Context, tconn *chrome.TestConn) (*TerminalApp, error) {
	// Launch the Terminal App.
	if err := apps.Launch(ctx, tconn, apps.Terminal.ID); err != nil {
		return nil, errors.Wrap(err, "failed to launch the Terminal App through package apps")
	}
	ta, err := Find(ctx, tconn)
	if err != nil {
		if closeErr := apps.Close(ctx, tconn, apps.Terminal.ID); closeErr != nil {
			testing.ContextLog(ctx, "Error closing terminal app: ", closeErr)
		}
		return nil, errors.Wrap(err, "failed to find the Terminal App")
	}
	return ta, nil
}

// Find finds an open Terminal App. An error is returned if terminal cannot be found.
func Find(ctx context.Context, tconn *chrome.TestConn) (*TerminalApp, error) {
	ui := uiauto.New(tconn)
	if err := ui.WaitUntilExists(rootWindow)(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to find Ternimal window")
	}

	// Check Terminal is on shelf.
	if err := ash.WaitForApp(ctx, tconn, apps.Terminal.ID); err != nil {
		return nil, errors.Wrap(err, "failed to find Terminal icon on shelf")
	}

	terminalApp := &TerminalApp{tconn: tconn}
	if err := terminalApp.WaitForPrompt(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to wait for prompt")
	}

	return terminalApp, nil
}

// WaitForPrompt waits until the terminal window shows a shell
// prompt. Useful for either waiting for the startup process to finish
// or for a terminal application to exit.
func (ta *TerminalApp) WaitForPrompt(ctx context.Context) error {
	webArea := nodewith.NameRegex(regexp.MustCompile(`\@penguin\: `)).Role(role.RootWebArea)
	prompt := nodewith.Name("$ ").Role(role.StaticText).Ancestor(webArea)
	return uiauto.New(ta.tconn).WithTimeout(3 * time.Minute).WaitUntilExists(prompt)(ctx)
}

// clickShelfMenuItem right clicks the terminal app icon on the shelf and left click the specified menu item.
func (ta *TerminalApp) clickShelfMenuItem(ctx context.Context, itemNameRegexp string) (retErr error) {
	revert, err := ash.EnsureTabletModeEnabled(ctx, ta.tconn, false)
	if err != nil {
		testing.ContextLog(ctx, "Unable to switch out of tablet mode, try to swipe up the hot seat")
		tc, err := pointer.NewTouchController(ctx, ta.tconn)
		if err != nil {
			return errors.Wrap(err, "failed to create the touch controller")
		}
		defer tc.Close()
		if err := ash.SwipeUpHotseatAndWaitForCompletion(ctx, ta.tconn, tc.EventWriter(), tc.TouchCoordConverter()); err != nil {
			return errors.Wrap(err, "failed to swipe up the hotseat")
		}
	}
	defer func() {
		if revert != nil {
			revert(ctx)
		}
	}()

	ui := uiauto.New(ta.tconn)

	return uiauto.Combine("Click menu item on the Shelf",
		ui.RightClick(nodewith.Name("Terminal").Role(role.Button).First()),
		ui.LeftClick(nodewith.NameRegex(regexp.MustCompile(itemNameRegexp)).Role(role.MenuItem)),
	)(ctx)
}

// LaunchThroughIcon launches Crostini by clicking the terminal app icon in launcher.
func LaunchThroughIcon(ctx context.Context, tconn *chrome.TestConn) (*TerminalApp, error) {
	// TODO(jinrongwu): type the whole name of Terminal instead of t when the following problem fixed.
	// The problem is: the launcher exits if typing more than one letter. This problem does not exists in other tests.
	if err := launcher.SearchAndLaunchWithQuery(ctx, tconn, "t", "Terminal"); err != nil {
		return nil, errors.Wrap(err, "failed to launch Terminal app")
	}

	ta, err := Find(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find Terminal window")
	}
	return ta, nil
}

// RestartCrostini shuts down Crostini and launch and exit the Terminal window.
func (ta *TerminalApp) RestartCrostini(ctx context.Context, keyboard *input.KeyboardEventWriter, cont *vm.Container, userName string) error {
	if err := ta.ShutdownCrostini(ctx, cont); err != nil {
		return errors.Wrap(err, "failed to shutdown crostini")
	}

	// Start the VM and container.
	ta, err := Launch(ctx, ta.tconn)
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
	if err := ta.clickShelfMenuItem(ctx, "Shut down Linux"); err != nil {
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

// RunCommand runs command in Terminal windows.
func (ta *TerminalApp) RunCommand(ctx context.Context, keyboard *input.KeyboardEventWriter, cmd string) error {
	if err := uiauto.New(ta.tconn).LeftClick(rootWindow)(ctx); err != nil {
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
	if err := ta.RunCommand(ctx, keyboard, "exit"); err != nil {
		return errors.Wrap(err, "failed to exit Terminal window")
	}

	// Wait for window to close.
	return uiauto.New(ta.tconn).WithTimeout(time.Minute).WaitUntilGone(rootWindow)(ctx)
}

// Close closes the Terminal App through clicking Close on shelf context menu.
func (ta *TerminalApp) Close(ctx context.Context) error {
	if err := ta.clickShelfMenuItem(ctx, "Close"); err != nil {
		return errors.Wrap(err, "failed to close Crostini")
	}

	// Wait for window to close.
	return uiauto.New(ta.tconn).WithTimeout(time.Minute).WaitUntilGone(rootWindow)(ctx)
}
