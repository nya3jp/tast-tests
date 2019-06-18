// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"os"
	"path/filepath"

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

	// Delete rollback-version and rollback-happened pref which are
	// generated during Rollback and Enterprise Rollback.
	// rollback-version is written when update_engine Rollback D-Bus API is
	// called. The existence of rollback-version prevents update_engine to
	// apply payload whose version is the same as rollback-version.
	// rollback-happened is written when update_engine finished Enterprise
	// Rollback operation.
	for _, p := range []string{"rollback-version", "rollback-happened"} {
		if err := os.RemoveAll(filepath.Join("/mnt/stateful_partition/unencrypted/preserve/update_engine/prefs", p)); err != nil {
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

	// dumpInstalledDLCModules calls dlcservice's GetInstalled D-Bus method via dlcservice_util command and saves the returned results to filename within the output directory.
	dumpInstalledDLCModules := func(filename string) {
		s.Log("Asking dlcservice for installed DLC modules")
		f, err := os.Create(filepath.Join(s.OutDir(), filename))
		if err != nil {
			s.Fatal("Failed to create file: ", err)
		}
		defer f.Close()
		cmd := testexec.CommandContext(ctx, "dlcservice_util", "--list")
		cmd.Stdout = f
		if err := cmd.Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to get installed DLC modules: ", err)
		}
	}

	defer func() {
		// Removes the installed DLC module and unmounts all DLC images mounted under /run/imageloader.
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

	if err := upstart.EnsureJobRunning(ctx, dlcserviceJob); err != nil {
		s.Fatalf("Failed to ensure %s running: %v", dlcserviceJob, err)
	}

	dumpInstalledDLCModules("modules_before_install.txt")

	s.Log("Installing DLC ", dlcModuleID)
	cmd := testexec.CommandContext(ctx, "sudo", "-u", "chronos", "dlcservice_util", "--install", "--dlc_ids="+dlcModuleID, "--omaha_url="+srv.URL)
	if output, err := cmd.CombinedOutput(); err != nil {
		defer cmd.DumpLog(ctx)
		s.Fatal("Failed to install DLC modules: ", err)
	} else {
		s.Logf("Installation result: %q", output)
	}

	dumpInstalledDLCModules("modules_after_install.txt")

	s.Log("Uninstalling DLC ", dlcModuleID)
	if err := testexec.CommandContext(ctx, "sudo", "-u", "chronos", "dlcservice_util", "--uninstall", "--dlc_ids="+dlcModuleID).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to uninstall DLC modules: ", err)
	}

	dumpInstalledDLCModules("modules_after_uninstall.txt")
}
