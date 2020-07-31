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
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/network/iface"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	ieeeDebugPath       = "/sys/kernel/debug/ieee80211"
	simulateFWCrashPath = "/ath10k/simulate_fw_crash"
	crashAth10kBaseName = `kernel_ath10k_error_firmware_crashed\.\d{8}\.\d{6}\.\d+\.0`
	crashAth10kMetaName = crashAth10kBaseName + `\.meta`
	sigAth10k           = "sig=ath10k_.*firmware crashed"
)

var expectedAth10kRegexes = []string{
	crashAth10kBaseName + `\.kcrash`,
	crashAth10kBaseName + `\.log\.gz`,
	crashAth10kMetaName,
}

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

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 3*time.Second)
	defer cancel()
	if err := crash.SetUpCrashTest(ctx, crash.FilterCrashes("kernel_ath10k_error"), opt); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest(cleanupCtx)

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

	phyName, err := netIface.PhyName(ctx)
	if err != nil {
		s.Fatal("Failed to get the network parent device name: ", err)
	}

	ath10kPath := filepath.Join(filepath.Join(ieeeDebugPath, phyName), simulateFWCrashPath)
	if _, err := os.Stat(ath10kPath); os.IsNotExist(err) {
		s.Fatalf("Failed to find the file %s: %v", ath10kPath, err)
	}

	s.Log("Inducing artificial ath10k crash")
	if err := ioutil.WriteFile(ath10kPath, []byte("soft"), 0); err != nil {
		s.Fatal("Failed to induce iwlwifi error in fw_nmi: ", err)
	}

	s.Log("Waiting for files")
	removeFilesCtx := ctx
	ctx, cancel = ctxutil.Shorten(removeFilesCtx, time.Second)
	defer cancel()

	crashDirs, err := crash.GetDaemonStoreCrashDirs(ctx)
	if err != nil {
		s.Fatal("Couldn't get daemon store dirs: ", err)
	}
	// We might not be logged in, so also allow system crash dir.
	crashDirs = append(crashDirs, crash.SystemCrashDir)
	files, err := crash.WaitForCrashFiles(ctx, crashDirs, expectedAth10kRegexes, crash.Timeout(60*time.Second))
	if err != nil {
		s.Fatal("Couldn't find expected files: ", err)
	}
	defer func(ctx context.Context) {
		if err := crash.RemoveAllFiles(ctx, files); err != nil {
			s.Error("Couldn't clean up files: ", err)
		}
	}(removeFilesCtx)

	if len(files[crashAth10kMetaName]) == 1 {
		metaFile := files[crashAth10kMetaName][0]
		contents, err := ioutil.ReadFile(metaFile)
		if err != nil {
			s.Errorf("Couldn't read meta file %s contents: %v", metaFile, err)
		} else {
			if res, err := regexp.Match(sigAth10k, contents); err != nil {
				s.Error("Failed to frun regex: ", err)
				if err := crash.MoveFilesToOut(ctx, s.OutDir(), metaFile); err != nil {
					s.Error("Failed to save the meta file: ", err)
				}
			} else if !res {
				s.Error("Failed to find the expected Ath10k signature")
				if err := crash.MoveFilesToOut(ctx, s.OutDir(), metaFile); err != nil {
					s.Error("Failed to save the meta file: ", err)
				}
			}

			if !strings.Contains(string(contents), "upload_var_weight=50") {
				s.Error(".meta file did not contain expected weight")
				if err := crash.MoveFilesToOut(ctx, s.OutDir(), metaFile); err != nil {
					s.Error("Failed to save the meta file: ", err)
				}
			}
		}
	} else {
		s.Errorf("Unexpectedly found multiple meta files: %q", files[crashAth10kMetaName])
		if err := crash.MoveFilesToOut(ctx, s.OutDir(), files[crashAth10kMetaName]...); err != nil {
			s.Error("Failed to save unexpected crashes: ", err)
		}
	}

}
