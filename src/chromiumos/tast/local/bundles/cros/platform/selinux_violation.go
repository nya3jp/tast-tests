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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SelinuxViolation,
		Desc:         "Verify selinux violations are logged as expected",
		Contacts:     []string{"mutexlox@google.com", "cros-monitoring-forensics@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "metrics_consent", "selinux"},
		Pre:          crash.ChromePreWithVerboseConsent(),
	})
}

func saveSelinuxLog(ctx context.Context, destDir string) error {
	out, err := testexec.CommandContext(ctx, "journalctl", "--boot", "--identifier=audit", "--lines=500").Output()
	if err != nil {
		return errors.Wrap(err, "failed to get selinux audit log entries")
	}

	if err := ioutil.WriteFile(filepath.Join(destDir, "audit.log"), out, 0644); err != nil {
		return errors.Wrap(err, "failed to write selinux audit log")
	}
	return nil
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
	s.Log("Generating audit event")
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

	const (
		logFileRegex  = `selinux_violation_cros\.\d{8}\.\d{6}\.0\.log`
		metaFileRegex = `selinux_violation_cros\.\d{8}\.\d{6}\.0\.meta`
	)
	expectedRegexes := []string{logFileRegex, metaFileRegex}

	s.Log("Waiting for crash files")

	files, err := crash.WaitForCrashFiles(ctx, []string{crash.SystemCrashDir}, oldFiles, expectedRegexes)
	if err != nil {
		if err := saveSelinuxLog(ctx, s.OutDir()); err != nil {
			s.Error("Failed to save selinux log: ", err)
		}
		s.Fatalf("Couldn't find expected files: %v. Attempting to save audit log", err)
	}

	expectedLogMsgs := []string{"AVC avc:  granted  { create }",
		fileName,
		"cros_audit_sanity_test_file"}

	var matchingFile string
	for _, f := range files[logFileRegex] {
		contents, err := ioutil.ReadFile(f)
		if err != nil {
			s.Errorf("Couldn't read log file %s: %v", f, err)
		} else {
			fileMatches := true
			for _, m := range expectedLogMsgs {
				if !strings.Contains(string(contents), m) {
					// Only a Log (not an error) because it might just be a
					// different selinux failure
					s.Logf("Didn't find %s", m)
					fileMatches = false
				}
			}
			if fileMatches {
				if matchingFile != "" {
					s.Errorf("Found two matching files: %s and %s", matchingFile, f)
				} else {
					matchingFile = f
				}
			}
		}
	}

	if matchingFile != "" {
		// We found the right one, so remove only the relevant logs.
		metaFile := strings.TrimSuffix(matchingFile, ".log") + ".meta"
		for _, f := range []string{matchingFile, metaFile} {
			if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
				s.Error(ctx, "Couldn't clean up %s: %v", f, err)
			}
		}
	} else {
		// We did not find the right one. The ones that are left may be
		// real failures or the one we were looking for formatted
		// differently than we expected.
		// Move files to out dir for inspection.
		s.Error("Did not find selinux failure. Moving files found to out dir")
		if err := saveSelinuxLog(ctx, s.OutDir()); err != nil {
			s.Error("Failed to save selinux log: ", err)
		}
		allFiles := append(append([]string(nil), files[logFileRegex]...), files[metaFileRegex]...)
		if err := crash.MoveFilesToOut(ctx, s.OutDir(), allFiles...); err != nil {
			s.Error("Could not move files to out dir: ", err)
		}
	}
}
