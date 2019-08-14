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
		Func:         PackageInfo,
		Desc:         "Queries the information for a Debian package that we have copied into the container",
		Contacts:     []string{"smbarber@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"informational"},
		Timeout:      7 * time.Minute,
		Data:         []string{crostini.ImageArtifact, "package.deb"},
		Pre:          crostini.StartedByArtifact(),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func PackageInfo(ctx context.Context, s *testing.State) {
	cont := s.PreValue().(crostini.PreData).Container
	filePath := "/home/testuser/package.deb"

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
		s.Fatalf("LinuxPackageInfo returned an incorrect package id of: %q", packageID)
	}
	s.Log("Package ID: " + packageID)
}
