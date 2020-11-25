// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WifiCheckIntelFWDump,
		Desc:         "Verify device coredumps are not empty",
		Contacts:     []string{"arowa@chromium.org", "cros-telemetry@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"wifi", "intel_wifi_chip"},
		// TODO(crbug.com/1070299): Remove the below hard-coded devices
		// and the the software dependency "intel_wifi_chip" above.
		// Instead, use the Intel WiFi dependency when wifi hardware
		// dependencies are implemented.
		// NB: These exclusions are somewhat overly broad; some
		// (but not all) blooglet, ezkinil, and trembyle devices have
		// WiFi chips that would work for this test. However, for now
		// there is no better way to specify the exact hardware
		// parameters needed for this test. (See linked bug.)
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("blooglet", "dalboz", "ezkinil", "trembyle")),
	})
}

func WifiCheckIntelFWDump(ctx context.Context, s *testing.State) {
	const (
		iwlwifiDir      = "/sys/kernel/debug/iwlwifi"
		crashDir        = "/var/spool/crash"
		devCoreDumpName = `devcoredump_iwlwifi\.[0-9]{8}\.[0-9]{6}\.[0-9]*\.devcore`
		fwDbgCollect    = "fw_dbg_collect"
	)

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

	s.Log("Triggering a devcoredump")
	// Use the find command to get the full path to the tirgger type file.
	path, err := testexec.CommandContext(ctx, "find", iwlwifiDir, "-name", fwDbgCollect).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to find fw_restart file: ", err)
	}

	// Trigger a wifi fw dump.
	err = testexec.CommandContext(ctx, "sh", "-c", string("echo 1 > "+string(path))).Run(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to trigger device coredump: ", err)
	}

	s.Log("Waiting for .devcore file to be added to crash directory")

	// Check that expected device coredump is copied to crash directory.
	devCoreFiles, err := crash.WaitForCrashFiles(ctx, []string{crashDir},
		[]string{devCoreDumpName})
	if err != nil {
		s.Fatal("Failed while polling crash directory: ", err)
	}
	file := devCoreFiles[devCoreDumpName][0]

	var currentFileSize int64
	// Wait for the fw dump to be fully writen.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		fileInfo, err := os.Stat(file)
		if err != nil {
			return errors.New("failed to get the file information of the fw core dump")
		}
		if fileInfo.Size() == currentFileSize {
			return nil
		}
		currentFileSize = fileInfo.Size()
		return errors.New("failed the fw dump is still being writen")
	}, &testing.PollOptions{
		Timeout:  5 * time.Second,
		Interval: 500 * time.Millisecond,
	}); err != nil {
		s.Fatal("Failed to wait for fw dump to be fully writen, err: ", err)
	}

	// Check that the fw dump is not empty.
	// TODO(b:169152720): Confirm the expected size of a firmware dump
	// and replace the 1MB value.
	if currentFileSize <= 1000000 {
		s.Fatalf("Unexpected fw dump size; got %f MB, want > 1 MB", currentFileSize)
	}

	if err := crash.RemoveAllFiles(ctx, devCoreFiles); err != nil {
		s.Log("Couldn't clean up files: ", err)
	}
}
