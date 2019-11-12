// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"os"

	"chromiumos/tast/crash"
	localCrash "chromiumos/tast/local/crash"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	iwlwifiDir = "/sys/kernel/debug/iwlwifi"
	crashDir   = "/var/spool/crash"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     DevCoredump,
		Desc:     "Verify device coredumps are handled as expected",
		Contacts: []string{"mwiitala@google.com", "cros-monitoring-forensics@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		// TODO: Check for a dependency or attribute group I could use to check if device has intel WiFi
		SoftwareDeps: []string{"wifi"},
	})
}

func DevCoredump(ctx context.Context, s *testing.State) {
	// This test does not call SetUpCrashTest() because it is designed to test device coredump
	// handling on developer images. SetupCrashTest() causes the DUT to behave as if it were
	// running a base image and thus no .devcore files would be created if it were called.

	// Memorize existing crash files to distinguish new files from them.
	existingFiles, err := crash.GetCrashes(crashDir)
	if err != nil {
		s.Fatal("Failed to get existing files from crash directory: ", err)
	}

	// Verify that DUT has Intel WiFi.
	if _, err = os.Stat(iwlwifiDir); os.IsNotExist(err) {
		// TODO: Remove this check if Intel Wifi dependency exists to skip running test entirely.
		s.Log("iwlwifi directory does not exist on DUT, skipping test")
		return
	}

	s.Log("Triggering a devcoredump by restarting wifi firmware")

	// Use the find command to get the full path to the fw_restart file.
	path, err := testexec.CommandContext(ctx, "find", iwlwifiDir, "-name", "fw_restart").Output()
	if err != nil {
		s.Fatal("Failed to find fw_restart file: ", err)
	}

	// Trigger a wifi fw restart by echoing 1 into the fw_restart file.
	err = testexec.CommandContext(ctx, "sh", "-c", string("echo 1 > "+string(path))).Run()
	if err != nil {
		s.Fatal("Failed to trigger device coredump: ", err)
	}

	s.Log("Waiting for .devcore file to be added to crash directory")

	// Check that expected device coredump is copied to crash directory.
	devCoreFiles, err := localCrash.WaitForCrashFiles(ctx, []string{crashDir},
		existingFiles,
		[]string{"devcoredump_iwlwifi\\.[0-9]{8}\\.[0-9]{6}\\.[0-9]\\.devcore"})
	if err != nil {
		s.Fatal("Failed while polling crash directory: ", err)
	}
	if len(devCoreFiles) == 0 {
		s.Fatal("Failed to generate .devcore file")
	}
}
