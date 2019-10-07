// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
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
		dlcModuleID     = "test1-dlc"
		dlcserviceJob   = "dlcservice"
		updateEngineJob = "update-engine"
	)

	type expect bool
	const (
		success expect = true
		failure expect = false
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

	// Create the log that will hold all dumps and logs.
	f, err := os.Create(filepath.Join(s.OutDir(), "completelog.txt"))
	if err != nil {
		s.Fatal("Failed to create file: ", err)
	}
	defer f.Close()

	// dumpInstalledDLCModules calls dlcservice's GetInstalled D-Bus method
	// via dlcservice_util command.
	dumpInstalledDLCModules := func(tag string) {
		s.Log("Asking dlcservice for installed DLC modules")
		if _, err := fmt.Fprintf(f, "[%s]:\n", tag); err != nil {
			s.Fatal("Failed to write tag to file: ", err)
		}
		if err := f.Sync(); err != nil {
			s.Fatal("Failed to sync (flush) to file: ", err)
		}
		cmd := testexec.CommandContext(ctx, "dlcservice_util", "--list")
		cmd.Stdout = f
		cmd.Stderr = f
		if err := cmd.Run(); err != nil {
			s.Fatal("Failed to get installed DLC modules: ", err)
		}
	}

	install := func(dlcs []string, omahaURL string, e expect) {
		s.Log("Installing DLC(s): ", dlcs)
		cmd := testexec.CommandContext(ctx, "sudo", "-u", "chronos", "dlcservice_util",
			"--install", "--dlc_ids="+strings.Join(dlcs, ":"), "--omaha_url="+omahaURL)
		cmd.Stdout = f
		cmd.Stderr = f
		if err := cmd.Run(); err != nil && e {
			s.Fatal("Failed to install DLC modules: ", err)
		} else if err == nil && !e {
			s.Fatal("Should have failed to install DLC modules: ", err)
		}
	}

	uninstall := func(dlcs string) {
		s.Log("Uninstalling DLC(s): ", dlcs)
		cmd := testexec.CommandContext(ctx, "sudo", "-u", "chronos", "dlcservice_util",
			"--uninstall", "--dlc_ids="+dlcModuleID)
		cmd.Stdout = f
		cmd.Stderr = f
		if err := cmd.Run(); err != nil {
			s.Fatal("Failed to uninstall DLC modules: ", err)
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
	dumpInstalledDLCModules("00_initial_state")

	// Install single DLC.
	install([]string{dlcModuleID}, srv.URL, success)
	dumpInstalledDLCModules("01_install_dlc")

	// Install already installed DLC.
	install([]string{dlcModuleID}, srv.URL, success)
	dumpInstalledDLCModules("02_install_already_installed")

	// Install duplicates of already installed DLC.
	install([]string{dlcModuleID, dlcModuleID}, srv.URL, failure)
	dumpInstalledDLCModules("03_install_already_installed_duplicate")

	// Uninstall single DLC.
	uninstall(dlcModuleID)
	dumpInstalledDLCModules("04_uninstall_dlc")

	// Install duplicates of DLC atomically.
	install([]string{dlcModuleID, dlcModuleID}, srv.URL, failure)
	dumpInstalledDLCModules("05_atommically_install_duplicate")

	install([]string{"bad-dlc"}, "http://???", failure)
	dumpInstalledDLCModules("07_install_bad_dlc")
}
