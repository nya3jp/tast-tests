// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"path/filepath"

	"chromiumos/tast/local/bundles/cros/platform/dlc"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DLCServicePreloading,
		Desc:         "Verifies that DLC preloading works",
		Contacts:     []string{"kimjae@chromium.org", "ahassani@chromium.org", "chromeos-core-services@google.com"},
		SoftwareDeps: []string{"dlc"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

func DLCServicePreloading(ctx context.Context, s *testing.State) {
	// Dlcservice related constants.
	const (
		dummyID               = "dummy-dlc"
		dummyPackage          = "package"
		dlcserviceJob         = "dlcservice"
		dlcserviceServiceName = "org.chromium.DlcService"
	)

	// Check dlcservice is up and running.
	if err := upstart.EnsureJobRunning(ctx, dlcserviceJob); err != nil {
		s.Fatalf("Failed to ensure %s running: %v", dlcserviceJob, err)
	}

	// Cleanup so dummy DLC gets unmounted to free up a loopback device.
	defer func() {
		dlc.RestartUpstartJob(ctx, dlcserviceJob, dlcserviceServiceName)
		path := filepath.Join("/run/imageloader", dummyID, dummyPackage)
		if err := testexec.CommandContext(ctx, "imageloader", "--unmount", "--mount_point="+path).Run(testexec.DumpLogOnError); err != nil {
			s.Error("Failed to unmount DLC: ", err)
		}
	}()

	// TODO(kimjae): Add logic in the future to convert already installed
	// dummy DLC into a preload by force. There is no need to do this right
	// now as this is the only test managing dummy DLC on a test DUT.
	convertInstalledToPreloaded := func(dlcID, dlcPackage string) {
		manifest, err := dlc.ReadImageloaderManifest(ctx, dlcID, dlcPackage)
		if err != nil {
			s.Fatal("Failed to ReadImageloaderManifest: ", err)
		}
		s.Logf("Need to convert installed DLC=%s Package=%s to a preloaded image with size=%v", dlcID, dlcPackage, manifest.Size)
	}

	dlcListOutputs, err := dlc.GetInstalled(ctx)
	if err != nil {
		s.Fatal("GetInstall failed: ", err)
	}
	for _, dlcListOutput := range dlcListOutputs {
		// If the dummy DLC is installed, do conversions to a preloaded DLC image.
		if dlcListOutput.ID == dummyID {
			convertInstalledToPreloaded(dummyID, dlcListOutput.Package)
		}
	}

	// Dummy DLC is preloaded on test images, installing without a Omaha URL should successfully install.
	if err := dlc.Install(ctx, dummyID, ""); err != nil {
		s.Fatal("Install failed: ", err)
	}
	if err := dlc.DumpAndVerifyInstalledDLCs(ctx, s.OutDir(), "install_preload_allowed", dummyID); err != nil {
		s.Fatal("DumpAndVerifyInstalledDLCs failed: ", err)
	}
}
