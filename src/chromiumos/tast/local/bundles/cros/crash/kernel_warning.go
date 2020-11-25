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
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     KernelWarning,
		Desc:     "Verify kernel warnings are logged as expected",
		Contacts: []string{"mutexlox@google.com", "cros-telemetry@google.com"},
		Attr:     []string{"group:mainline"},
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
	if err := crash.SetUpCrashTest(ctx, opt); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest(ctx)

	if err := crash.RestartAnomalyDetector(ctx); err != nil {
		s.Fatal("Failed to restart anomaly detector: ", err)
	}

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
			s.Error(".meta file did not contain expected contents")
			crash.MoveFilesToOut(ctx, s.OutDir(), metaFile)
		}
	} else {
		s.Errorf("Unexpectedly found multiple meta files: %q", files[metaName])
		crash.MoveFilesToOut(ctx, s.OutDir(), files[metaName]...)
	}

	if err := crash.RemoveAllFiles(ctx, files); err != nil {
		s.Log("Couldn't clean up files: ", err)
	}
}
