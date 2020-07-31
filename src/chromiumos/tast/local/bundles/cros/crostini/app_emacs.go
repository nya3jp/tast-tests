// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"strings"
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
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     AppEmacs,
		Desc:     "Test Emacs in Terminal window",
		Contacts: []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:              "artifact",
			Pre:               crostini.StartedByArtifact(),
			ExtraData:         []string{crostini.ImageArtifact},
			Timeout:           7 * time.Minute,
			ExtraHardwareDeps: crostini.CrostiniStable,
		}, {
			Name:    "download_buster",
			Pre:     crostini.StartedByDownloadBuster(),
			Timeout: 10 * time.Minute,
		}},
		SoftwareDeps: []string{"chrome", "vm_host"},
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

	// Reboot the container in the end in case any error in the middle and emacs is not closed.
	defer cont.Reboot(cleanupCtx)

	// Install emacs in container. This is a workaround until emacs is pre-installed in a image.
	if err := installEmacsInContainer(ctx, cont); err != nil {
		s.Fatal("Failed to install emacs in Terminal window: ", err)
	}

	// Open Terminal app.
	terminalApp, err := terminalapp.Launch(ctx, tconn, strings.Split(cr.User(), "@")[0])
	if err != nil {
		s.Fatal("Failed to open Terminal app: ", err)
	}
	defer terminalApp.Close(cleanupCtx, keyboard)

	// Create a file with emacs in Terminal.
	const (
		testFile   = "test.txt"
		testString = "This is a test string"
	)
	if err := createFileWithEmacs(ctx, keyboard, terminalApp, tconn, testFile, testString); err != nil {
		s.Fatal("Failed to create file with emacs in Terminal: ", err)
	}

	// Check the content of the test file.
	if err := cont.CheckFileContent(ctx, testFile, testString+"\n"); err != nil {
		s.Fatal("Failed to verify the content of the file: ", err)
	}
}

// installEmacsInContainer installs emacs in container.
func installEmacsInContainer(ctx context.Context, cont *vm.Container) error {
	// Check whether emas has been installed or not.
	if err := cont.Command(ctx, "emacs", "--version").Run(testexec.DumpLogOnError); err == nil {
		return nil
	}

	testing.ContextLog(ctx, "Installing emacs")
	if err := cont.RunMultiCommandsInSequence(ctx, "sudo apt-get update", "sudo DEBIAN_FRONTEND=noninteractive apt-get -y install emacs"); err != nil {
		return errors.Wrap(err, "failed to install emacs in container")
	}

	if err := cont.Command(ctx, "emacs", "--version").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to install emacs")
	}
	return nil
}

// createFileWithEmacs creates a file with emacs and types a string into it and save it in container.
func createFileWithEmacs(ctx context.Context, keyboard *input.KeyboardEventWriter, terminalApp *terminalapp.TerminalApp, tconn *chrome.TestConn, testFile, testString string) error {
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

	// Press ctrl+x and ctrl+c to save and quit.
	if err = keyboard.Accel(ctx, "ctrl+X"); err != nil {
		return errors.Wrap(err, "failed to press ctrl+X in emacs window")
	}
	if err = keyboard.Accel(ctx, "ctrl+C"); err != nil {
		return errors.Wrap(err, "failed to press ctrl+C in emacs window")
	}

	// Left click right-bottom of the emacs window.
	if err = mouse.Click(ctx, tconn, coords.Point{X: appWindow.Location.Left + appWindow.Location.Width, Y: appWindow.Location.Top + appWindow.Location.Height}, mouse.LeftButton); err != nil {
		return errors.Wrap(err, "failed left click on emacs window")
	}

	// Type y.
	if err = keyboard.Type(ctx, "y"); err != nil {
		return errors.Wrap(err, "failed to type y in emacs window")
	}

	if err = ui.WaitUntilGone(ctx, tconn, param, 15*time.Second); err != nil {
		return errors.Wrap(err, "failed to close emacs window")
	}
	return nil
}
