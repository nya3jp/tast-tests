// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http/httptest"
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
		Contacts:     []string{"kimjae@chromium.org", "ahassani@chromium.org", "chromeos-core-services@google.com"},
		SoftwareDeps: []string{"dlc"},
		Attr:         []string{"group:mainline"},
	})
}

func DLCService(ctx context.Context, s *testing.State) {
	const (
		dlcID1          = "test1-dlc"
		dlcID2          = "test2-dlc"
		dlcserviceJob   = "dlcservice"
		updateEngineJob = "update-engine"
		dlcCacheDir     = "/var/cache/dlc"
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

	runCmd := func(msg string, e expect, name string, args ...string) {
		cmd := testexec.CommandContext(ctx, name, args...)
		cmd.Stdout = f
		cmd.Stderr = f
		if err := cmd.Run(); err != nil && e {
			s.Fatal("Failed to ", msg, err)
		} else if err == nil && !e {
			s.Fatal("Should have failed to ", msg)
		}
	}

	install := func(dlcs []string, omahaURL string, e expect) {
		s.Log("Installing DLC(s): ", dlcs)
		runCmd("install", e, "sudo", "-u", "chronos", "dlcservice_util", "--install",
			"--dlc_ids="+strings.Join(dlcs, ":"), "--omaha_url="+omahaURL)
	}

	uninstall := func(dlcs string, e expect) {
		s.Log("Uninstalling DLC(s): ", dlcs)
		runCmd("uninstall", e, "sudo", "-u", "chronos", "dlcservice_util",
			"--uninstall", "--dlc_ids="+dlcs)
	}

	startServer := func(dlcIDs ...string) *httptest.Server {
		srv, err := updateserver.New(ctx, dlcIDs...)
		if err != nil {
			s.Fatal("Failed to start update server: ", err)
		}
		return srv
	}

	stopServer := func(srv *httptest.Server) {
		s.Log("Closing Server")
		srv.Close()
	}

	defer func() {
		// Removes the installed DLC module and unmounts all DLC images
		// mounted under /run/imageloader.
		if err := testexec.CommandContext(ctx, "imageloader", "--unmount_all").Run(testexec.DumpLogOnError); err != nil {
			s.Error("Failed to unmount all: ", err)
		}
		d, err := ioutil.ReadDir(dlcCacheDir)
		if err != nil {
			s.Error("Failed to open DLC cache directory for clean up: ", err)
		}
		for _, f := range d {
			if err := os.RemoveAll(filepath.Join(dlcCacheDir, f.Name())); err != nil {
				s.Error("Failed to clean up: ", err)
			}
		}
	}()

	s.Run(ctx, "Single DLC combination tests", func(context.Context, *testing.State) {
		srv := startServer(dlcID1)
		defer stopServer(srv)

		// Before performing any Install/Uninstall.
		dumpInstalledDLCModules("00_initial_state")

		// Install single DLC.
		install([]string{dlcID1}, srv.URL, success)
		dumpInstalledDLCModules("01_install_dlc")

		// Install already installed DLC.
		install([]string{dlcID1}, srv.URL, success)
		dumpInstalledDLCModules("02_install_already_installed")

		// Install duplicates of already installed DLC.
		install([]string{dlcID1, dlcID1}, srv.URL, failure)
		dumpInstalledDLCModules("03_install_already_installed_duplicate")

		// Uninstall single DLC.
		uninstall(dlcID1, success)
		dumpInstalledDLCModules("04_uninstall_dlc")

		// Uninstall already uninstalled DLC.
		uninstall(dlcID1, success)
		dumpInstalledDLCModules("05_uninstall_already_uninstalled_dlc")

		// Install duplicates of DLC atomically.
		install([]string{dlcID1, dlcID1}, srv.URL, failure)
		dumpInstalledDLCModules("06_atommically_install_duplicate")

		// Install unsupported DLC.
		install([]string{"bad-dlc"}, "http://???", failure)
		dumpInstalledDLCModules("07_install_bad_dlc")
	})

	s.Run(ctx, "Multi DLC combination tests", func(context.Context, *testing.State) {
		srv := startServer(dlcID1, dlcID2)
		defer stopServer(srv)

		// Install multiple DLC(s).
		install([]string{dlcID1, dlcID2}, srv.URL, success)
		dumpInstalledDLCModules("08_install_multiple_dlcs")

		// Install multiple DLC(s) already installed.
		install([]string{dlcID1, dlcID2}, srv.URL, success)
		dumpInstalledDLCModules("09_install_multiple_dlcs_already_installed")
	})
}
