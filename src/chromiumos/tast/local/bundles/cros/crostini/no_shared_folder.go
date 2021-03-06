// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/sharedfolders"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         NoSharedFolder,
		Desc:         "Test shared folder list in Settings app when there is no folder shared",
		Contacts:     []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline"},
		Vars:         []string{"keepState"},
		VarDeps:      []string{"ui.gaiaPoolDefault"},
		SoftwareDeps: []string{"chrome", "vm_host"},
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
func NoSharedFolder(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cont := s.PreValue().(crostini.PreData).Container
	cr := s.PreValue().(crostini.PreData).Chrome

	// Use a shortened context for test operations to reserve time for cleanup.
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()
	defer crostini.RunCrostiniPostTest(ctx, s.PreValue().(crostini.PreData))

	// Check list of shared folders in Settings app.
	sharedFolders := sharedfolders.NewSharedFolders(tconn)
	if err := sharedFolders.CheckNoSharedFolders(cont, cr)(ctx); err != nil {
		s.Fatal("Failed to check shared folders list by default: ", err)
	}
}
