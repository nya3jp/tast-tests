// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"os"

	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/testexec"
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
		// TODO(crbug.com/1070299): Remove the below hard-coded devices
		// and use Intel WiFi dependency when wifi hardware
		// dependencies are implemented.
		// NB: These exclusions are somewhat overly broad; some
		// (but not all) blooglet, ezkinil, and trembyle devices have
		// WiFi chips that would work for this test. However, for now
		// there is no better way to specify the exact hardware
		// parameters needed for this test. (See linked bug.)
		HardwareDeps: hwdep.D(hwdep.SkipOnPlatform("bob",
			"elm",
			"grunt",
			"hana",
			"jacuzzi",
			"kevin",
			"kukui",
			"oak",
			"scarlet",
			"veyron_fievel",
			"veyron_mickey",
			"veyron_tiger",
		), hwdep.SkipOnModel("blooglet", "ezkinil", "trembyle")),
	})
}

func DevCoredump(ctx context.Context, s *testing.State) {
	// Verify that DUT has Intel WiFi.
	if _, err := os.Stat(iwlwifiDir); os.IsNotExist(err) {
		s.Fatal("iwlwifi directory does not exist on DUT, skipping test")
	}

	// This test uses crash.DevImage because it is designed to test device
	// coredump handling on developer images.  Without it, no .devcore
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

	s.Log("Waiting for .devcore file to be added to crash directory")

	// Check that expected device coredump is copied to crash directory.
	devCoreFiles, err := crash.WaitForCrashFiles(ctx, []string{crashDir},
		[]string{`devcoredump_iwlwifi\.[0-9]{8}\.[0-9]{6}\.[0-9]*\.devcore`})
	if err != nil {
		s.Fatal("Failed while polling crash directory: ", err)
	}
	if err := crash.RemoveAllFiles(ctx, devCoreFiles); err != nil {
		s.Log("Couldn't clean up files: ", err)
	}
}
