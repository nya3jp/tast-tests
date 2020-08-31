// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/terminalapp"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     CommandCd,
		Desc:     "Test command cd in Terminal window",
		Contacts: []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"keepState"},
		Params: []testing.Param{{
			Name:              "artifact",
			Pre:               crostini.StartedByArtifact(),
			ExtraData:         []string{crostini.ImageArtifact},
			Timeout:           7 * time.Minute,
			ExtraHardwareDeps: crostini.CrostiniStable,
		}, {
			Name:              "artifact_unstable",
			Pre:               crostini.StartedByArtifact(),
			ExtraData:         []string{crostini.ImageArtifact},
			Timeout:           7 * time.Minute,
			ExtraHardwareDeps: crostini.CrostiniUnstable,
		}, {
			Name:    "download_stretch",
			Pre:     crostini.StartedByDownloadStretch(),
			Timeout: 10 * time.Minute,
		}, {
			Name:    "download_buster",
			Pre:     crostini.StartedByDownloadBuster(),
			Timeout: 10 * time.Minute,
		}},
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func CommandCd(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cr := s.PreValue().(crostini.PreData).Chrome
	keyboard := s.PreValue().(crostini.PreData).Keyboard
	cont := s.PreValue().(crostini.PreData).Container
	defer crostini.RunCrostiniPostTest(ctx,
		s.PreValue().(crostini.PreData).Container,
		s.PreValue().(crostini.PreData).Chrome.User())

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
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

	// Create a test folder.
	folderName := "testFolder"
	if err = terminalApp.RunCommand(ctx, keyboard, fmt.Sprintf("mkdir %s", folderName)); err != nil {
		s.Fatal("Failed to run command mkdir in Terminal window: ", err)
	}

	defer cont.Command(cleanupCtx, "rm", "-rf", folderName).Run(testexec.DumpLogOnError)

	// Cd to the newly created folder.
	if err = terminalApp.RunCommand(ctx, keyboard, fmt.Sprintf("cd %s", folderName)); err != nil {
		s.Fatal("Failed to run command in Terminal window: ", err)
	}

	// Run pwd to check the path has changed.
	outputFile := "test.txt"
	if err = terminalApp.RunCommand(ctx, keyboard, fmt.Sprintf("pwd > %s", outputFile)); err != nil {
		s.Fatal("Failed to run command in Terminal window: ", err)
	}

	// Check the content of the test file.
	if err := cont.CheckFileContent(ctx, filepath.Join("/home", userName, folderName, outputFile), fmt.Sprintf("/home/%s/%s\n", userName, folderName)); err != nil {
		s.Fatal("Cd failed to take user into the newly created folder: ", err)
	}
}
