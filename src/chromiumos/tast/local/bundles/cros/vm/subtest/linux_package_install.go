// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subtest

import (
	"errors"
	"fmt"

	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// LinuxPackageInstall performs the installation for a Debian package that we
// have copied into the container. This test does not log its own error because
// other tests will be gated on its success or failure so the result will be
// analyzed by the caller.
func LinuxPackageInstall(s *testing.State, cont *vm.Container) error {
	s.Log("Executing LinuxPackageInstall test")
	ctx := s.Context()
	err := cont.LinuxPackageInstall(ctx, "/home/testuser/cros-tast-tests-deb.deb")
	if err != nil {
		return fmt.Errorf("Failed executing LinuxPackageInstall: %v", err)
	}

	// Verify the package shows up in the dpkg installed list.
	cmd := cont.Command(ctx, "sh", "-c", "dpkg -l | grep cros-tast-tests")
	if err = cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return fmt.Errorf("Failed to run dpkg -l: %v", err)
	}
	if !cmd.ProcessState.Success() {
		return errors.New("Failed checking for cros-tast-tests in dpkg -l")
	}

	// Verify the two files we expect to be installed are actually there.
	cmd = cont.Command(ctx, "sh", "-c",
		"ls /usr/share/applications/x11_demo.desktop && ls /usr/share/applications/wayland_demo.desktop")
	if err = cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return fmt.Errorf("Failed to ls to check file existence: %v", err)
	}
	if !cmd.ProcessState.Success() {
		return errors.New("Failed checking for existence of x11_demo.desktop and wayland_demo.desktop")
	}

	return nil
}
