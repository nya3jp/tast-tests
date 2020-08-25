// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     RunWithARC,
		Desc:     "Checks that ARC(VM) runs in parallel with Crostini",
		Contacts: []string{"niwa@chromium.org", "arcvm-eng@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"keepState"},
		Timeout:  7 * time.Minute,
		Data:     []string{crostini.ImageArtifact},
		Pre:      crostini.StartedARCEnabled(),
		// TODO(b/150013652): Stop using 'arc' here and use ExtraSoftwareDeps instead.
		SoftwareDeps: []string{"chrome", "vm_host", "arc"},
		Params: []testing.Param{
			{
				Name:              "artifact",
				ExtraHardwareDeps: crostini.CrostiniStable,
			},
			{
				Name:              "artifact_unstable",
				ExtraHardwareDeps: crostini.CrostiniUnstable,
			},
		},
	})
}

func RunWithARC(ctx context.Context, s *testing.State) {
	// First ensure crostini works in isolation by running a simple test.
	cont := s.PreValue().(crostini.PreData).Container
	if err := crostini.BasicCommandWorks(ctx, cont); err != nil {
		s.Fatal("Failed to run a command in the container: ", err)
	}
	defer crostini.RunCrostiniPostTest(ctx, cont)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	// Ensures package manager service is running by checking the existence of the "android" package.
	pkgs, err := a.InstalledPackages(ctx)
	if err != nil {
		s.Fatal("getting installed packages failed: ", err)
	}

	if _, ok := pkgs["android"]; !ok {
		s.Fatal("android package not found: ", pkgs)
	}
}
