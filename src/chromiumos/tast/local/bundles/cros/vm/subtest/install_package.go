// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subtest

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// installedFiles is a list of the Linux files installed by our test .deb
// package.
var installedFiles = []string{
	"/usr/share/applications/x11_demo.desktop",
	"/usr/share/applications/x11_demo_fixed_size.desktop",
	"/usr/share/applications/wayland_demo.desktop",
	"/usr/share/applications/wayland_demo_fixed_size.desktop",
	"/usr/share/icons/hicolor/32x32/apps/x11_demo.png",
	"/usr/share/icons/hicolor/32x32/apps/wayland_demo.png",
}

// InstallPackage performs the installation for a Debian package that we
// have copied into the container. This test does not log its own error because
// other tests will be gated on its success or failure so the result will be
// analyzed by the caller.
func InstallPackage(ctx context.Context, cont *vm.Container, filePath string) error {
	testing.ContextLog(ctx, "Executing LinuxPackageInstall test")
	err := cont.InstallPackage(ctx, filePath)
	if err != nil {
		return errors.Wrap(err, "failed executing LinuxPackageInstall")
	}
	// Verify the package shows up in the dpkg installed list.
	cmd := cont.Command(ctx, "dpkg", "-s", "cros-tast-tests")
	if err = cmd.Run(); err != nil {
		return errors.New("failed checking for cros-tast-tests in dpkg -s")
	}

	// Verify the four files we expect to be installed are actually there.
	for _, testFile := range installedFiles {
		cmd = cont.Command(ctx, "sh", "-c", "[ -f "+testFile+" ]")
		if err = cmd.Run(); err != nil {
			return errors.Errorf("failed to check file existence of: %v", testFile)
		}
	}

	return nil
}
