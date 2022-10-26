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
		Func:         DLCServiceCrosDeploy,
		Desc:         "Verifies that DLC cros deploying works by mimicking the cros deploy flow",
		Contacts:     []string{"kimjae@chromium.org", "chromeos-core-services@google.com"},
		SoftwareDeps: []string{"dlc"},
		Attr:         []string{"group:mainline"},
	})
}

func DLCServiceCrosDeploy(ctx context.Context, s *testing.State) {
	// Check dlcservice is up and running.
	if err := upstart.EnsureJobRunning(ctx, dlc.JobName); err != nil {
		s.Fatalf("Failed to ensure %s running: %v", dlc.JobName, err)
	}

	info := dlc.Info{ID: dlctest.TestID1, Package: dlctest.TestPackage}
	if err := dlc.Cleanup(ctx, info); err != nil {
		s.Fatal("Initial cleanup failed: ", err)
	}
	// Deferred cleanup to always end with no test DLC installations.
	defer func() {
		if err := dlc.Cleanup(ctx, info); err != nil {
			s.Error("Ending cleanup failed: ", err)
		}
	}()

	// Copy test DLC into dlcservice cache location. Let cleanup handle cache dir cleanup.
	cachePath := filepath.Join(dlc.CacheDir, dlctest.TestID1, dlctest.TestPackage)
	for _, slot := range []string{dlctest.SlotA, dlctest.SlotB} {
		cacheSlotPath := filepath.Join(cachePath, slot)
		if err := os.MkdirAll(cacheSlotPath, dlctest.DirPerm); err != nil {
			s.Fatal("Failed to make cache directory: ", err)
		}
		if err := dlctest.CopyFileWithPermissions(filepath.Join(dlctest.TestDir, dlctest.TestID1, dlctest.TestPackage, dlctest.ImageFile),
			filepath.Join(cacheSlotPath, dlctest.ImageFile), dlctest.FilePerm); err != nil {
			s.Fatal("Failed to copy test to cache: ", err)
		}
	}
	if err := dlctest.ChownContentsToDlcservice(filepath.Join(dlc.CacheDir, dlctest.TestID1)); err != nil {
		s.Fatal("Failed to chown: ", err)
	}

	// Restart dlcservice.
	if err := upstart.RestartJobAndWaitForDbusService(ctx, dlc.JobName, dlc.ServiceName); err != nil {
		s.Fatal("Failed to restart dlcservice: ", err)
	}

	// Test DLC is deployed now, so install it without a Omaha URL.
	if err := dlc.Install(ctx, dlctest.TestID1, ""); err != nil {
		s.Fatal("Install failed: ", err)
	}
	if err := dlctest.DumpAndVerifyInstalledDLCs(ctx, s.OutDir(), "install_cros_deploy", dlctest.TestID1); err != nil {
		s.Fatal("DumpAndVerifyInstalledDLCs failed: ", err)
	}
}
