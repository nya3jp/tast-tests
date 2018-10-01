// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subtest

import (
	"context"
	"strings"

	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// LinuxPackageInfo queries the information for a Debian package that we have copied
// into the container.
func LinuxPackageInfo(ctx context.Context, s *testing.State, cont *vm.Container, filePath string) {
	s.Log("Executing PackageInfo test")
	err, packageId := cont.LinuxPackageInfo(ctx, filePath)
	if err != nil {
		s.Error("Failed getting LinuxPackageInfo: ", err)
		return
	}
	if !strings.HasPrefix(packageId, "cros-tast-tests;") {
		s.Errorf("LinuxPackageInfo returned an incorrect package id of: %q", packageId)
	}
}
