// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/bundles/cros/platform/updateserver"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DLCService,
		Desc:         "Verifies that DLC D-Bus API (install, uninstall, etc.) works",
		Contacts:     []string{"xiaochu@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"dlc"},
		// update_engine refuses to install updates while at the OOBE screen,
		// so ensure that a user is logged in when this test runs.
		Pre: chrome.LoggedIn(),
	})
}

func DLCService(ctx context.Context, s *testing.State) {
	const (
		dlcModuleID   = "test-dlc"
		dlcserviceJob = "dlcservice"
	)

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
		if err := os.RemoveAll("/var/lib/dlc/" + dlcModuleID); err != nil {
			s.Error("Failed to clean up: ", err)
		}
	}()

	srv, err := updateserver.New(ctx, dlcModuleID)
	if err != nil {
		s.Fatal("Failed to start update server: ", err)
	}
	defer srv.Close()

	s.Logf("Restarting %s job", dlcserviceJob)
	if err := upstart.RestartJob(ctx, dlcserviceJob); err != nil {
		s.Fatalf("Failed to restart %s: %v", dlcserviceJob, err)
	}
	// Checks dlcservice exits on idle (dlcservice is a short-lived process).
	if err := upstart.WaitForJobStatus(ctx, dlcserviceJob, upstart.StopGoal, upstart.WaitingState, upstart.TolerateWrongGoal, time.Minute); err != nil {
		s.Fatalf("Job %s did not exit on idle: %v", dlcserviceJob, err)
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

	// Checks dlcservice exits on idle.
	if err := upstart.WaitForJobStatus(ctx, dlcserviceJob, upstart.StopGoal, upstart.WaitingState, upstart.TolerateWrongGoal, time.Minute); err != nil {
		s.Fatalf("Job %s did not exit on idle: %v", dlcserviceJob, err)
	}
}
