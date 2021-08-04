// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/network/iface"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	iwlwifiPath   = "/sys/kernel/debug/iwlwifi"
	fwnmiPath     = "/iwlmvm/fw_nmi"
	funcName      = `(NMI_INTERRUPT_UNKNOWN|ADVANCED_SYSASSERT)`
	crashBaseName = `kernel_iwlwifi_error_` + funcName + `\.\d{8}\.\d{6}\.\d+\.0`
	messagesFile  = "/var/log/messages"
	logName       = "filesystem_and_disk_status.txt"
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
		Attr:         []string{"group:mainline", "group:wificell", "wificell_func"},
		SoftwareDeps: []string{"wifi"},
		// NB: The WifiIntel dependency tracks a manually maintained list of devices.
		// If the test is skipping when it should run or vice versa, check the hwdep
		// to see if your board is incorrectly included/excluded.
		HardwareDeps: hwdep.D(hwdep.WifiIntel()),
	})
}

func KernelIwlwifiError(ctx context.Context, s *testing.State) {
	// TODO(b:193677828) Remove the filesystem and disk checks below when the issue with
	// /var/log/messages being broken sometimes is fixed.
	// Get filesystem/disks info for debugging"
	dfOutput, err := testexec.CommandContext(ctx, "df", "-mP").Output()
	if err != nil {
		s.Error("Failed to run the command df -mP: ", err)
	}
	content := "Output of the command df -mP at the beginning of the test:\n" + string(dfOutput)
	duOutput, err := testexec.CommandContext(ctx, "du", "-a", "/mnt/stateful_partition/encrypted").Output()
	if err != nil {
		s.Error("Failed to run the command du -a /mnt/stateful_partition/encrypted: ", err)
	}
	content = content + "\n\nOutput of the command du -a /mnt/stateful_partition/encrypted at the beginning of the test:\n" + string(duOutput)
	// Write the filesystem/disks info logs to the file logName.
	dir, ok := testing.ContextOutDir(ctx)
	if !ok {
		s.Error("Failed to get OutDir")
	}

	if err := ioutil.WriteFile(filepath.Join(dir, logName),
		[]byte(content), 0644); err != nil {
		s.Error("Failed to write filesystem/disks info outputs: ", err)
	}

	// Check that /var/log/messages exist.
	s.Log("Checking for /var/log/messages existance")
	_, err = os.Stat(messagesFile)
	if os.IsNotExist(err) {
		s.Errorf("File %s does not exists: %v", messagesFile, err)
	}

	// TODO(crbug.com/950346): Remove the below check and add dependency on Intel WiFi
	// when hardware dependencies are implemented.
	// Verify that DUT has Intel WiFi.
	if _, err := os.Stat(iwlwifiPath); os.IsNotExist(err) {
		s.Fatal("iwlwifi directory does not exist on DUT, skipping test")
	}

	opt := crash.WithMockConsent()

	if err := crash.SetUpCrashTest(ctx, crash.FilterCrashes("kernel_iwlwifi_error"), opt); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest(ctx)

	if err := crash.RestartAnomalyDetectorWithSendAll(ctx, true); err != nil {
		s.Fatal("Failed to restart anomaly detector: ", err)
	}
	// Restart anomaly detector to clear its --testonly-send-all flag at the end of execution.
	defer crash.RestartAnomalyDetector(ctx)

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
	files, err := crash.WaitForCrashFiles(ctx, []string{crash.SystemCrashDir}, expectedRegexes, crash.Timeout(60*time.Second))
	if err != nil {
		s.Fatal("Couldn't find expected files: ", err)
	}
	defer func() {
		if err := crash.RemoveAllFiles(ctx, files); err != nil {
			s.Error("Couldn't clean up files: ", err)
		}
	}()

	metaName := crashBaseName + `\.meta`
	if len(files[metaName]) != 1 {
		s.Errorf("Unexpectedly found multiple meta files: %q", files[metaName])
		if err := crash.MoveFilesToOut(ctx, s.OutDir(), files[metaName]...); err != nil {
			s.Error("Failed to save additional meta files: ", err)
		}
		return
	}
	metaFile := files[metaName][0]
	contents, err := ioutil.ReadFile(metaFile)
	if err != nil {
		s.Fatalf("Couldn't read meta file %s contents: %v", metaFile, err)
	}
	if !strings.Contains(string(contents), "upload_var_weight=50") {
		s.Error(".meta file did not contain expected weight")
		if err := crash.MoveFilesToOut(ctx, s.OutDir(), metaFile); err != nil {
			s.Error("Failed to save the meta file: ", err)
		}
	}
}
