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
	"chromiumos/tast/local/dbusutil"
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

	restartUpstartJob := func(ctx context.Context, s *testing.State, job, serviceName string) {
		// Restart job.
		s.Logf("Restarting %s job", job)
		if err := upstart.RestartJob(ctx, job); err != nil {
			s.Fatalf("Failed to restart %s: %v", job, err)
		}

		// Wait for service to be ready.
		if bus, err := dbusutil.SystemBus(); err != nil {
			s.Fatal("Failed to connect to the message bus: ", err)
		} else if err := dbusutil.WaitForService(ctx, bus, serviceName); err != nil {
			s.Fatal("Failed to wait for D-Bus service: ", err)
		}
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
	restartUpstartJob(ctx, s, updateEngineJob, updateEngineServiceName)

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
	// Initial cleanup.
	cleanup()
	// Deferred cleanup.
	defer cleanup()

	// Before performing any Install/Uninstall.
	dlc.DumpAndVerifyInstalledDLCs(ctx, s, "initial_state")

	s.Run(ctx, "Single DLC combination tests", func(ctx context.Context, s *testing.State) {
		func() {
			n, err := nebraska.Start(ctx)
			if err != nil {
				s.Fatal("Nebraska failed to start: ", err)
			}
			s.Log("Started Nebraska")
			defer n.Stop(s, "single-dlc")

			// Install DLC from Nebraska/Omaha.
			dlc.Install(ctx, s, dlcID1, n.URL)
			dlc.DumpAndVerifyInstalledDLCs(ctx, s, "install_from_url", dlcID1)
		}()

		// Install already installed DLC when Nebraska/Omaha is down with empty url.
		dlc.Install(ctx, s, dlcID1, "")
		dlc.DumpAndVerifyInstalledDLCs(ctx, s, "install_already_installed_no_url", dlcID1)

		// Uninstall DLC after installing.
		dlc.Uninstall(ctx, s, dlcID1)
		dlc.DumpAndVerifyInstalledDLCs(ctx, s, "uninstall_after_installing")

		// Uninstall already uninstalled DLC.
		dlc.Uninstall(ctx, s, dlcID1)
		dlc.DumpAndVerifyInstalledDLCs(ctx, s, "uninstall_already_uninstalled")
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
			dlc.Install(ctx, s, dlcID1, n.URL)
			dlc.DumpAndVerifyInstalledDLCs(ctx, s, "reboot_install_before_reboot", dlcID1)
		}()

		// Restart dlcservice to mimic a device reboot.
		restartUpstartJob(ctx, s, dlcserviceJob, dlcserviceServiceName)

		// Install single DLC after mimicking a reboot. Pass an empty url so
		// Nebraska/Omaha aren't hit.
		dlc.Install(ctx, s, dlcID1, "")
		dlc.DumpAndVerifyInstalledDLCs(ctx, s, "install_single_after_reboot", dlcID1)

		// Uninstall single DLC after mimicking a reboot.
		dlc.Uninstall(ctx, s, dlcID1)
		dlc.DumpAndVerifyInstalledDLCs(ctx, s, "uninstall_single_after_reboot")
	})
}
