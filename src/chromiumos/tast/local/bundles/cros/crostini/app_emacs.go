// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/terminalapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     AppEmacs,
		Desc:     "Test Emacs in Terminal window",
		Contacts: []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"keepState"},
		Params: []testing.Param{{
			Name:              "download_buster",
			Pre:               crostini.StartedByDownloadBusterLargeContainer(),
			ExtraHardwareDeps: crostini.CrostiniAppTest,
			Timeout:           15 * time.Minute,
		}},
		SoftwareDeps: []string{"chrome", "vm_host", "amd64"},
	})
}
func AppEmacs(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cr := s.PreValue().(crostini.PreData).Chrome
	keyboard := s.PreValue().(crostini.PreData).Keyboard
	cont := s.PreValue().(crostini.PreData).Container

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 90*time.Second)
	defer cancel()
	defer crostini.RunCrostiniPostTest(cleanupCtx, s.PreValue().(crostini.PreData))

	// Open Terminal app.
	terminalApp, err := terminalapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to open Terminal app: ", err)
	}

	restartIfError := true

	defer func() {
		// Restart Crostini in the end in case any error in the middle and Emacs is not closed.
		// This also closes the Terminal window.
		if restartIfError {
			if err := terminalApp.RestartCrostini(cleanupCtx, keyboard, cont, cr.User()); err != nil {
				s.Log("Failed to restart Crostini: ", err)
			}
		} else {
			terminalApp.Exit(cleanupCtx, keyboard)
		}
	}()

	if err := createFileWithEmacs(ctx, keyboard, terminalApp, tconn, cont); err != nil {
		s.Fatal("Failed to create file with emacs in Terminal: ", err)
	}

	restartIfError = false
}

// createFileWithEmacs creates a file with emacs and types a string into it and save it in container.
func createFileWithEmacs(ctx context.Context, keyboard *input.KeyboardEventWriter, terminalApp *terminalapp.TerminalApp, tconn *chrome.TestConn, cont *vm.Container) error {
	const (
		testFile   = "test.txt"
		testString = "This is a test string"
	)

	// Open emacs in Terminal.
	if err := terminalApp.RunCommand(ctx, keyboard, fmt.Sprintf("emacs %s", testFile)); err != nil {
		return errors.Wrap(err, "failed to run command 'emacs' in Terminal window")
	}

	// Find app window.
	param := ui.FindParams{
		Name: "emacs@penguin",
		Role: ui.RoleTypeWindow,
	}
	appWindow, err := ui.FindWithTimeout(ctx, tconn, param, 15*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find emacs window")
	}

	// Left click left-top of the emacs window.
	if err = mouse.Click(ctx, tconn, coords.Point{X: appWindow.Location.Left, Y: appWindow.Location.Top}, mouse.LeftButton); err != nil {
		return errors.Wrap(err, "failed left click on emacs window")
	}

	// Type test string into the new file.
	if err = keyboard.Type(ctx, testString); err != nil {
		return errors.Wrapf(err, "failed to type %s in emacs window", testString)
	}

	// Press ctrl+x and ctrl+s to save.
	if err = keyboard.Accel(ctx, "ctrl+X"); err != nil {
		return errors.Wrap(err, "failed to press ctrl+X in emacs window")
	}
	if err = keyboard.Accel(ctx, "ctrl+S"); err != nil {
		return errors.Wrap(err, "failed to press ctrl+C in emacs window")
	}

	// Press ctrl+x and ctrl+c to and quit.
	if err = keyboard.Accel(ctx, "ctrl+X"); err != nil {
		return errors.Wrap(err, "failed to press ctrl+X in emacs window")
	}
	if err = keyboard.Accel(ctx, "ctrl+C"); err != nil {
		return errors.Wrap(err, "failed to press ctrl+C in emacs window")
	}

	if err = ui.WaitUntilGone(ctx, tconn, param, 15*time.Second); err != nil {
		return errors.Wrap(err, "failed to close emacs window")
	}

	// Check the content of the test file.
	if err := cont.CheckFileContent(ctx, testFile, testString+"\n"); err != nil {
		return errors.Wrap(err, "failed to verify the content of the file")
	}
	return nil
}
