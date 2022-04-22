// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package terminalapp supports actions on Terminal on Chrome OS.
package terminalapp

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

const uiTimeout = 15 * time.Second

var (
	linuxLink  = nodewith.Name("Linux").Role(role.Link)
	linuxTab   = nodewith.NameContaining("@penguin: ").Role(role.Window).ClassName("BrowserFrame")
	rootWindow = nodewith.NameStartingWith("Terminal").Role(role.Window).ClassName("BrowserFrame")
	homeTab    = nodewith.Name("Terminal").Role(role.Window).ClassName("BrowserFrame")
	sshWebArea = nodewith.Name("chronos@localhost:~").Role(role.RootWebArea)
)

// TerminalApp represents an instance of the Terminal App.
type TerminalApp struct {
	tconn *chrome.TestConn
	ui    *uiauto.Context
	kb    *input.KeyboardEventWriter
}

// Launch launches the Terminal App connected to default penguin container and returns it.
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

// Find finds an open Terminal App and opens a Linux tab if not already open.
// An error is returned if terminal cannot be found.
func Find(ctx context.Context, tconn *chrome.TestConn) (*TerminalApp, error) {
	ui := uiauto.New(tconn)

	// Find either Linux tab or Home tab with Linux link.
	err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := ui.Exists(linuxTab)(ctx); err == nil {
			return nil
		}
		if err := ui.Exists(linuxLink)(ctx); err == nil {
			return nil
		}
		return errors.New("waiting for Linux tab, or home tab with Linux link")
	}, &testing.PollOptions{Timeout: 10 * time.Second})

	// If this is Home tab with Linux link, open Linux tab.
	if err = ui.Exists(linuxLink)(ctx); err == nil {
		if err := ui.LeftClick(nodewith.Name("Linux").Role(role.Link))(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to click Terminal Home Linux")
		}
	}

	// Ensure Linux tab is active and container is connected.
	if err := ui.WaitUntilExists(linuxTab)(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to find the Terminal App window")
	}

	// Check Terminal is on shelf.
	if err := ash.WaitForApp(ctx, tconn, apps.Terminal.ID, time.Minute); err != nil {
		return nil, errors.Wrap(err, "failed to find Terminal icon on shelf")
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find keyboard")
	}

	terminalApp := &TerminalApp{tconn: tconn, ui: ui, kb: kb}
	if err := terminalApp.WaitForPrompt()(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to wait for terminal prompt")
	}

	return terminalApp, nil
}

// LaunchSSH launches Terminal App and connects to usernameATHost
// with the optional sshArgs. An error is returned if the app fails to launch.
func LaunchSSH(ctx context.Context, tconn *chrome.TestConn, usernameAtHost, sshArgs, password string) (*TerminalApp, error) {
	// Launch the Terminal App.
	if err := apps.Launch(ctx, tconn, apps.Terminal.ID); err != nil {
		return nil, errors.Wrap(err, "failed to launch the Terminal App through package apps")
	}
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find keyboard")
	}

	// If an existing 'chronos@localhost' exists, delete it.
	ui := uiauto.New(tconn)
	var ta = &TerminalApp{tconn: tconn, ui: ui, kb: kb}

	loggedInPrompt := nodewith.Name(" ~ $").Role(role.StaticText).Ancestor(sshWebArea)
	if err := uiauto.Combine("launch ssh",
		ta.DeleteSSHConnection(usernameAtHost),
		ui.LeftClick(nodewith.Name("Add SSH").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Command").Role(role.TextField)),
		kb.TypeAction(usernameAtHost+" -o StrictHostKeyChecking=no "+sshArgs),
		ui.LeftClick(nodewith.Name("OK").Role(role.Button)),
		ui.LeftClick(nodewith.Name(usernameAtHost).Role(role.Link)),
		ui.LeftClick(nodewith.Name("("+usernameAtHost+") Password:").Role(role.TextField)),
		kb.TypeAction(password),
		kb.AccelAction("Enter"),
		ui.WaitUntilExists(loggedInPrompt),
	)(ctx); err != nil {
		return nil, err
	}

	return ta, nil
}

// DeleteSSHConnection deletes the specified connection link if it exists.
func (ta *TerminalApp) DeleteSSHConnection(name string) uiauto.Action {
	l := nodewith.Name(name).Role(role.Link)
	return action.IfSuccessThen(ta.ui.WithTimeout(3*time.Second).WaitUntilExists(l),
		uiauto.Combine("delete ssh link "+name,
			ta.ui.LeftClick(nodewith.Name("Edit SSH").Role(role.Link)),
			ta.ui.LeftClick(nodewith.Name("Delete").Role(role.Button)),
			ta.ui.WaitUntilGone(l),
		))
}

