// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/godbus/dbus"

	"chromiumos/tast/local/bundles/cros/platform/updateserver"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DLCService,
		Desc:         "Verifies that DLC D-Bus API (install, uninstall, etc.) works",
		Contacts:     []string{"xiaochu@chromium.org"},
		SoftwareDeps: []string{"dlc"},
	})
}

func DLCService(ctx context.Context, s *testing.State) {
	const (
		dlcModuleID     = "test-dlc"
		dlcserviceJob   = "dlcservice"
		updateEngineJob = "update-engine"
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
	if err := upstart.RestartJob(ctx, updateEngineJob); err != nil {
		s.Fatalf("Failed to restart %s: %v", updateEngineJob, err)
	}

	// Wait for update-engine to be ready.
	if bus, err := dbus.SystemBus(); err != nil {
		s.Fatal("Failed to connect to the message bus: ", err)
	} else if err := dbusutil.WaitForService(ctx, bus, "org.chromium.UpdateEngine"); err != nil {
		s.Fatal("Failed to wait for D-Bus service: ", err)
	}

	// dumpInstalledDLCModules calls dlcservice's GetInstalled D-Bus method
	// via dlcservice_util command.
	dumpInstalledDLCModules := func() {
		s.Log("Asking dlcservice for installed DLC modules")
		cmd := testexec.CommandContext(ctx, "dlcservice_util", "--list")
		if o, err := cmd.CombinedOutput(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to get installed DLC modules: ", err)
		} else {
			s.Logf("Currently installed: %q", o)
		}
	}

	install := func(dlcs, omahaUrl string) {
		s.Log("Installing DLC(s): ", strings.Replace(dlcs, ":", ", ", -1))
		cmd := testexec.CommandContext(ctx, "sudo", "-u", "chronos", "dlcservice_util",
			"--install", "--dlc_ids="+dlcs, "--omaha_url="+omahaUrl)
		if o, err := cmd.CombinedOutput(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to install DLC modules: ", err)
		} else {
			s.Logf("Installation result: %q", o)
		}
	}

	installBad := func(dlcs, omahaUrl string) {
		s.Log("Installing Bad DLC(s): ", strings.Replace(dlcs, ":", ", ", -1))
		cmd := testexec.CommandContext(ctx, "sudo", "-u", "chronos", "dlcservice_util",
			"--install", "--dlc_ids="+dlcs, "--omaha_url="+omahaUrl)
		if err := cmd.Run(testexec.DumpLogOnError); err == nil {
			s.Fatal("DLC modules should not have installed: ", err)
		}
	}

	uninstall := func(dlcs string) {
		s.Log("Uninstalling DLC(s): ", strings.Replace(dlcs, ":", ", ", -1))
		cmd := testexec.CommandContext(ctx, "sudo", "-u", "chronos", "dlcservice_util",
			"--uninstall", "--dlc_ids="+dlcModuleID)
		if o, err := cmd.CombinedOutput(); err != nil {
			defer cmd.DumpLog(ctx)
			s.Fatal("Failed to uninstall DLC modules: ", err)
		} else {
			s.Logf("Uninstallation result: %q", o)
		}
	}

	defer func() {
		// Removes the installed DLC module and unmounts all DLC images
		// mounted under /run/imageloader.
		if err := testexec.CommandContext(ctx, "imageloader", "--unmount_all").Run(testexec.DumpLogOnError); err != nil {
			s.Error("Failed to unmount all: ", err)
		}
		if err := os.RemoveAll("/var/cache/dlc/" + dlcModuleID); err != nil {
			s.Error("Failed to clean up: ", err)
		}
	}()

	srv, err := updateserver.New(ctx, dlcModuleID)
	if err != nil {
		s.Fatal("Failed to start update server: ", err)
	}
	defer srv.Close()

	// Before performing any Install/Uninstall.
	dumpInstalledDLCModules()

	// Install single DLC.
	install(dlcModuleID, srv.URL)
	dumpInstalledDLCModules()

	// Install already installed DLC.
	install(dlcModuleID, srv.URL)
	dumpInstalledDLCModules()

	// Install duplicates of already installed DLC.
	install(dlcModuleID+":"+dlcModuleID, srv.URL)
	dumpInstalledDLCModules()

	// Uninstall single DLC.
	uninstall(dlcModuleID)
	dumpInstalledDLCModules()

	// Install duplicates of DLC atomically.
	install(dlcModuleID+":"+dlcModuleID, srv.URL)
	dumpInstalledDLCModules()

	// Uninstall single DLC.
	uninstall(dlcModuleID)
	dumpInstalledDLCModules()

	installBad("bad-dlc", "http://???")
	dumpInstalledDLCModules()
}
