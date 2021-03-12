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
)

const (
	athPath          = "/sys/kernel/debug/ieee80211/phy0/ath10k/simulate_fw_crash"
	funcAthName      = `(firmware_crashed)`
	crashAthBaseName = `kernel_ath_error_` + funcAthName + `\.\d{8}\.\d{6}\.\d+\.0`
)

var (
	expectedAthRegexes = []string{crashAthBaseName + `\.kcrash`,
		crashAthBaseName + `\.log\.gz`,
		crashAthBaseName + `\.meta`}
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         KernelAthError,
		Desc:         "Verify kernel ath crashes are logged as expected",
		Contacts:     []string{"arowa@google.com", "cros-telemetry@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"wifi"},
		//HardwareDeps: hwdep.D(hwdep.Board("trogdor")),
	})
}

func KernelAthError(ctx context.Context, s *testing.State) {
	opt := crash.WithMockConsent()

	if err := crash.SetUpCrashTest(ctx, opt); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest(ctx)

	if err := crash.RestartAnomalyDetector(ctx); err != nil {
		s.Fatal("Failed to restart anomaly detector: ", err)
	}

	if _, err := os.Stat(athPath); err != nil {
		s.Fatalf("Failed to get the file information for %s: %v", athPath, err)
	}

	s.Log("Inducing artificial ath crash")
	if err := ioutil.WriteFile(athPath, []byte("soft"), 0); err != nil {
		s.Fatal("Failed to induce iwlwifi error in fw_nmi: ", err)
	}

	s.Log("Waiting for files")
	files, err := crash.WaitForCrashFiles(ctx, []string{crash.SystemCrashDir}, expectedAthRegexes, crash.Timeout(60*time.Second))
	if err != nil {
		s.Fatal("Couldn't find expected files: ", err)
	}

	if err := crash.RemoveAllFiles(ctx, files); err != nil {
		s.Error("Couldn't clean up files: ", err)
	}

}
