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
	"chromiumos/tast/testutil"
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
	completeLog := filepath.Join(s.OutDir(), "completelog.txt")

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
	f, err := os.Create(completeLog)
	if err != nil {
		s.Fatal("Failed to create file: ", err)
	}
	defer f.Close()

	// Append to the log that will hold all dumps and logs.
	appendToLog := func(log, title string, o []byte) {
		data := fmt.Sprintf("\n[%s]:\n%s", title, o)
		if err := testutil.AppendToFile(log, data); err != nil {
			s.Error("Failed to append to file: ", err)
		}
	}

	// dumpInstalledDLCModules calls dlcservice's GetInstalled D-Bus method
	// via dlcservice_util command.
	dumpInstalledDLCModules := func(title string) {
		s.Log("Asking dlcservice for installed DLC modules")
		if o, err := testexec.CommandContext(ctx, "dlcservice_util", "--list").CombinedOutput(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to get installed DLC modules: ", err)
		} else {
			appendToLog(completeLog, title, o)
		}
	}

	install := func(dlcs, omahaURL string) {
		const title = "Installing DLC(s)"
		s.Log(title+": ", strings.Replace(dlcs, ":", ", ", -1))
		if o, err := testexec.CommandContext(ctx, "sudo", "-u", "chronos", "dlcservice_util",
			"--install", "--dlc_ids="+dlcs, "--omaha_url="+omahaURL).CombinedOutput(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to install DLC modules: ", err)
		} else {
			appendToLog(completeLog, title, o)
		}
	}

	installBad := func(dlcs, omahaURL string) {
		const title = "Installing Bad DLC(s)"
		s.Log(title+": ", strings.Replace(dlcs, ":", ", ", -1))
		if o, err := testexec.CommandContext(ctx, "sudo", "-u", "chronos", "dlcservice_util",
			"--install", "--dlc_ids="+dlcs, "--omaha_url="+omahaURL).CombinedOutput(testexec.DumpLogOnError); err != nil {
			appendToLog(completeLog, title, o)
		} else {
			s.Fatal("DLC modules should not have installed: ", err)
		}
	}

	uninstall := func(dlcs string) {
		const title = "Uninstalling DLC(s)"
		s.Log(title+": ", strings.Replace(dlcs, ":", ", ", -1))
		if o, err := testexec.CommandContext(ctx, "sudo", "-u", "chronos", "dlcservice_util",
			"--uninstall", "--dlc_ids="+dlcModuleID).CombinedOutput(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to uninstall DLC modules: ", err)
		} else {
			appendToLog(completeLog, title, o)
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
	install(dlcModuleID, srv.URL)
	dumpInstalledDLCModules("01_install_dlc")

	// Install already installed DLC.
	install(dlcModuleID, srv.URL)
	dumpInstalledDLCModules("02_install_already_installed")

	// Install duplicates of already installed DLC.
	install(dlcModuleID+":"+dlcModuleID, srv.URL)
	dumpInstalledDLCModules("03_install_already_installed_duplicate")

	// Uninstall single DLC.
	uninstall(dlcModuleID)
	dumpInstalledDLCModules("04_uninstall_dlc")

	// Install duplicates of DLC atomically.
	install(dlcModuleID+":"+dlcModuleID, srv.URL)
	dumpInstalledDLCModules("05_atommically_install_duplicate")

	// Uninstall single DLC.
	uninstall(dlcModuleID)
	dumpInstalledDLCModules("06_uninstall_dlc")

	installBad("bad-dlc", "http://???")
	dumpInstalledDLCModules("07_install_bad_dlc")
}
