// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/local/bundles/cros/platform/dlc"
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

	info := dlc.Info{ID: dlc.TestID1, Package: dlc.TestPackage}
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
	cachePath := filepath.Join(dlc.CacheDir, dlc.TestID1, dlc.TestPackage)
	for _, slot := range []string{dlc.SlotA, dlc.SlotB} {
		cacheSlotPath := filepath.Join(cachePath, slot)
		if err := os.MkdirAll(cacheSlotPath, dlc.DirPerm); err != nil {
			s.Fatal("Failed to make cache directory: ", err)
		}
		if err := dlc.CopyFile(filepath.Join(dlc.TestDir, dlc.TestID1, dlc.TestPackage, dlc.ImageFile),
			filepath.Join(cacheSlotPath, dlc.ImageFile), dlc.FilePerm); err != nil {
			s.Fatal("Failed to copy test to cache: ", err)
		}
	}
	if err := dlc.ChownContentsToDlcservice(filepath.Join(dlc.CacheDir, dlc.TestID1)); err != nil {
		s.Fatal("Failed to chown: ", err)
	}

	// Restart dlcservice.
	if err := dlc.RestartUpstartJobAndWait(ctx, dlc.JobName, dlc.ServiceName); err != nil {
		s.Fatal("Failed to restart dlcservice: ", err)
	}

	// Test DLC is deployed now, so install it without a Omaha URL.
	if err := dlc.Install(ctx, dlc.TestID1, ""); err != nil {
		s.Fatal("Install failed: ", err)
	}
	if err := dlc.DumpAndVerifyInstalledDLCs(ctx, s.OutDir(), "install_cros_deploy", dlc.TestID1); err != nil {
		s.Fatal("DumpAndVerifyInstalledDLCs failed: ", err)
	}
}
