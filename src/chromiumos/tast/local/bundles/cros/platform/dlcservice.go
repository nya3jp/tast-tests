// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/bundles/cros/platform/updateserver"
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
	})
}

func DLCService(ctx context.Context, s *testing.State) {
	const (
		dlcModuleID   = "test-dlc"
		dlcserviceJob = "dlcservice"
	)

	// dumpInstalledDLCModules calls dlcservice's GetInstalled D-Bus method and saves the returned results to filename within the output directory.
	dumpInstalledDLCModules := func(filename string) {
		s.Log("Asking dlcservice for installed DLC modules")
		cmd := testexec.CommandContext(ctx, "dlcservice_util", "--list")
		if out, err := cmd.Output(); err != nil {
			defer cmd.DumpLog(ctx)
			s.Fatal("Failed to get installed DLC modules: ", err)
		} else if err := ioutil.WriteFile(filepath.Join(s.OutDir(), filename), out, 0644); err != nil {
			s.Fatal("Failed to write output: ", err)
		}
	}

	defer func() {
		// Removes the installed DLC module and unmounts all DLC images mounted under /run/imageloader.
		cmd := testexec.CommandContext(ctx, "imageloader", "--unmount_all")
		if err := cmd.Run(); err != nil {
			s.Error("Failed to unmount all: ", err)
		}
		if err := os.RemoveAll("/var/lib/dlc/" + dlcModuleID); err != nil {
			s.Error("Failed to clean up: ", err)
		}
	}()

	_, server := updateserver.NewServer(ctx, dlcModuleID)
	defer server.Close()

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
	cmd := testexec.CommandContext(ctx, "sudo", "-u", "chronos", "dlcservice_util", "--install", "--dlc_ids="+dlcModuleID, "--omaha_url="+server.URL)
	if output, err := cmd.CombinedOutput(); err != nil {
		defer cmd.DumpLog(ctx)
		s.Fatal("Failed to install DLC modules: ", err)
	} else {
		s.Logf("Installation result: %q", output)
	}

	dumpInstalledDLCModules("modules_after_install.txt")

	s.Log("Uninstalling DLC ", dlcModuleID)
	cmd = testexec.CommandContext(ctx, "sudo", "-u", "chronos", "dlcservice_util", "--uninstall", "--dlc_ids="+dlcModuleID)
	if _, err := cmd.Output(); err != nil {
		defer cmd.DumpLog(ctx)
		s.Fatal("Failed to uninstall DLC modules: ", err)
	}

	dumpInstalledDLCModules("modules_after_uninstall.txt")

	// Checks dlcservice exits on idle.
	if err := upstart.WaitForJobStatus(ctx, dlcserviceJob, upstart.StopGoal, upstart.WaitingState, upstart.TolerateWrongGoal, time.Minute); err != nil {
		s.Fatalf("Job %s did not exit on idle: %v", dlcserviceJob, err)
	}
}
