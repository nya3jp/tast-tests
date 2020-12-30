// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multivm

import (
	"context"
	"time"

	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/multivm"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Login,
		Desc:         "Tests Chrome Login with different VMs running",
		Contacts:     []string{"cwd@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		Timeout:      10 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "novm",
			Pre:  multivm.NoVMStarted(),
		}, {
			Name:              "arc_crostini",
			Pre:               multivm.ArcCrostiniStarted(),
			ExtraData:         []string{vm.ArtifactData(), crostini.GetContainerMetadataArtifact("buster", false), crostini.GetContainerRootfsArtifact("buster", false)},
			ExtraHardwareDeps: crostini.CrostiniStable,
			ExtraSoftwareDeps: []string{"vm_host", "android_vm"},
		}, {
			Name:              "arc",
			Pre:               multivm.ArcStarted(),
			ExtraSoftwareDeps: []string{"android_vm"},
		}, {
			Name:              "crostini",
			Pre:               multivm.CrostiniStarted(),
			ExtraData:         []string{vm.ArtifactData(), crostini.GetContainerMetadataArtifact("buster", false), crostini.GetContainerRootfsArtifact("buster", false)},
			ExtraHardwareDeps: crostini.CrostiniStable,
			ExtraSoftwareDeps: []string{"vm_host"},
		}},
	})
}

func Login(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(*multivm.PreData)

	if err := pre.Chrome.Responded(ctx); err != nil {
		s.Fatal("Chrome did not respond: ", err)
	}

	if pre.ARC != nil {
		// Ensures package manager service is running by checking the existence of the "android" package.
		pkgs, err := pre.ARC.InstalledPackages(ctx)
		if err != nil {
			s.Fatal("Getting installed packages failed: ", err)
		}

		if _, ok := pkgs["android"]; !ok {
			s.Fatal("Android package not found: ", pkgs)
		}
	}

	if pre.Crostini != nil {
		if err := crostini.BasicCommandWorks(ctx, pre.Crostini); err != nil {
			s.Fatal("Crostini basic commands don't work: ", err)
		}
	}
}
