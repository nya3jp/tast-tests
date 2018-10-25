// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subtest

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// UninstallApplication tests uninstalling the application installed by the
// install package test.
func UninstallApplication(ctx context.Context, s *testing.State,
	cont *vm.Container, ownerID, desktopFileID, appID string) {
	testing.ContextLog(ctx, "Executing UninstallPackageOwningFile test")
	if err := cont.UninstallPackageOwningFile(ctx, desktopFileID); err != nil {
		s.Error("Failed executing UninstallPackageOwningFile: ", err)
		return
	}

	// Verify the package does not show up in the dpkg installed list.
	// Note that we pipe this output through "cat"; otherwise we get
	// unpredictable line length limits which may cause the grep to miss a package
	// (the ending of the package name can be cut off even if the package is still
	// installed)
	cmd := cont.Command(ctx, "sh", "-c", "dpkg -l | cat")
	if output, err := cmd.Output(); err != nil {
		s.Errorf("Error running dpkg -l: %v", err)
	} else if bytes.Contains(output, []byte("cros-tast-tests")) {
		s.Error("cros-tast-tests still in dpkg -l output")
	}

	// Verify the four files we expect to be installed were removed.
	for _, testFile := range installedFiles {
		cmd = cont.Command(ctx, "sh", "-c", "[ -f "+testFile+" ]")
		if err := cmd.Run(); err == nil {
			s.Errorf("File %v was not removed", testFile)
		}
	}

	// Check that the app is not installed
	s.Log("Checking that app icons do not exist")
	if err := checkIconNonExistence(ctx, ownerID, appID); err != nil {
		s.Error("Icon not removed: ", err)
	}
}

// checkIconNonExistence determines if the Crostini icon folder for the
// specified application exists in the filesystem and contains at least one
// file. It produces an error if so.
func checkIconNonExistence(ctx context.Context, ownerID, appID string) error {
	iconDir := filepath.Join("/home/user", ownerID, "crostini.icons", appID)
	// Remove happens some time after the install, so we need to poll.
	err := testing.Poll(ctx, func(context.Context) error {
		_, err := os.Stat(iconDir)
		if os.IsNotExist(err) {
			return nil // Directory doesn't exist; success
		}
		if err != nil {
			return err // Unexpected error
		}
		return errors.New("directory still exists")
	}, &testing.PollOptions{Timeout: 10 * time.Second})
	if err != nil {
		return errors.Wrapf(err, "could not confirm removal of %v", iconDir)
	}
	return nil
}
