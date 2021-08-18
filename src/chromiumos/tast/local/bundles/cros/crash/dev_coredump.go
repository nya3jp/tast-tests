// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"os"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	iwlwifiDir = "/sys/kernel/debug/iwlwifi"
	crashDir   = "/var/spool/crash"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DevCoredump,
		Desc:         "Verify device coredumps are handled as expected",
		Contacts:     []string{"briannorris@chromium.org", "cros-telemetry@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"wifi"},
		// NB: The WifiIntel dependency tracks a manually maintained list of devices.
		// If the test is skipping when it should run or vice versa, check the hwdep
		// to see if your board is incorrectly included/excluded.
		HardwareDeps: hwdep.D(hwdep.WifiIntel()),
	})
}

func DevCoredump(ctx context.Context, s *testing.State) {
	// Verify that DUT has Intel WiFi.
	if _, err := os.Stat(iwlwifiDir); os.IsNotExist(err) {
		s.Fatal("iwlwifi directory does not exist on DUT, skipping test")
	}

	// This test uses crash.DevImage because it is designed to test device
	// coredump handling on developer images.  Without it, no .devcore.gz
	// files would be created.
	if err := crash.SetUpCrashTest(ctx, crash.WithMockConsent(), crash.DevImage()); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest(ctx)

	s.Log("Triggering a devcoredump by restarting wifi firmware")

	// Use the find command to get the full path to the fw_restart file.
	path, err := testexec.CommandContext(ctx, "find", iwlwifiDir, "-name", "fw_restart").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to find fw_restart file: ", err)
	}

	// Trigger a wifi fw restart by echoing 1 into the fw_restart file.
	err = testexec.CommandContext(ctx, "sh", "-c", string("echo 1 > "+string(path))).Run(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to trigger device coredump: ", err)
	}

	s.Log("Waiting for .devcore.gz file to be added to crash directory")

	// Check that expected device coredump is copied to crash directory.
	devCoreFiles, err := crash.WaitForCrashFiles(ctx, []string{crashDir},
		[]string{`devcoredump_iwlwifi\.\d{8}\.\d{6}\.\d+\.\d+\.devcore.gz`})
	if err != nil {
		s.Fatal("Failed while polling crash directory: ", err)
	}
	if err := crash.RemoveAllFiles(ctx, devCoreFiles); err != nil {
		s.Log("Couldn't clean up files: ", err)
	}
}
