// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/local/crash"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	ath10kPath          = "/sys/kernel/debug/ieee80211/phy0/ath10k/simulate_fw_crash"
	funcAth10kName      = `(firmware_crashed)`
	crashAth10kBaseName = `kernel_ath10k_error_` + funcAth10kName + `\.\d{8}\.\d{6}\.\d+\.0`
)

var (
	expectedAth10kRegexes = []string{crashAth10kBaseName + `\.kcrash`,
		crashAth10kBaseName + `\.log\.gz`,
		crashAth10kBaseName + `\.meta`}
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

	if err := crash.RemoveAllFiles(ctx, files); err != nil {
		s.Error("Couldn't clean up files: ", err)
	}

}
