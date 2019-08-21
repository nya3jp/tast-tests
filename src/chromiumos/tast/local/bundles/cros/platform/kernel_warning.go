// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/crash"
	"chromiumos/tast/errors"
	platformCrash "chromiumos/tast/local/bundles/cros/platform/crash"
	"chromiumos/tast/local/metrics"
	"chromiumos/tast/local/set"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     KernelWarning,
		Desc:     "Verify kernel warnings are logged as expected",
		Contacts: []string{"mutexlox@google.com", "cros-monitoring-forensics@chromium.org"},
		Attr:     []string{"informational"},
		Data:     []string{platformCrash.TestCert},
	})
}

func KernelWarning(ctx context.Context, s *testing.State) {
	if err := metrics.SetConsent(ctx, s.DataPath(platformCrash.TestCert)); err != nil {
		s.Fatal("Failed to set consent: ", err)
	}

	const systemCrashDir = "/var/spool/crash"
	oldFiles, err := crash.GetCrashes(systemCrashDir)
	if err != nil {
		s.Fatal("Failed to get original crashes: ", err)
	}

	w, err := syslog.NewWatcher(syslog.MessageFile)
	if err != nil {
		s.Fatalf("Couldn't create watcher for %s: %v", syslog.MessageFile, err)
	}
	defer w.Close()

	// Restart anomaly detector to clear its cache of recently seen service
	// failures and ensure this one is logged.
	if err := upstart.RestartJob(ctx, "anomaly-detector"); err != nil {
		s.Fatal("Couldn't restart anomaly-detector: ", err)
	}

	// Wait for anomaly detector to indicate that it's ready. Otherwise, it'll miss the warning.
	if err := w.WaitForMessage(ctx, "Opened journal and sought to end"); err != nil {
		s.Fatal("Failed to wait for anomaly detector to start: ", err)
	}

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

	// TODO(mutexlox): This code is very similar to service_failure.go. Pull it out into a method.
	err = testing.Poll(ctx, func(c context.Context) error {
		newFiles, err := crash.GetCrashes(systemCrashDir)
		if err != nil {
			s.Fatal("Failed to get new crashes: ", err)
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
					s.Fatalf("Invalid regexp %s (err: %v)", re, err)
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
			return errors.Errorf("no file matched %s (found %s)", strings.Join(missing, ", "), strings.Join(diffFiles, ", "))
		}
		// Clean up files.
		for _, f := range files {
			if err := os.Remove(f); err != nil {
				s.Logf("Couldn't clean up %s: %v", f, err)
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second})
	if err != nil {
		s.Error("Failed: ", err)
	}
}
