// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SelinuxViolation,
		Desc:         "Verify selinux violations are logged as expected",
		Contacts:     []string{"mutexlox@google.com", "cros-monitoring-forensics@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "metrics_consent"},
		Pre:          chrome.LoggedIn(),
	})
}

func SelinuxViolation(ctx context.Context, s *testing.State) {
	// Directory name should keep in sync with platform2/sepolicy/policy/chromeos/dev/cros_ssh_session.te
	const markerDirName = "cros_selinux_audit_sanity_test"

	cr := s.PreValue().(*chrome.Chrome)
	if err := crash.SetUpCrashTest(ctx, crash.WithConsent(cr)); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest()

	oldFiles, err := crash.GetCrashes(crash.SystemCrashDir)
	if err != nil {
		s.Fatal("Failed to get original crashes: ", err)
	}

	// Restart anomaly detector to clear its cache of recently seen service
	// failures and ensure this one is logged.
	if err := crash.RestartAnomalyDetectorWithSendAll(ctx, true); err != nil {
		s.Fatal("Failed to restart anomaly detector: ", err)
	}

	// Restart anomaly detector to clear its --testonly-send-all flag at the end of execution.
	defer crash.RestartAnomalyDetector(ctx)

	// Generate an audit event by creating a file inside markerDirectory
	td, err := ioutil.TempDir("/tmp", "tast.platform.SelinuxViolation.")
	if err != nil {
		s.Fatal("Failed to create temporary directory for testing: ", err)
	}
	defer os.RemoveAll(td)
	markerDirectory := filepath.Join(td, markerDirName)
	if err := os.Mkdir(markerDirectory, 0700); err != nil {
		s.Fatal("Failed to create marker directory for testing: ", err)
	}
	f, err := ioutil.TempFile(markerDirectory, "audit-marker-")
	if err != nil {
		s.Fatal("Failed to create marker file: ", err)
	}
	fileName := path.Base(f.Name())
	f.Close()

	expectedRegexes := []string{`selinux_violation\.\d{8}\.\d{6}\.0\.log`,
		`selinux_violation\.\d{8}\.\d{6}\.0\.meta`}

	files, err := crash.WaitForCrashFiles(ctx, []string{crash.SystemCrashDir}, oldFiles, expectedRegexes)
	if err != nil {
		s.Error("Couldn't find expected files: ", err)
	}

	expectedLogMsgs := []string{"AVC avc:  granted  { create }",
		fileName,
		"cros_audit_sanity_test_file"}

	for _, f := range files {
		if strings.HasSuffix(f, ".log") {
			contents, err := ioutil.ReadFile(f)
			if err != nil {
				s.Errorf("Couldn't read log file %s: %v", f, err)
				continue
			}
			for _, m := range expectedLogMsgs {
				if !strings.Contains(string(contents), m) {
					s.Errorf("didn't find expected log contents: %q. Leaving %s for debugging", m, filepath.Base(f))
					os.Rename(f, s.OutDir())
				}
			}
		}
		if err := os.Remove(f); err != nil && !os.IsNotExist(f) {
			s.Logf("Couldn't clean up %s: %v", f, err)
		}
	}
}
