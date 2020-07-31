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
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     AppGedit,
		Desc:     "Test gedit in Terminal window",
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
		HardwareDeps: hwdep.D(hwdep.Platform("hatch", "eve", "atlas", "nami")),
	})
}
func AppGedit(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cr := s.PreValue().(crostini.PreData).Chrome
	keyboard := s.PreValue().(crostini.PreData).Keyboard
	cont := s.PreValue().(crostini.PreData).Container

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 90*time.Second)
	defer cancel()

	// Open Terminal app.
	terminalApp, err := terminalapp.Launch(ctx, tconn, strings.Split(cr.User(), "@")[0])
	if err != nil {
		s.Fatal("Failed to open Terminal app: ", err)
	}

	// Restart crostini in the end in case any error in the middle and gedit is not closed.
	// This also closes the Terminal window.
	defer func() {
		if err := terminalApp.RestartCrostini(cleanupCtx, keyboard, cont, cr.User()); err != nil {
			s.Fatal("Failed to restart crostini: ", err)
		}
	}()

	// Install gedit in container.
	if err := installGeditInContainer(ctx, cont); err != nil {
		s.Fatal("Failed to install gedit in container: ", err)
	}

	const (
		testFile   = "test.txt"
		testString = "This is a test string"
	)

	// Create a file using gedit in Terminal.
	if err := terminalApp.CreateFileWithApp(ctx, keyboard, tconn, "gedit "+testFile, "Gedit", testString, fmt.Sprintf("%s (~/) - gedit", testFile)); err != nil {
		s.Fatal("Failed to create file with gedit in Terminal: ", err)
	}

	// Check the content of the test file.
	if err := cont.CheckFileContent(ctx, testFile, testString+"\n"); err != nil {
		s.Fatal("Failed to verify the content of the file: ", err)
	}
}

// installGeditInContainer installs gedit in container.
func installGeditInContainer(ctx context.Context, cont *vm.Container) error {
	// Check whether gedit has been installed.
	if err := cont.Command(ctx, "gedit", "--version").Run(testexec.DumpLogOnError); err == nil {
		return nil
	}

	testing.ContextLog(ctx, "Installing gedit")
	if err := cont.RunMultiCommandsInSequence(ctx, "sudo apt-get update", "sudo DEBIAN_FRONTEND=noninteractive apt-get -y install gedit"); err != nil {
		return errors.Wrap(err, "failed to install gedit in container")
	}

	if err := cont.Command(ctx, "gedit", "--version").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to install gedit")
	}
	return nil
}
