// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/network/iface"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CheckIntelFWDump,
		Desc: "Verifies that device coredumps are not empty",
		Contacts: []string{
			"arowa@chromium.org",              // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
			"cros-telemetry@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"wifi"},
		// NB: The WifiIntel dependency tracks a manually maintained list of devices.
		// If the test is skipping when it should run or vice versa, check the hwdep
		// to see if your board is incorrectly included/excluded.
		HardwareDeps: hwdep.D(hwdep.WifiIntel()),
	})
}

func CheckIntelFWDump(ctx context.Context, s *testing.State) {
	const (
		iwlwifiDir       = "/sys/kernel/debug/iwlwifi"
		crashDir         = "/var/spool/crash"
		devCoreDumpName  = `devcoredump_iwlwifi\.\d{8}\.\d{6}\.\d+\.\d+\.devcore`
		fwDbgCollectPath = "/iwlmvm/fw_dbg_collect"
	)

	// Verify that DUT has Intel WiFi.
	if _, err := os.Stat(iwlwifiDir); os.IsNotExist(err) {
		s.Fatal("iwlwifi directory does not exist on DUT, skipping test")
	}

	// This test uses crash.DevImage because it is designed to test device
	// coredump handling on developer images.  Without it, no .devcore
	// files would be created.
	ctxForTearingDownCrashTest := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()
	if err := crash.SetUpCrashTest(ctx, crash.WithMockConsent(), crash.DevImage()); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest(ctxForTearingDownCrashTest)

	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create shill manager proxy: ", err)
	}

	ifaceName, err := shill.WifiInterface(ctx, m, 5*time.Second)
	if err != nil {
		s.Fatal("Failed to get the WiFi interface: ", err)
	}

	netIface := iface.NewInterface(ifaceName)

	devName, err := netIface.ParentDeviceName(ctx)
	if err != nil {
		s.Fatal("Failed to get the network parent device name: ", err)
	}

	fwDbgCollect := filepath.Join(filepath.Join(iwlwifiDir, devName), fwDbgCollectPath)
	if _, err := os.Stat(fwDbgCollect); err != nil {
		s.Fatalf("Failed to get the file information for %s: %v", fwDbgCollect, err)
	}

	s.Log("Triggering a devcoredump")
	if err := ioutil.WriteFile(fwDbgCollect, []byte("1"), 0); err != nil {
		s.Fatal("Failed to trigger a devcoredump: ", err)
	}

	// Check that expected device coredump is copied to crash directory.
	ctxForRemovingAllFiles := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()
	s.Log("Waiting for .devcore file to be added to crash directory")
	devCoreFiles, err := crash.WaitForCrashFiles(ctx, []string{crashDir},
		[]string{devCoreDumpName})
	if err != nil {
		s.Fatal("Failed while polling crash directory: ", err)
	}
	defer func(ctx context.Context) {
		if err := crash.RemoveAllFiles(ctx, devCoreFiles); err != nil {
			s.Log("Couldn't clean up files: ", err)
		}
	}(ctxForRemovingAllFiles)

	file := devCoreFiles[devCoreDumpName][0]

	var currentFileSize int64
	// Wait for the fw dump to be fully written.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		fileInfo, err := os.Stat(file)
		if err != nil {
			return errors.New("failed to get the file information of the fw core dump")
		}
		if fileInfo.Size() > 0 && fileInfo.Size() == currentFileSize {
			return nil
		}
		currentFileSize = fileInfo.Size()
		return errors.New("failed the fw dump is still being written")
	}, &testing.PollOptions{
		// Ran the test a 100 times on the boards octopus and zork. The file usually
		// takes less than 2ms to be fully written and 10s of milliseconds at worst.
		// Also, Intel observed ~300ms memory read time for iwlwifi memory dumps and
		// few seconds in extreme cases such as using virtualizations. see b/170974763
		// for more context.
		Timeout:  5 * time.Second,
		Interval: 100 * time.Millisecond,
	}); err != nil {
		s.Fatal("Failed to wait for fw dump to be fully written, err: ", err)
	}

	// Check that the fw dump is not empty.
	// TODO(b:169152720): Confirm the expected size of a firmware dump
	// and replace the 1MB value.
	if currentFileSize <= 1000000 {
		s.Fatalf("Unexpected fw dump size; got %f MB, want > 1 MB", float64(currentFileSize)/1000000)
	}
}
