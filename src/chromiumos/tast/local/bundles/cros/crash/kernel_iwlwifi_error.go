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

	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	iwlwifiPath   = "/sys/kernel/debug/iwlwifi"
	fwnmiPath     = "/iwlmvm/fw_nmi"
	funcName      = `NMI_INTERRUPT_UNKNOWN`
	crashBaseName = `kernel_iwlwifi_error_` + funcName + `\.\d{8}\.\d{6}\.0`
)

var (
	pciNameRegexp   = regexp.MustCompile("bus info: pci@(.*)")
	expectedRegexes = []string{crashBaseName + `\.kcrash`,
		crashBaseName + `\.log\.gz`,
		crashBaseName + `\.meta`}
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         KernelIwlwifiError,
		Desc:         "Verify kernel iwlwifi errors are logged as expected",
		Contacts:     []string{"arowa@google.com", "cros-telemetry@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"wifi"},
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

	if err := crash.SetUpCrashTest(ctx, opt); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest(ctx)

	if err := crash.RestartAnomalyDetector(ctx); err != nil {
		s.Fatal("Failed to restart anomaly detector: ", err)
	}

	output, err := testexec.CommandContext(ctx, "lshw", "-c", "network").Output()
	if err != nil {
		s.Fatal("Failed to get the pci name of the wifi chip: ", err)
	}

	// Find the network pci interface name.
	pci := pciNameRegexp.FindStringSubmatch(string(output))

	fwnmi := filepath.Join(filepath.Join(iwlwifiPath, pci[1]), fwnmiPath)
	if _, err := os.Stat(fwnmi); err != nil {
		s.Fatalf("Failed to get the file information for %s: %v", fwnmi, err)
	}

	s.Log("Inducing artificial iwlwifi error")
	if err := ioutil.WriteFile(fwnmi, []byte("1"), 0); err != nil {
		s.Fatal("Failed to induce iwlwifi error in lkdtm: ", err)
	}

	s.Log("Waiting for files")
	files, err := crash.WaitForCrashFiles(ctx, []string{crash.SystemCrashDir}, nil, expectedRegexes)
	defer func() {
		if err := crash.RemoveAllFiles(ctx, files); err != nil {
			s.Error("Couldn't clean up files: ", err)
		}
	}()
	if err != nil {
		s.Fatal("Couldn't find expected files: ", err)
	}

}
