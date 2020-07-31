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
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     AppVscode,
		Desc:     "Test visual studio code in Terminal window",
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
		SoftwareDeps: []string{"chrome", "vm_host", "amd64"},
	})
}
func AppVscode(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cr := s.PreValue().(crostini.PreData).Chrome
	keyboard := s.PreValue().(crostini.PreData).Keyboard
	cont := s.PreValue().(crostini.PreData).Container

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 90*time.Second)
	defer cancel()

	// Reboot the container in the end in case any error in the middle and visual studio code is not closed.
	defer cont.Reboot(cleanupCtx)

	// Install visual studio code in container. This is a workaround until visual studio is pre-installed in a image.
	if err := installVscodeInContainer(ctx, cont); err != nil {
		s.Fatal("Failed to install visual studio code in container: ", err)
	}

	// Open Terminal app.
	terminalApp, err := terminalapp.Launch(ctx, tconn, strings.Split(cr.User(), "@")[0])
	if err != nil {
		s.Fatal("Failed to open Terminal app: ", err)
	}
	defer terminalApp.Close(cleanupCtx, keyboard)

	// Create a file using visual studio code in Terminal.
	const (
		testFile   = "test.go"
		testString = "//This is a test string."
	)
	if err := terminalApp.CreateFileWithApp(ctx, keyboard, tconn, fmt.Sprintf("code %s", testFile), "Visual Studio Code", testString, fmt.Sprintf("● %s - Visual Studio Code", testFile)); err != nil {
		s.Fatal("Failed to create file with visual studio code in Terminal: ", err)
	}

	// Check the content of the test file.
	if err := cont.CheckFileContent(ctx, testFile, testString); err != nil {
		s.Fatal("Failed to verify the content of the file: ", err)
	}
}

// installVscodeInContainer installs visual studio code in container.
func installVscodeInContainer(ctx context.Context, cont *vm.Container) error {
	// Check whether visual studio code has been installed.
	if err := cont.Command(ctx, "code", "--version").Run(testexec.DumpLogOnError); err == nil {
		return nil
	}
	testing.ContextLog(ctx, "Installing visual studio code")
	if err := cont.RunMultiCommandsInSequence(ctx, "sudo apt-get update",
		"sudo DEBIAN_FRONTEND=noninteractive apt-get -y install software-properties-common",
		"curl -sSL https://packages.microsoft.com/keys/microsoft.asc | sudo apt-key add -",
		"sudo add-apt-repository \"deb [arch=amd64] https://packages.microsoft.com/repos/vscode stable main\"",
		"sudo apt-get update",
		"sudo DEBIAN_FRONTEND=noninteractive apt-get -y install code"); err != nil {
		return errors.Wrap(err, "failed to install visual studio code in container")
	}

	if err := cont.Command(ctx, "code", "--version").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to install visual studio code")
	}
	return nil
}
