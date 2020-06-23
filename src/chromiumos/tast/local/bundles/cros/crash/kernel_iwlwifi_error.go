// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"io/ioutil"
	"os"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     KernelIwlwifiError,
		Desc:     "Verify kernel iwlwifi errors are logged as expected",
		Contacts: []string{"arowa@google.com", "cros-telemetry@google.com"},
		Attr:     []string{"informational"},
		Params: []testing.Param{{
			Name:              "real_consent",
			ExtraSoftwareDeps: []string{"chrome", "metrics_consent"},
			Pre:               crash.ChromePreWithVerboseConsent(),
			Val:               crash.RealConsent,
		}, {
			Name: "mock_consent",
			Val:  crash.MockConsent,
		}},
	})
}

func KernelIwlwifiError(ctx context.Context, s *testing.State) {
	opt := crash.WithMockConsent()
	useConsent := s.Param().(crash.ConsentType)
	if useConsent == crash.RealConsent {
		opt = crash.WithConsent(s.PreValue().(*chrome.Chrome))
	}
	if err := crash.SetUpCrashTest(ctx, opt); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest(ctx)

	oldFiles, err := crash.GetCrashes(crash.SystemCrashDir)
	if err != nil {
		s.Fatal("Failed to get original crashes: ", err)
	}

	if err := crash.RestartAnomalyDetector(ctx); err != nil {
		s.Fatal("Failed to restart anomaly detector: ", err)
	}

	fwnmi := "/sys/kernel/debug/iwlwifi/0000:00:14.3/iwlmvm/fw_nmi"
	if _, err := os.Stat(fwnmi); err == nil {
		s.Log("Inducing artificial iwlwifi error")
		if err := ioutil.WriteFile(fwnmi, []byte("1"), 0); err != nil {
			s.Fatal("Failed to induce iwlwifi error in lkdtm: ", err)
		}
		s.Log("Waiting for files")
		const funcName = `NMI_INTERRUPT_UNKNOWN`
		const baseName = `kernel_iwlwifi_error_` + funcName + `\.\d{8}\.\d{6}\.0`
		expectedRegexes := []string{baseName + `\.kcrash`,
			baseName + `\.log\.gz`,
			baseName + `\.meta`}
		files, err := crash.WaitForCrashFiles(ctx, []string{crash.SystemCrashDir}, oldFiles, expectedRegexes)
		if err != nil {
			s.Fatal("Couldn't find expected files: ", err)
		}
		if err := crash.RemoveAllFiles(ctx, files); err != nil {
			s.Log("Couldn't clean up files: ", err)
		}
	} else {
		s.Log("Failed to induce an artifical iwlwifi error: ", err)
	}

}
