// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"
	"os"

	"chromiumos/tast/crash"
	"chromiumos/tast/local/chrome"
	localCrash "chromiumos/tast/local/crash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         KernelWarning,
		Desc:         "Verify kernel warnings are logged as expected",
		Contacts:     []string{"mutexlox@google.com", "cros-monitoring-forensics@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Pre:          chrome.LoggedIn(), // chrome.LoggedIn sets up metrics consent via SkipToLoginForTesting
	})
}

func KernelWarning(ctx context.Context, s *testing.State) {
	if err := localCrash.SetUpCrashTest(); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer localCrash.TearDownCrashTest()

	oldFiles, err := crash.GetCrashes(localCrash.SystemCrashDir)
	if err != nil {
		s.Fatal("Failed to get original crashes: ", err)
	}

	if err := localCrash.RestartAnomalyDetector(ctx); err != nil {
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
	expectedRegexes := []string{`kernel_warning\.\d{8}\.\d{6}\.0\.kcrash`,
		`kernel_warning\.\d{8}\.\d{6}\.0\.log\.gz`,
		`kernel_warning\.\d{8}\.\d{6}\.0\.meta`}
	files, err := localCrash.WaitForCrashFiles(ctx, []string{localCrash.SystemCrashDir}, oldFiles, expectedRegexes)
	if err != nil {
		s.Error("Couldn't find expected files: ", err)
	}
	// Clean up files.
	for _, f := range files {
		if err := os.Remove(f); err != nil {
			s.Logf("Couldn't clean up %s: %v", f, err)
		}
	}
}
