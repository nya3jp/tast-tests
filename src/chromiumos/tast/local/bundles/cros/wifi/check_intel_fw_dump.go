// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wifi/intelfwextractor"
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
		Attr: []string{"group:mainline"},
		// TODO(b:169152720), Remove "no_kernel_upstream" to enable the test to run on
		// boards with upstream kernel when upstream iwlwifi is able to produce valid
		// fw dumps.
		SoftwareDeps: []string{"wifi", "no_kernel_upstream"},
		// NB: The WifiIntel dependency tracks a manually maintained list of devices.
		// If the test is skipping when it should run or vice versa, check the hwdep
		// to see if your board is incorrectly included/excluded.
		HardwareDeps: hwdep.D(hwdep.WifiIntel()),
	})
}

func CheckIntelFWDump(ctx context.Context, s *testing.State) {
	const (
		iwlwifiDir       = "/sys/kernel/debug/iwlwifi"
		devCoreDumpName  = `devcoredump_iwlwifi\.\d{8}\.\d{6}\.\d+\.\d+\.devcore.gz`
		metaDumpName     = `devcoredump_iwlwifi\.\d{8}\.\d{6}\.\d+\.\d+\.meta`
		logDumpName      = `devcoredump_iwlwifi\.\d{8}\.\d{6}\.\d+\.\d+\.log`
		fwDbgCollectPath = "/iwlmvm/fw_dbg_collect"
		kernelDevCDDir   = "/sys/class/devcoredump"
		kernelDevCDName  = `devcd\d+`
	)

	// Verify that DUT has Intel WiFi.
	if _, err := os.Stat(iwlwifiDir); os.IsNotExist(err) {
		s.Fatal("iwlwifi directory does not exist on DUT, skipping test")
	}

	files, err := os.ReadDir(kernelDevCDDir)
	if err != nil {
		// the /sys/class/devcoredump is unregistered due to unexpected kernel errors
		// Intel FW dumping requires the devcoredump kernel module.
		s.Fatal("devcoredump directory does not exist on DUT, indicating kernel errors. Failing the test")
	}

	re := regexp.MustCompile(kernelDevCDName)
	// The existence of these files indicates that devcoredump was invoked shortly before which are not cleaned up
	// devcoredump is invoked by crash_reporter during crash collection and testing, by which
	// a file filter is created to ignore Intel WiFi FW dumps. See b/228462848#comment7
	for _, file := range files {
		if re.MatchString(file.Name()) {
			s.Logf("Found existing core dump %s, waiting up to 30 seconds for crash_reporter to finish", file.Name())
			fullPath := filepath.Join(kernelDevCDDir, file.Name(), "data")
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				if _, err := os.Stat(fullPath); err != nil {
					testing.ContextLogf(ctx, "File %s has already been cleaned up, exit waiting", fullPath)
					return nil
				}
				return errors.New("Waiting crash_reporter to finish")
			}, &testing.PollOptions{Timeout: 30 * time.Second, Interval: time.Second}); err != nil {
				// 30s deadline exceeded, this is expected for most of cases
				s.Logf("File %s is still there, proceeding the cleaning up", fullPath)
			} else {
				// poll ends without errors, which means the devcoredump data directory was
				// unregistered during the waiting, skipping writing to it.
				continue
			}

			s.Log("Cleaning up the footprints left by crash_reporter")
			if err := os.WriteFile(fullPath, []byte("0"), 0); err != nil {
				// it's okay to fail to write devcoredump date file
				s.Logf("Didn't write devcoredump data file %s: %s. It's expected for a rare case "+
					"that crash_reporter unregistered the file just before the writing. "+
					"Continue testing", fullPath, err)
			}
		}
	}

	// This test uses crash.DevImage because it is designed to test device
	// coredump handling on developer images.  Without it, no .devcore.gz
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
	if err := os.WriteFile(fwDbgCollect, []byte("1"), 0); err != nil {
		s.Fatal("Failed to trigger a devcoredump: ", err)
	}

	crashDirs, err := crash.GetDaemonStoreCrashDirs(ctx)
	if err != nil {
		s.Fatal("Couldn't get daemon store dirs: ", err)
	}
	// We might not be logged in, so also allow system crash dir.
	crashDirs = append(crashDirs, crash.SystemCrashDir)

	// Check that expected device coredump is copied to crash directory.
	ctxForRemovingAllFiles := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()
	s.Log("Waiting for {.devcore.gz, .meta, .log} files to be added to crash directory")
	devCoreFiles, err := crash.WaitForCrashFiles(ctx, crashDirs,
		[]string{devCoreDumpName, metaDumpName, logDumpName})
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

	if err := intelfwextractor.ValidateFWDump(ctx, file); err != nil {
		s.Fatal("Failed to validate the fw dump: ", err)
	}

}
