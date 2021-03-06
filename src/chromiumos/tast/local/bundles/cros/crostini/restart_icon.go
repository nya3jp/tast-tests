// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/terminalapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RestartIcon,
		Desc:         "Tests that we can shut down and restart crostini through clicking the Terminal icon on launcher",
		Contacts:     []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Vars:         []string{"keepState"},
		Params: []testing.Param{
			// Parameters generated by params_test.go. DO NOT EDIT.
			{
				Name:              "stretch_stable",
				ExtraData:         []string{crostini.GetContainerMetadataArtifact("stretch", false), crostini.GetContainerRootfsArtifact("stretch", false)},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Pre:               crostini.StartedByDlcStretch(),
				Timeout:           7 * time.Minute,
			}, {
				Name:              "stretch_unstable",
				ExtraAttr:         []string{"informational"},
				ExtraData:         []string{crostini.GetContainerMetadataArtifact("stretch", false), crostini.GetContainerRootfsArtifact("stretch", false)},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				Pre:               crostini.StartedByDlcStretch(),
				Timeout:           7 * time.Minute,
			}, {
				Name:              "buster_stable",
				ExtraData:         []string{crostini.GetContainerMetadataArtifact("buster", false), crostini.GetContainerRootfsArtifact("buster", false)},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Pre:               crostini.StartedByDlcBuster(),
				Timeout:           7 * time.Minute,
			}, {
				Name:              "buster_unstable",
				ExtraAttr:         []string{"informational"},
				ExtraData:         []string{crostini.GetContainerMetadataArtifact("buster", false), crostini.GetContainerRootfsArtifact("buster", false)},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				Pre:               crostini.StartedByDlcBuster(),
				Timeout:           7 * time.Minute,
			},
		},
	})
}

func RestartIcon(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	cont := pre.Container
	tconn := pre.TestAPIConn
	defer crostini.RunCrostiniPostTest(ctx, s.PreValue().(crostini.PreData))

	terminalApp, err := terminalapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to lauch terminal: ", err)
	}

	if err := terminalApp.ShutdownCrostini(cont)(ctx); err != nil {
		s.Fatal("Failed to shutdown crostini: ", err)
	}

	terminalApp, err = terminalapp.LaunchThroughIcon(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to lauch terminal after shutdown: ", err)
	}

}
