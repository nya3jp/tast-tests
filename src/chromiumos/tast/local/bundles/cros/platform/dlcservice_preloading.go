// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"os"
	"path/filepath"

	dlctest "chromiumos/tast/local/bundles/cros/platform/dlc"
	"chromiumos/tast/local/dlc"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DLCServicePreloading,
		Desc:         "Verifies that DLC preloading works by setting up a prelaoded test DLC and installing it",
		Contacts:     []string{"kimjae@chromium.org", "chromeos-core-services@google.com"},
		SoftwareDeps: []string{"dlc"},
		Attr:         []string{"group:mainline"},
	})
}

func DLCServicePreloading(ctx context.Context, s *testing.State) {
	// Check dlcservice is up and running.
	if err := upstart.EnsureJobRunning(ctx, dlc.JobName); err != nil {
		s.Fatalf("Failed to ensure %s running: %v", dlc.JobName, err)
	}

	if err := dlc.Cleanup(ctx, dlc.Info{ID: dlctest.TestID2, Package: dlctest.TestPackage}); err != nil {
		s.Fatal("Initial cleanup failed: ", err)
	}
	// Deferred cleanup to always end with no test DLC installations.
	defer func() {
		if err := dlc.Cleanup(ctx, dlc.Info{ID: dlctest.TestID2, Package: dlctest.TestPackage}); err != nil {
			s.Error("Ending cleanup failed: ", err)
		}
	}()

	// Make sure that the test DLC is not installed using GetInstalled DBus method.
	dlcListOutputs, err := dlctest.GetInstalled(ctx)
	if err != nil {
		s.Fatal("GetInstall failed: ", err)
	}
	for _, dlcListOutput := range dlcListOutputs {
		if dlcListOutput.ID == dlctest.TestID2 {
			s.Fatal("Not continuing as ", dlctest.TestID2, " is already installed.")
		}
	}

	// Place test DLC into the preload path. Let cleanup deal with preload cleanup.
	preloadPath := filepath.Join(dlc.PreloadDir, dlctest.TestID2, dlctest.TestPackage)
	if err := os.MkdirAll(preloadPath, 0755); err != nil {
		s.Fatal("Failed to make preload directory: ", err)
	}
	if err := dlctest.CopyFileWithPermissions(filepath.Join(dlctest.TestDir, dlctest.TestID2, dlctest.TestPackage, dlctest.ImageFile),
		filepath.Join(preloadPath, dlctest.ImageFile), 0644); err != nil {
		s.Fatal("Failed to copy file: ", err)
	}
	if err := dlctest.ChownContentsToDlcservice(filepath.Join(dlc.PreloadDir, dlctest.TestID2)); err != nil {
		s.Fatal("Failed to chown: ", err)
	}

	// Test DLC is preloaded now, so install it without a Omaha URL.
	if err := dlc.Install(ctx, dlctest.TestID2, ""); err != nil {
		s.Fatal("Install failed: ", err)
	}
	if err := dlctest.DumpAndVerifyInstalledDLCs(ctx, s.OutDir(), "install_preload_allowed", dlctest.TestID2); err != nil {
		s.Fatal("DumpAndVerifyInstalledDLCs failed: ", err)
	}
}
