// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/network/iface"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	ieeeDebugPath       = "/sys/kernel/debug/ieee80211"
	simulateFwCrashPath = "/ath10k/simulate_fw_crash"
	crashAth10kBaseName = `kernel_ath10k_error_firmware_crashed\.\d{8}\.\d{6}\.\d+\.0`
	crashAth10kMetaName = crashAth10kBaseName + `\.meta`
	sigAth10k           = "sig=ath10k_.*firmware crashed"
)

var (
	expectedAth10kRegexes = []string{crashAth10kBaseName + `\.kcrash`,
		crashAth10kBaseName + `\.log\.gz`,
		crashAth10kMetaName}
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         KernelAth10kError,
		Desc:         "Verify kernel ath10k crashes are logged as expected",
		Contacts:     []string{"arowa@google.com", "cros-telemetry@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"wifi"},
		HardwareDeps: hwdep.D(hwdep.WifiQualcomm()),
	})
}

func KernelAth10kError(ctx context.Context, s *testing.State) {
	opt := crash.WithMockConsent()

	if err := crash.SetUpCrashTest(ctx, opt); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest(ctx)

	if err := crash.RestartAnomalyDetector(ctx); err != nil {
		s.Fatal("Failed to restart anomaly detector: ", err)
	}

	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create shill manager proxy: ", err)
	}

	ifaceName, err := shill.WifiInterface(ctx, m, 5*time.Second)
	if err != nil {
		s.Fatal("Failed to get the WiFi interface: ", err)
	}

	netIface := iface.NewInterface(ifaceName)

	phyName, err := netIface.PhyName(ctx)
	if err != nil {
		s.Fatal("Failed to get the network parent device name: ", err)
	}

	ath10kPath := filepath.Join(filepath.Join(ieeeDebugPath, phyName), simulateFwCrashPath)
	if _, err := os.Stat(ath10kPath); err != nil {
		s.Fatalf("Failed to get the file information for %s: %v", ath10kPath, err)
	}

	s.Log("Inducing artificial ath10k crash")
	if err := ioutil.WriteFile(ath10kPath, []byte("soft"), 0); err != nil {
		s.Fatal("Failed to induce iwlwifi error in fw_nmi: ", err)
	}

	s.Log("Waiting for files")
	files, err := crash.WaitForCrashFiles(ctx, []string{crash.SystemCrashDir}, expectedAth10kRegexes, crash.Timeout(60*time.Second))
	if err != nil {
		s.Fatal("Couldn't find expected files: ", err)
	}

	if len(files[crashAth10kMetaName]) == 1 {
		metaFile := files[crashAth10kMetaName][0]
		contents, err := ioutil.ReadFile(metaFile)
		if err != nil {
			s.Errorf("Couldn't read meta file %s contents: %v", metaFile, err)
		} else if res, err := regexp.MatchString(sigAth10k, string(contents)); err != nil || !res {
			s.Error("Failed to find the expected Ath10k signature")
			crash.MoveFilesToOut(ctx, s.OutDir(), metaFile)
		}
	} else {
		s.Errorf("Unexpectedly found multiple meta files: %q", files[crashAth10kMetaName])
		crash.MoveFilesToOut(ctx, s.OutDir(), files[crashAth10kMetaName]...)
	}

	if err := crash.RemoveAllFiles(ctx, files); err != nil {
		s.Error("Couldn't clean up files: ", err)
	}

}
