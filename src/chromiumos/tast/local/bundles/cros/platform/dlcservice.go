// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"os"
	"path/filepath"
	"time"

	dlctest "chromiumos/tast/local/bundles/cros/platform/dlc"
	"chromiumos/tast/local/bundles/cros/platform/nebraska"
	"chromiumos/tast/local/dlc"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DLCService,
		Desc:         "Verifies that DLC D-Bus API (install, uninstall, purge, etc.) works",
		Contacts:     []string{"kimjae@chromium.org", "chromeos-core-services@google.com"},
		SoftwareDeps: []string{"dlc"},
		Attr:         []string{"group:mainline"},
		Timeout:      5 * time.Minute,
	})
}

func DLCService(ctx context.Context, s *testing.State) {
	// UpdateEngine related constants.
	const (
		updateEngineJob                      = "update-engine"
		updateEngineServiceName              = "org.chromium.UpdateEngine"
		updateEnginePowerwashSafePrefsSubDir = "update_engine/prefs"
		updateEnginePowerwashSafeDir         = "/mnt/stateful_partition/unencrypted/preserve"
	)

	// Check dlcservice is up and running.
	if err := upstart.EnsureJobRunning(ctx, dlc.JobName); err != nil {
		s.Fatalf("Failed to ensure %s running: %v", dlc.JobName, err)
	}

	// Delete rollback-version and rollback-happened pref which are
	// generated during Rollback and Enterprise Rollback.
	// rollback-version is written when update_engine Rollback D-Bus API is
	// called. The existence of rollback-version prevents update_engine to
	// apply payload whose version is the same as rollback-version.
	// rollback-happened is written when update_engine finished Enterprise
	// Rollback operation.
	for _, p := range []string{"rollback-version", "rollback-happened"} {
		prefsPath := filepath.Join(updateEnginePowerwashSafeDir, updateEnginePowerwashSafePrefsSubDir, p)
		if err := os.RemoveAll(prefsPath); err != nil {
			s.Fatal("Failed to clean up pref: ", err)
		}
	}

	// Restart update-engine to pick up the new prefs.
	s.Logf("Restarting %s job", updateEngineJob)
	upstart.RestartJobAndWaitForDbusService(ctx, updateEngineJob, updateEngineServiceName)

	// Initial cleanup to always start with no test DLC installations.
	if err := dlc.Cleanup(ctx, dlc.Info{ID: dlctest.TestID1, Package: dlctest.TestPackage}); err != nil {
		s.Fatal("Initial cleanup failed: ", err)
	}
	// Deferred cleanup to always end with no test DLC installations.
	defer func() {
		if err := dlc.Cleanup(ctx, dlc.Info{ID: dlctest.TestID1, Package: dlctest.TestPackage}); err != nil {
			s.Error("Ending cleanup failed: ", err)
		}
	}()

	install := func(ctx context.Context, s *testing.State, id, omahaURL string) {
		if err := dlc.Install(ctx, id, omahaURL); err != nil {
			s.Fatal("Install failed: ", err)
		}
	}

	purge := func(ctx context.Context, s *testing.State, id string) {
		if err := dlc.Purge(ctx, id); err != nil {
			s.Fatal("Purge failed: ", err)
		}
	}

	dump := func(ctx context.Context, s *testing.State, tag string, ids ...string) {
		if err := dlctest.DumpAndVerifyInstalledDLCs(ctx, s.OutDir(), tag, ids...); err != nil {
			s.Fatal("Dump failed: ", err)
		}
	}

	// Dump the list of installed DLCs before performing any operations.
	dump(ctx, s, "initial_state")

	s.Run(ctx, "DLC combination tests", func(ctx context.Context, s *testing.State) {
		func() {
			n, err := nebraska.Start(ctx)
			if err != nil {
				s.Fatal("Nebraska failed to start: ", err)
			}
			s.Log("Started Nebraska")
			defer n.Stop(s, "single-dlc")

			// Install single DLC.
			install(ctx, s, dlctest.TestID1, n.URL)
			dump(ctx, s, "install_single", dlctest.TestID1)

		}()

		// Install already installed DLC when Nebraska/Omaha is down with empty url.
		install(ctx, s, dlctest.TestID1, "")
		dump(ctx, s, "install_already_installed_no_url", dlctest.TestID1)

		// Purge DLC after installing.
		purge(ctx, s, dlctest.TestID1)
		dump(ctx, s, "purge_after_installing")

		// Purge already purged DLC.
		purge(ctx, s, dlctest.TestID1)
		dump(ctx, s, "purge_already_purged")
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

			// Install DLC.
			install(ctx, s, dlctest.TestID1, n.URL)
			dump(ctx, s, "reboot_install_before_reboot", dlctest.TestID1)
		}()

		// Restart dlcservice to mimic a device reboot.
		s.Logf("Restarting %s job", dlc.JobName)
		upstart.RestartJobAndWaitForDbusService(ctx, dlc.JobName, dlc.ServiceName)

		// Install DLC after mimicking a reboot. Pass an empty url so
		// Nebraska/Omaha aren't hit.
		install(ctx, s, dlctest.TestID1, "")
		dump(ctx, s, "install_after_reboot", dlctest.TestID1)

		// Purge DLC after mimicking a reboot.
		purge(ctx, s, dlctest.TestID1)
		dump(ctx, s, "purge_after_reboot")
	})
}
