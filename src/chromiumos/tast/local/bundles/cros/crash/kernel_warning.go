// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"io/ioutil"
	"os"
	"strings"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         KernelWarning,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify kernel warnings are logged as expected",
		Contacts:     []string{"mutexlox@google.com", "cros-telemetry@google.com"},
		Attr:         []string{"group:mainline"},
		// TODO(b/201790026): The lkdtm resides on the debugfs,
		// which is not accessible when integrity mode is
		// enabled.
		//
		// We either need to refactor the test not to use lkdtm
		// to trigger crashes or we need to modify the kernel to
		// allow access to the required path. Skip on reven for
		// now, since reven uses integrity mode.
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("reven")),
		Params: []testing.Param{{
			Name:              "real_consent",
			ExtraSoftwareDeps: []string{"chrome", "metrics_consent"},
			ExtraAttr:         []string{"informational"},
			Pre:               crash.ChromePreWithVerboseConsent(),
			Val:               crash.RealConsent,
		}, {
			Name: "mock_consent",
			Val:  crash.MockConsent,
		}},
	})
}

func KernelWarning(ctx context.Context, s *testing.State) {
	opt := crash.WithMockConsent()
	useConsent := s.Param().(crash.ConsentType)
	if useConsent == crash.RealConsent {
		opt = crash.WithConsent(s.PreValue().(*chrome.Chrome))
	}
	if err := crash.SetUpCrashTest(ctx, crash.FilterCrashes("kernel_warning"), opt); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest(ctx)

	if err := crash.RestartAnomalyDetectorWithSendAll(ctx, true); err != nil {
		s.Fatal("Failed to restart anomaly detector: ", err)
	}
	// Restart anomaly detector to clear its --testonly-send-all flag at the end of execution.
	defer crash.RestartAnomalyDetector(ctx)

	s.Log("Inducing artificial warning")
	lkdtm := "/sys/kernel/debug/provoke-crash/DIRECT"
	if _, err := os.Stat(lkdtm); err == nil {
		if err := ioutil.WriteFile(lkdtm, []byte("WARNING"), 0); err != nil {
			s.Fatal("Failed to induce warning in lkdtm: ", err)
		}
	} else {
		if err := ioutil.WriteFile("/proc/breakme", []byte("warning"), 0); err != nil {
			s.Fatal("Failed to induce warning in breakme: ", err)
		}
	}

	s.Log("Waiting for files")
	const (
		funcName = `[a-zA-Z0-9_]*(?:lkdtm|breakme|direct_entry)[a-zA-Z0-9_]*`
		baseName = `kernel_warning_` + funcName + `\.\d{8}\.\d{6}\.\d+\.0`
		metaName = baseName + `\.meta`
	)
	expectedRegexes := []string{baseName + `\.kcrash`,
		baseName + `\.log\.gz`,
		metaName}
	files, err := crash.WaitForCrashFiles(ctx, []string{crash.SystemCrashDir}, expectedRegexes)
	if err != nil {
		s.Fatal("Couldn't find expected files: ", err)
	}

	if len(files[metaName]) == 1 {
		metaFile := files[metaName][0]
		if contents, err := ioutil.ReadFile(metaFile); err != nil {
			s.Errorf("Couldn't read meta file %s contents: %v", metaFile, err)
		} else if !strings.Contains(string(contents), "upload_var_in_progress_integration_test=crash.KernelWarning") {
			s.Error(".meta file did not contain expected in_progress_integration_test")
			if err := crash.MoveFilesToOut(ctx, s.OutDir(), metaFile); err != nil {
				s.Error("Failed to save the meta file: ", err)
			}
		} else if !strings.Contains(string(contents), "upload_var_weight=10") {
			s.Error(".meta file did not contain expected weight")
			if err := crash.MoveFilesToOut(ctx, s.OutDir(), metaFile); err != nil {
				s.Error("Failed to save the meta file: ", err)
			}
		}
	} else {
		s.Errorf("Unexpectedly found multiple meta files: %q", files[metaName])
		if err := crash.MoveFilesToOut(ctx, s.OutDir(), files[metaName]...); err != nil {
			s.Error("Failed to save additional meta files: ", err)
		}
	}

	if err := crash.RemoveAllFiles(ctx, files); err != nil {
		s.Log("Couldn't clean up files: ", err)
	}
}
