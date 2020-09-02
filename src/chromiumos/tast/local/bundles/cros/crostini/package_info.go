// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     PackageInfo,
		Desc:     "Queries the information for a Debian package that we have copied into the container",
		Contacts: []string{"smbarber@chromium.org", "cros-containers-dev@google.com"},
		Attr:     []string{"group:mainline"},
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
			ExtraAttr:         []string{"informational"},
		}, {
			Name:      "download_stretch",
			Pre:       crostini.StartedByDownloadStretch(),
			Timeout:   10 * time.Minute,
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "download_buster",
			Pre:       crostini.StartedByDownloadBuster(),
			Timeout:   10 * time.Minute,
			ExtraAttr: []string{"informational"},
		}},
		Data:         []string{"package.deb"},
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func PackageInfo(ctx context.Context, s *testing.State) {
	cont := s.PreValue().(crostini.PreData).Container
	const filePath = "/home/testuser/package.deb"
	defer crostini.RunCrostiniPostTest(ctx, s.PreValue().(crostini.PreData))

	if err := crostini.TransferToContainer(ctx, cont, s.DataPath("package.deb"), filePath); err != nil {
		s.Fatal("Failed to transfer .deb to the container: ", err)
	}
	defer func() {
		if err := crostini.RemoveContainerFile(ctx, cont, filePath); err != nil {
			s.Fatal("Failed to remove .deb from the container: ", err)
		}
	}()

	packageID, err := cont.LinuxPackageInfo(ctx, filePath)
	if err != nil {
		s.Fatal("Failed getting LinuxPackageInfo: ", err)
	}
	if !strings.HasPrefix(packageID, "cros-tast-tests;") {
		s.Fatal("LinuxPackageInfo returned an incorrect package id: ", packageID)
	}
	s.Log("Package ID: " + packageID)
}
