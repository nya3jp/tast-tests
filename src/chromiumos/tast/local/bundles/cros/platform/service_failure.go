// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"time"

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
		Func:     ServiceFailure,
		Desc:     "Verify service failures are logged as expected",
		Contacts: []string{"cros-monitoring-forensics@chromium.org"},
		Attr:     []string{"informational"},
		Data:     []string{platform_crash.TestCert},
	})
}

func ServiceFailure(ctx context.Context, s *testing.State) {
	const systemCrashDir = "/var/spool/crash"
	const failingServiceName = "failing-service"

	failureStages := []string{"PRE_START_EXIT", "MAIN_EXIT", "POST_START_EXIT"}
	messages := []string{"pre-start process", "main process", "post-start process"}

	if err := metrics.SetConsent(ctx, s.DataPath(platform_crash.TestCert)); err != nil {
		s.Fatal("Failed to set consent: ", err)
	}

	oldFiles, err := crash.GetCrashes(systemCrashDir)
	if err != nil {
		s.Fatalf("Failed to get original crashes: %s", err)
	}

	for i, env := range failureStages {
		// Restart anomaly detector to clear its cache of recently seen service
		// failures and ensure this one is logged.
		if err := upstart.RestartJob(ctx, "anomaly-detector"); err != nil {
			s.Fatalf("Couldn't restart anomaly-detector: %s", err)
		}

		// Give enough time for the anomaly detector to open the journal and scan to the end.
		// (Otherwise, it might miss the warning message.)
		testing.Sleep(ctx, time.Millisecond*500)

		if err := upstart.StartJob(ctx, failingServiceName, fmt.Sprintf("%s=1", env)); err != nil {
			// Ignore error; it's expected.
			// (upstart exits nonzero if a job fails in pre-start)
		}

		expectedLogMsg := fmt.Sprintf("%s %s", failingServiceName, messages[i])

		err = testing.Poll(ctx, func(c context.Context) error {
			newFiles, err := crash.GetCrashes(systemCrashDir)
			if err != nil {
				s.Fatalf("Failed to get new crashes: %s", err)
			}
			diffFiles := set.DiffStringSlice(newFiles, oldFiles)
			expectedRegexes := []string{`service_failure_failing_service\.\d{8}\.\d{6}\.0\.log`,
				`service_failure_failing_service\.\d{8}\.\d{6}\.0\.meta`}
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
				return errors.Errorf("Missing some files: %v", missing)
			}
			// Clean up files and check contents.
			for _, f := range files {
				if strings.HasSuffix(f, ".log") {
					contents, err := ioutil.ReadFile(f)
					if err != nil {
						s.Errorf("Couldn't read log file: %s", err)
					}
					if !strings.Contains(string(contents), expectedLogMsg) {
						s.Errorf("Didn't find expected log contents: `%s`. Leaving %s for debugging", expectedLogMsg, f)
						continue
					}
				}
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
}