// ExitSSH exits the current connection and closes the app.
func (ta *TerminalApp) ExitSSH() uiauto.Action {
	exitDialog := nodewith.NameStartingWith("Program exited with status code").
		Role(role.StaticText).Ancestor(nodewith.Role(role.Dialog))
	terminalWebArea := nodewith.Name("Terminal").Role(role.RootWebArea)
	return uiauto.Combine("exit ssh",
		ta.ui.LeftClick(sshWebArea.First()),
		ta.kb.TypeAction("exit"),
		ta.kb.AccelAction("Enter"),
		ta.ui.WaitUntilExists(exitDialog),
		ta.kb.TypeAction("x"),
		ta.ui.WaitUntilExists(terminalWebArea),
		ta.kb.AccelAction("Ctrl+W"),
	)
}

// WaitForPrompt waits until the terminal window shows a shell
// prompt. Useful for either waiting for the startup process to finish
// or for a terminal application to exit.
func (ta *TerminalApp) WaitForPrompt() uiauto.Action {
	webArea := nodewith.NameRegex(regexp.MustCompile(`\@penguin\: `)).Role(role.RootWebArea)
	prompt := nodewith.Name("$ ").Role(role.StaticText).Ancestor(webArea)
	return ta.ui.WithTimeout(3 * time.Minute).WaitUntilExists(prompt)
}

// ClickShelfMenuItem right clicks the terminal app icon on the shelf and left click the specified menu item.
func (ta *TerminalApp) ClickShelfMenuItem(itemNameRegexp string) uiauto.Action {
	return func(ctx context.Context) error {
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
		if revert != nil {
			defer revert(ctx)
		}

		return uiauto.Combine("click menu item on the Shelf",
			ta.ui.RightClick(nodewith.Name("Terminal").Role(role.Button).First()),
			ta.ui.LeftClick(nodewith.NameRegex(regexp.MustCompile(itemNameRegexp)).Role(role.MenuItem)))(ctx)
	}
}

// LaunchThroughIcon launches Crostini by clicking the terminal app icon in launcher.
func LaunchThroughIcon(ctx context.Context, tconn *chrome.TestConn) (*TerminalApp, error) {
	// TODO(jinrongwu): type the whole name of Terminal instead of t when the following problem fixed.
	// The problem is: the launcher exits if typing more than one letter. This problem does not exists in other tests.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find keyboard")
	}
	if err := launcher.SearchAndLaunchWithQuery(tconn, kb, "t", "Terminal")(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to launch Terminal app")
	}

	ta, err := Find(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find Terminal window")
	}
	return ta, nil
}

// RestartCrostini shuts down Crostini and launch and exit the Terminal window.
func (ta *TerminalApp) RestartCrostini(keyboard *input.KeyboardEventWriter, cont *vm.Container, userName string) uiauto.Action {
	return func(ctx context.Context) error {
		if err := ta.ShutdownCrostini(cont)(ctx); err != nil {
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

		if err := ta.ClickShelfMenuItem("Close")(ctx); err != nil {
			return errors.Wrap(err, "failed to close Terminal app")
		}

		return nil
	}
}

// ShutdownCrostini shuts down Crostini.
func (ta *TerminalApp) ShutdownCrostini(cont *vm.Container) uiauto.Action {
	return func(ctx context.Context) error {
		if err := ta.ClickShelfMenuItem("Shut down Linux")(ctx); err != nil {
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
			return errors.Wrap(err, "VM failed to stop")
		}

		return nil
	}
}

// RunCommand runs command in Terminal windows.
func (ta *TerminalApp) RunCommand(keyboard *input.KeyboardEventWriter, cmd string) uiauto.Action {
	return uiauto.Combine("run command "+cmd,
		// Focus on the Terminal window.
		ta.ui.LeftClick(linuxTab),
		// Type command.
		keyboard.TypeAction(cmd),
		// Press Enter.
		keyboard.AccelAction("Enter"))
}

// Exit closes the Terminal App through entering exit in the Terminal window.
func (ta *TerminalApp) Exit(keyboard *input.KeyboardEventWriter) uiauto.Action {
	return uiauto.Combine("exit Terminal window",
		ta.RunCommand(keyboard, "exit"),
		ta.ui.WithTimeout(time.Minute).WaitUntilGone(linuxTab))
}

// Close closes the Terminal App through clicking Close on shelf context menu.
func (ta *TerminalApp) Close() uiauto.Action {
	return uiauto.Combine("close Terminal window",
		ta.ClickShelfMenuItem("Close"),
		ta.ui.WithTimeout(time.Minute).WaitUntilGone(rootWindow))
}
