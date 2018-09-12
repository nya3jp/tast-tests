// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subtest

import (
	"strings"

	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// LinuxPackageInstall performs the installation for a Debian package that we
// have copied into the container.
func LinuxPackageInstall(s *testing.State, cont *vm.Container) {
	s.Log("Executing LinuxPackageInstall test")
	err := cont.LinuxPackageInstall(s.Context(), "/home/testuser/cros-tast-tests-deb.deb")
	if err != nil {
		s.Error("Failed getting LinuxPackageInfo: ", err)
		return
	}
	if !strings.HasPrefix(packageId, "cros-tast-tests;") {
		s.Errorf("LinuxPackageInfo returned an incorrect package id of: %q", packageId)
	}
}
