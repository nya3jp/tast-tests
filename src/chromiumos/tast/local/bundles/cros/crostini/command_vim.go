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
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/terminalapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CommandVim,
		Desc:         "Test vim in Terminal window",
		Contacts:     []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline"},
		Params:       crostini.MakeTestParams(crostini.TestInformational),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func CommandVim(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cr := s.PreValue().(crostini.PreData).Chrome
	keyboard := s.PreValue().(crostini.PreData).Keyboard
	cont := s.PreValue().(crostini.PreData).Container
	defer crostini.RunCrostiniPostTest(ctx, cont)

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	// Clean up the home directory in the end.
	defer func() {
		if err := cont.Cleanup(cleanupCtx, "."); err != nil {
			s.Error("Failed to remove all files in home directory in the container: ", err)
		}
	}()

	userName := strings.Split(cr.User(), "@")[0]

	// Open Terminal app.
	terminalApp, err := terminalapp.Launch(ctx, tconn, userName)
	if err != nil {
		s.Fatal("Failed to open Terminal app: ", err)
	}
	defer terminalApp.Exit(cleanupCtx, keyboard)

	// Install vim in container.
	if err := installVimInContainer(ctx, cont); err != nil {
		s.Fatal("Failed to install vim in container: ", err)
	}

	const (
		testFile   = "test.txt"
		testString = "This is a test string."
	)

	// Create a file using vim in Terminal.
	if err := createFileWithVim(ctx, keyboard, terminalApp, testFile, testString); err != nil {
		s.Fatal("Failed to create file with vim in Terminal: ", err)
	}

	// Check the content of the test file.
	if err := cont.CheckFileContent(ctx, testFile, testString+"\n"); err != nil {
		s.Fatal("The content of the file is wrong: ", err)
	}
}

// createFileWithVim creates a file with vim and types a string into it and save it in container.
func createFileWithVim(ctx context.Context, keyboard *input.KeyboardEventWriter, terminalApp *terminalapp.TerminalApp, testFile, testString string) error {
	// Open file through running command vim filename in Terminal.
	if err := terminalApp.RunCommand(ctx, keyboard, fmt.Sprintf("vim %s", testFile)); err != nil {
		return errors.Wrapf(err, "failed to run command vim %s in Terminal window", testFile)
	}

	// TODO: verify vim has opened successfully.

	// Type i to enter edit mode.
	if err := keyboard.Type(ctx, "i"); err != nil {
		return errors.Wrap(err, "failed to enter edit mode of vim in Terminal")
	}

	// Type test string into the new file.
	if err := keyboard.Type(ctx, testString); err != nil {
		return errors.Wrapf(err, "failed to type %s in Terminal", testString)
	}

	// Press ESC to exit edit mode.
	if err := keyboard.Type(ctx, string('\x1b')); err != nil {
		return errors.Wrap(err, "failed to exit edit mode of vim in Terminal")
	}

	// Type :x to save.
	if err := keyboard.Type(ctx, ":x"); err != nil {
		return errors.Wrap(err, "failed to type :x to save vim in Terminal")
	}

	// Press Enter.
	if err := keyboard.Accel(ctx, "Enter"); err != nil {
		return errors.Wrap(err, "failed to press Enter in Terminal")
	}
	return nil
}

// installVimInContainer installs vim in container.
func installVimInContainer(ctx context.Context, cont *vm.Container) error {
	// Check whether vim is preinstalled or not.
	if err := cont.Command(ctx, "vim", "--version").Run(testexec.DumpLogOnError); err == nil {
		return nil
	}

	// Run command sudo apt update in container.
	if err := cont.Command(ctx, "sudo", "apt", "update").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to run command sudo apt update in container")
	}

	// Run command sudo apt install vim in container.
	if err := cont.Command(ctx, "sudo", "DEBIAN_FRONTEND=noninteractive", "apt-get", "-y", "install", "vim").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to run command sudo apt install vim in container")
	}

	// Run command vim --version and check the output to make sure vim has been installed successfully.
	if err := cont.Command(ctx, "vim", "--version").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to install vim in container")
	}
	return nil
}
