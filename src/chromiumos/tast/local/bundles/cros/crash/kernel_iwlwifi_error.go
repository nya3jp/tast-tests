// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/network/iface"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	iwlwifiPath   = "/sys/kernel/debug/iwlwifi"
	fwnmiPath     = "/iwlmvm/fw_nmi"
	funcName      = `NMI_INTERRUPT_UNKNOWN`
	crashBaseName = `kernel_iwlwifi_error_` + funcName + `\.\d{8}\.\d{6}\.0`
)

var (
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
		// TODO(crbug.com/1070299): Remove the below hard-coded devices
		// and use Intel WiFi dependency when wifi hardware
		// dependencies are implemented.
		// NB: These exclusions are somewhat overly broad; some
		// (but not all) blooglet, ezkinil, and trembyle devices have
		// WiFi chips that would work for this test. However, for now
		// there is no better way to specify the exact hardware
		// parameters needed for this test. (See linked bug.)
		HardwareDeps: hwdep.D(hwdep.SkipOnPlatform("bob",
			"elm",
			"grunt",
			"hana",
			"jacuzzi",
			"kevin",
			"kukui",
			"scarlet",
			"veyron_fievel",
			"veyron_mickey",
			"veyron_tiger",
		), hwdep.SkipOnModel("blooglet", "ezkinil", "trembyle")),
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

	fwnmi := filepath.Join(filepath.Join(iwlwifiPath, devName), fwnmiPath)
	if _, err := os.Stat(fwnmi); err != nil {
		s.Fatalf("Failed to get the file information for %s: %v", fwnmi, err)
	}

	s.Log("Inducing artificial iwlwifi error")
	if err := ioutil.WriteFile(fwnmi, []byte("1"), 0); err != nil {
		s.Fatal("Failed to induce iwlwifi error in fw_nmi: ", err)
	}

	s.Log("Waiting for files")
	files, err := crash.WaitForCrashFiles(ctx, []string{crash.SystemCrashDir}, expectedRegexes)
	if err != nil {
		s.Fatal("Couldn't find expected files: ", err)
	}

	if err := crash.RemoveAllFiles(ctx, files); err != nil {
		s.Error("Couldn't clean up files: ", err)
	}

}
