// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	iwlwifiPath = "/sys/kernel/debug/iwlwifi"
	fwnmiPath   = "/iwlmvm/fw_nmi"
	funcName    = `NMI_INTERRUPT_UNKNOWN`
	baseName    = `kernel_iwlwifi_error_` + funcName + `\.\d{8}\.\d{6}\.0`
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     KernelIwlwifiError,
		Desc:     "Verify kernel iwlwifi errors are logged as expected",
		Contacts: []string{"arowa@google.com", "cros-telemetry@google.com"},
		Attr:     []string{"group:mainline"},
		Params: []testing.Param{{
			Name:              "real_consent",
			ExtraSoftwareDeps: []string{"chrome", "metrics_consent"},
			ExtraAttr:         []string{"informational"},
			Pre:               crash.ChromePreWithVerboseConsent(),
			Val:               crash.RealConsent,
		}, {
			Name:      "mock_consent",
			ExtraAttr: []string{"informational"},
			Val:       crash.MockConsent,
		}},
	})
}

func KernelIwlwifiError(ctx context.Context, s *testing.State) {
	// TODO(crbug.com/950346): Remove the below check and add dependency on Intel WiFi
	// when hardware dependencies are implemented.
	// Verify that DUT has Intel WiFi.
	if _, err := os.Stat(iwlwifiPath); os.IsNotExist(err) {
		s.Fatal("iwlwifi directory does not exist on DUT, skipping test")
	}

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

	output, err := testexec.CommandContext(ctx, "lshw", "-c", "network").Output()
	if err != nil {
		s.Fatal("Failed to get the pci name of the wifi chip: ", err)
	}

	// Find the network pci interface name.
	pat := regexp.MustCompile("bus info: pci@(.*)")
	pci := pat.FindStringSubmatch(string(output))

	fwnmi := filepath.Join(filepath.Join(iwlwifiPath, pci[1]), fwnmiPath)
	if _, err := os.Stat(fwnmi); err != nil {
		s.Fatal("Failed to induce an artifical iwlwifi error: ", err)
	}

	s.Log("Inducing artificial iwlwifi error")
	if err := ioutil.WriteFile(fwnmi, []byte("1"), 0); err != nil {
		s.Fatal("Failed to induce iwlwifi error in lkdtm: ", err)
	}

	s.Log("Waiting for files")
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

}
