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
	"chromiumos/tast/local/chrome/ui/terminalapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     CommandPs,
		Desc:     "Test command ps in Terminal window",
		Contacts: []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Attr:     []string{"group:mainline", "informational"},
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

func CommandPs(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cr := s.PreValue().(crostini.PreData).Chrome
	keyboard := s.PreValue().(crostini.PreData).Keyboard
	cont := s.PreValue().(crostini.PreData).Container

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Open Terminal app.
	terminalApp, err := terminalapp.Launch(ctx, tconn, strings.Split(cr.User(), "@")[0])
	if err != nil {
		s.Fatal("Failed to open Terminal app: ", err)
	}
	defer terminalApp.Close(cleanupCtx, keyboard)

	outputFile := "test.txt"
	// Run command ps in Terminal window, redirect the output to a file for check.
	if err = terminalApp.RunCommand(ctx, keyboard, fmt.Sprintf("ps > %s", outputFile)); err != nil {
		s.Fatal("Failed to run command in Terminal window: ", err)
	}

	// Check the output of command ps.
	cmd := cont.Command(ctx, "cat", outputFile)
	result, err := cmd.Output()
	if err != nil {
		s.Fatal("Failed to cat the result file: ", err)
	}
	if !strings.Contains(string(result), "bash") || !strings.Contains(string(result), "ps") {
		s.Fatal("The out put of ps in container is wrong: ", string(result))
	}
}
