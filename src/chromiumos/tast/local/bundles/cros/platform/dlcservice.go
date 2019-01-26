// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"time"

	"chromiumos/tast/local/bundles/cros/platform/nebraska"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DLCService,
		Desc:         "Verifies that dlcservice exits on idle and accepts D-Bus calls",
		Contacts:     []string{"xiaochu@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"dlc"},
		Data:         []string{payloadData},
	})
}

const payloadData = "dlc/dlcservice_test-dlc.payload"

func DLCService(ctx context.Context, s *testing.State) {
	const (
		dlcModuleID   = "test-dlc"
		dlcserviceJob = "dlcservice"
		nebraskaPort  = "2412"
	)

	dumpInstalledDLCModules := func(filename string) {
		s.Log("Asking dlcservice for installed DLC modules")
		cmd := testexec.CommandContext(ctx, "dlcservice_util", "--list")
		if out, err := cmd.Output(); err != nil {
			cmd.DumpLog(ctx)
			s.Fatal("Failed to get installed DLC modules: ", err)
		} else if err := ioutil.WriteFile(filepath.Join(s.OutDir(), filename), out, 0644); err != nil {
			s.Fatal("Failed to write output: ", err)
		}
	}

	cleanUp := func() error {
		// Removes the DLC module.
		cmd := testexec.CommandContext(ctx, "rm", "-rf", "/var/lib/dlc/"+dlcModuleID)
		if err := cmd.Run(); err != nil {
			s.Fatal("Failed to clean up: ", err)
			return err
		}
		cmd = testexec.CommandContext(ctx, "imageloader", "--unmount_all")
		if err := cmd.Run(); err != nil {
			s.Fatal("Failed to unmount all: ", err)
			return err
		}
		return nil
	}

	defer cleanUp()

	// Ensures nebraska server is up.
	if cmd, err := nebraska.EnsureNebraskaUp(ctx, dlcModuleID, s.DataPath(payloadData), nebraskaPort); err != nil {
		defer cmd.Wait()
		defer cmd.Kill()
	}

	// Restarts dlcservice.
	s.Logf("Restarting %s job", dlcserviceJob)
	if err := upstart.RestartJob(ctx, dlcserviceJob); err != nil {
		s.Fatalf("Failed to start %s: %v", dlcserviceJob, err)
	}
	// Checks dlcservice exits on idle.
	// dlcservice is a short-lived process.
	if err := upstart.WaitForJobStatus(ctx, dlcserviceJob, upstart.StopGoal, upstart.WaitingState, upstart.TolerateWrongGoal, time.Minute); err != nil {
		s.Fatalf("Job %s did not exit on idle: %v", dlcserviceJob, err)
	}

	// Calls dlcservice's GetInstalled D-Bus method, checks the return results.
	// dlcservice is activated on-demand via D-Bus method call.
	dumpInstalledDLCModules("installed_dlc_modules_0.txt")

	// Installs a DLC module.
	s.Logf("Install a DLC: %s", dlcModuleID)
	cmd := testexec.CommandContext(ctx, "sudo", "-u", "chronos", "dlcservice_util", "--install", "--dlc_ids="+dlcModuleID, "--omaha_url=http://127.0.0.1:"+nebraskaPort)
	if _, err := cmd.Output(); err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to install DLC modules: ", err)
	}

	dumpInstalledDLCModules("installed_dlc_modules_1.txt")

	// Uninstalls a DLC module.
	s.Logf("Uninstall a DLC: %s", dlcModuleID)
	cmd = testexec.CommandContext(ctx, "sudo", "-u", "chronos", "dlcservice_util", "--uninstall", "--dlc_ids="+dlcModuleID)
	if _, err := cmd.Output(); err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to uninstall DLC modules: ", err)
	}

	dumpInstalledDLCModules("installed_dlc_modules_2.txt")

	// Checks dlcservice exits on idle.
	if err := upstart.WaitForJobStatus(ctx, dlcserviceJob, upstart.StopGoal, upstart.WaitingState, upstart.TolerateWrongGoal, time.Minute); err != nil {
		s.Fatalf("Job %s did not exit on idle: %v", dlcserviceJob, err)
	}
}
