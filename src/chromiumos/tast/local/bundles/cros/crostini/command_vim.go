// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/terminalapp"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CommandVim,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test vim in Terminal window",
		Contacts:     []string{"clumptini+oncall@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Params: []testing.Param{
			// Parameters generated by params_test.go. DO NOT EDIT.
			{
				Name:              "buster_stable",
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Fixture:           "crostiniBuster",
				Timeout:           7 * time.Minute,
			}, {
				Name:              "buster_unstable",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				Fixture:           "crostiniBuster",
				Timeout:           7 * time.Minute,
			}, {
				Name:              "bullseye_stable",
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Fixture:           "crostiniBullseye",
				Timeout:           7 * time.Minute,
			}, {
				Name:              "bullseye_unstable",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				Fixture:           "crostiniBullseye",
				Timeout:           7 * time.Minute,
			},
		},
	})
}

func CommandVim(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(crostini.FixtureData).Tconn
	keyboard := s.FixtValue().(crostini.FixtureData).KB
	cont := s.FixtValue().(crostini.FixtureData).Cont

	// Open Terminal app.
	terminalApp, err := terminalapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to open Terminal app: ", err)
	}

	// Install vim in container.
	if err := installVimInContainer(ctx, cont); err != nil {
		s.Fatal("Failed to install vim in container: ", err)
	}

	const (
		testFile   = "test.txt"
		testString = "This is a test string."
	)

	if err := uiauto.Combine("create a file with vim",
		// Open file through running command vim filename in Terminal.
		terminalApp.RunCommand(keyboard, fmt.Sprintf("vim %s", testFile)),
		// Type i to enter edit mode.
		keyboard.TypeAction("i"),
		// Type test string into the new file.
		keyboard.TypeAction(testString),
		// Press ESC to exit edit mode.
		keyboard.TypeAction(string('\x1b')),
		// Type :x to save.
		keyboard.TypeAction(":x"),
		// Press Enter.
		keyboard.AccelAction("Enter"),
		// Wait for vim to exit
		terminalApp.WaitForPrompt())(ctx); err != nil {
		s.Fatal("Failed to create file with vim in Terminal: ", err)
	}

	// Check the content of the test file.
	if err := cont.CheckFileContent(ctx, testFile, testString+"\n"); err != nil {
		s.Fatal("The content of the file is wrong: ", err)
	}
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
