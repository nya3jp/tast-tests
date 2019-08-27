// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"chromiumos/tast/crash"
	platformCrash "chromiumos/tast/local/bundles/cros/platform/crash"
	localCrash "chromiumos/tast/local/crash"
	"chromiumos/tast/local/metrics"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

type failureParams struct {
	envVar     string
	logMessage string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     ServiceFailure,
		Desc:     "Verify service failures are logged as expected",
		Contacts: []string{"mutexlox@google.com", "cros-monitoring-forensics@chromium.org"},
		Attr:     []string{"informational"},
		Data:     []string{platformCrash.TestCert},
		Params: []testing.Param{{
			Name: "pre_start",
			Val: failureParams{
				envVar:     "PRE_START_EXIT",
				logMessage: "pre-start process",
			},
		}, {
			Name: "main",
			Val: failureParams{
				envVar:     "MAIN_EXIT",
				logMessage: "main process",
			},
		}, {
			Name: "post_start",
			Val: failureParams{
				envVar:     "POST_START_EXIT",
				logMessage: "post-start process",
			},
		}},
	})
}

func ServiceFailure(ctx context.Context, s *testing.State) {
	const systemCrashDir = "/var/spool/crash"
	const failingServiceName = "failing-service"

	params := s.Param().(failureParams)

	if err := metrics.SetConsent(ctx, s.DataPath(platformCrash.TestCert)); err != nil {
		s.Fatal("Failed to set consent: ", err)
	}

	oldFiles, err := crash.GetCrashes(systemCrashDir)
	if err != nil {
		s.Fatal("Failed to get original crashes: ", err)
	}

	// Restart anomaly detector to clear its cache of recently seen service
	// failures and ensure this one is logged.
	if err := localCrash.RestartAnomalyDetector(ctx); err != nil {
		s.Fatal("Failed to restart anomaly detector: ", err)
	}

	if err := upstart.StartJob(ctx, failingServiceName, fmt.Sprintf("%s=1", params.envVar)); err != nil {
		// Ignore error; it's expected.
		// (upstart exits nonzero if a job fails in pre-start)
	}

	expectedLogMsg := fmt.Sprintf("%s %s", failingServiceName, params.logMessage)

	expectedRegexes := []string{`service_failure_failing_service\.\d{8}\.\d{6}\.0\.log`,
		`service_failure_failing_service\.\d{8}\.\d{6}\.0\.meta`}

	files, err := localCrash.WaitForCrashFiles(ctx, localCrash.SystemCrashDir, oldFiles, expectedRegexes)
	if err != nil {
		s.Error("Couldn't find expected files: ", err)
	}

	// Clean up files and check contents.
	for _, f := range files {
		if strings.HasSuffix(f, ".log") {
			contents, err := ioutil.ReadFile(f)
			if err != nil {
				s.Error("Couldn't read log file: ", err)
				continue
			}
			if !strings.Contains(string(contents), expectedLogMsg) {
				s.Errorf("Didn't find expected log contents: `%s`. Leaving %s for debugging", expectedLogMsg, f)
				continue
			}
		}
		if err := os.Remove(f); err != nil {
			s.Logf("Couldn't clean up %s: %v", f, err)
		}
	}
}
