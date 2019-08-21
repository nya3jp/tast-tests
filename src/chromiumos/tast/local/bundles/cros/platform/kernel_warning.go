// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"
	"os"
	"regexp"
	"time"

	// TODO(mutexlox): Re-enable this once dependent CL is submitted
	"chromiumos/tast/crash"
	"chromiumos/tast/errors"
	platform_crash "chromiumos/tast/local/bundles/cros/platform/crash"
	"chromiumos/tast/local/metrics"
	"chromiumos/tast/local/set"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     KernelWarning,
		Desc:     "Verify kernel warnings are logged as expected",
		Contacts: []string{"cros-monitoring-forensics@chromium.org"},
		Attr:     []string{"informational"},
		Data:     []string{platform_crash.TestCert},
	})
}

func KernelWarning(ctx context.Context, s *testing.State) {
	if err := metrics.SetConsent(ctx, s.DataPath(platform_crash.TestCert)); err != nil {
		s.Fatal("Failed to set consent: ", err)
	}

	const systemCrashDir = "/var/spool/crash"
	oldFiles, err := crash.GetCrashes(systemCrashDir)
	if err != nil {
		s.Fatalf("Failed to get original crashes: %s", err)
	}

	// Restart anomaly detector to clear its cache of recently seen service
	// failures and ensure this one is logged.
	if err := upstart.RestartJob(ctx, "anomaly-detector"); err != nil {
		s.Fatalf("Couldn't restart anomaly-detector: %s", err)
	}

	// Give enough time for the anomaly detector to open the journal and scan to the end.
	// (Otherwise, it might miss the warning message.)
	testing.Sleep(ctx, time.Millisecond*500)

	lkdtm := "/sys/kernel/debug/provoke-crash/DIRECT"
	if _, err := os.Stat(lkdtm); err == nil {
		if err := ioutil.WriteFile(lkdtm, []byte("WARNING"), 0); err != nil {
			s.Fatalf("Failed to induce warning in lkdtm: %s", err)
		}
	} else {
		if err := ioutil.WriteFile("/proc/breakme", []byte("warning"), 0); err != nil {
			s.Fatalf("Failed to induce warning in breakme: %s", err)
		}
	}

	// TODO(mutexlox): This code is very similar to service_failure.go. Pull it out into a method.
	err = testing.Poll(ctx, func(c context.Context) error {
		newFiles, err := crash.GetCrashes(systemCrashDir)
		if err != nil {
			s.Fatalf("Failed to get new crashes: %s", err)
		}
		diffFiles := set.DiffStringSlice(newFiles, oldFiles)

		expectedRegexes := []string{`kernel_warning\.\d{8}\.\d{6}\.0\.kcrash`,
			`kernel_warning\.\d{8}\.\d{6}\.0\.log\.gz`,
			`kernel_warning\.\d{8}\.\d{6}\.0\.meta`}

		var missing []string
		var files []string
		for _, re := range expectedRegexes {
			match := false
			for _, f := range diffFiles {
				match, err = regexp.MatchString(re, f)
				if err != nil {
					s.Fatalf("Invalid regexp %s", re)
				}
				if match {
					files = append(files, f)
					break
				}
			}
			if !match {
				missing = append(missing, re)
			}
		}
		if len(missing) != 0 {
			return errors.Errorf("Missing some files: %v. Actual files: %v", missing, diffFiles)
		}
		// Clean up files.
		for _, f := range files {
			if err := os.Remove(f); err != nil {
				s.Logf("Couldn't clean up %s: %s", f, err)
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second})
	if err != nil {
		s.Errorf("Failed: %s", err)
	}
}
