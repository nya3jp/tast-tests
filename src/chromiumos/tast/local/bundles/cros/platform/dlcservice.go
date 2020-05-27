// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/local/bundles/cros/platform/dlc"
	"chromiumos/tast/local/bundles/cros/platform/nebraska"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DLCService,
		Desc:         "Verifies that DLC D-Bus API (install, uninstall, etc.) works",
		Contacts:     []string{"kimjae@chromium.org", "ahassani@chromium.org", "chromeos-core-services@google.com"},
		SoftwareDeps: []string{"dlc"},
		Attr:         []string{"group:mainline"},
	})
}

func DLCService(ctx context.Context, s *testing.State) {
	const (
		dlcID1                  = "test1-dlc"
		testPackage             = "test-package"
		dlcserviceJob           = "dlcservice"
		dlcserviceServiceName   = "org.chromium.DlcService"
		updateEngineJob         = "update-engine"
		updateEngineServiceName = "org.chromium.UpdateEngine"
		dlcCacheDir             = "/var/cache/dlc"
		dlcLibDir               = "/var/lib/dlcservice/dlc"
		tmpDir                  = "/tmp"
	)

	// Check dlcservice is up and running.
	if err := upstart.EnsureJobRunning(ctx, dlcserviceJob); err != nil {
		s.Fatalf("Failed to ensure %s running: %v", dlcserviceJob, err)
	}

	// Delete rollback-version and rollback-happened pref which are
	// generated during Rollback and Enterprise Rollback.
	// rollback-version is written when update_engine Rollback D-Bus API is
	// called. The existence of rollback-version prevents update_engine to
	// apply payload whose version is the same as rollback-version.
	// rollback-happened is written when update_engine finished Enterprise
	// Rollback operation.
	for _, p := range []string{"rollback-version", "rollback-happened"} {
		prefsPath := filepath.Join("/mnt/stateful_partition/unencrypted/preserve/update_engine/prefs", p)
		if err := os.RemoveAll(prefsPath); err != nil {
			s.Fatal("Failed to clean up pref: ", err)
		}
	}

	// Restart update-engine to pick up the new prefs.
	s.Logf("Restarting %s job", updateEngineJob)
	dlc.RestartUpstartJob(ctx, updateEngineJob, updateEngineServiceName)

	cleanup := func() {
		// Removes the installed DLC module and unmounts all test DLC images mounted under /run/imageloader.
		ids := []string{dlcID1}
		for _, id := range ids {
			path := filepath.Join("/run/imageloader", id, testPackage)
			if err := testexec.CommandContext(ctx, "imageloader", "--unmount", "--mount_point="+path).Run(testexec.DumpLogOnError); err != nil {
				s.Errorf("Failed to unmount DLC (%s): %v", id, err)
			}
			for _, dir := range []string{dlcCacheDir, dlcLibDir} {
				if err := os.RemoveAll(filepath.Join(dir, id)); err != nil {
					s.Error("Failed to clean up: ", err)
				}
			}
		}
	}
	// Initial cleanup to always start with no test DLC installations.
	cleanup()
	// Deferred cleanup to always end with no test DLC installations.
	defer cleanup()

	install := func(ctx context.Context, s *testing.State, id, omahaURL string) {
		if err := dlc.Install(ctx, id, omahaURL); err != nil {
			s.Fatal("Install failed: ", err)
		}
	}

	uninstall := func(ctx context.Context, s *testing.State, id string) {
		if err := dlc.Uninstall(ctx, id); err != nil {
			s.Fatal("Uninstall failed: ", err)
		}
	}

	dump := func(ctx context.Context, s *testing.State, tag string, ids ...string) {
		if err := dlc.DumpAndVerifyInstalledDLCs(ctx, s.OutDir(), tag, ids...); err != nil {
			s.Fatal("Dump failed: ", err)
		}
	}

	// Dump the list of installed DLCs before performing any operations.
	dump(ctx, s, "initial_state")

	s.Run(ctx, "Single DLC combination tests", func(ctx context.Context, s *testing.State) {
		func() {
			n, err := nebraska.Start(ctx)
			if err != nil {
				s.Fatal("Nebraska failed to start: ", err)
			}
			s.Log("Started Nebraska")
			defer n.Stop(s, "single-dlc")

			// Install single DLC.
			install(ctx, s, dlcID1, n.URL)
			dump(ctx, s, "install_single", dlcID1)

		}()

		// Install already installed DLC when Nebraska/Omaha is down with empty url.
		install(ctx, s, dlcID1, "")
		dump(ctx, s, "install_already_installed_no_url", dlcID1)

		// Uninstall single DLC.
		uninstall(ctx, s, dlcID1)
		dump(ctx, s, "uninstall_dlc")

		// Uninstall already uninstalled DLC.
		uninstall(ctx, s, dlcID1)
		dump(ctx, s, "uninstall_already_uninstalled")
	})

	s.Run(ctx, "Mimic device reboot tests", func(ctx context.Context, s *testing.State) {
		// Stop nebraska after the first install to validate that the following
		// install after the dlcservice restart will perform a quick install from
		// DLC cache.
		func() {
			n, err := nebraska.Start(ctx)
			if err != nil {
				s.Fatal("Nebraska failed to start: ", err)
			}
			s.Log("Started Nebraska")
			defer n.Stop(s, "reboot-mimic-dlc")

			// Install single DLC.
			install(ctx, s, dlcID1, n.URL)
			dump(ctx, s, "reboot_install_before_reboot", dlcID1)
		}()

		// Restart dlcservice to mimic a device reboot.
		s.Logf("Restarting %s job", dlcserviceJob)
		dlc.RestartUpstartJob(ctx, dlcserviceJob, dlcserviceServiceName)

		// Install single DLC after mimicking a reboot. Pass an empty url so
		// Nebraska/Omaha aren't hit.
		install(ctx, s, dlcID1, "")
		dump(ctx, s, "install_single_after_reboot", dlcID1)

		// Uninstall single DLC after mimicking a reboot.
		uninstall(ctx, s, dlcID1)
		dump(ctx, s, "uninstall_single_after_reboot")
	})
}
