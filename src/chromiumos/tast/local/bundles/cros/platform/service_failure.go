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
	name          string
	envVar        string
	logMessage    string
	servicePrefix string
}

var testParams = []failureParams{
	{
		name:          "pre start",
		envVar:        "PRE_START_EXIT",
		logMessage:    "pre-start process",
		servicePrefix: "",
	},
	{
		name:          "main",
		envVar:        "MAIN_EXIT",
		logMessage:    "main process",
		servicePrefix: "",
	},
	{
		name:          "post start",
		envVar:        "POST_START_EXIT",
		logMessage:    "post-start process",
		servicePrefix: "",
	},
	{
		name:          "arc pre start",
		envVar:        "PRE_START_EXIT",
		logMessage:    "pre-start process",
		servicePrefix: "arc-",
	},
	{
		name:          "arc main",
		envVar:        "MAIN_EXIT",
		logMessage:    "main process",
		servicePrefix: "arc-",
	},
	{
		name:          "arc post exit",
		envVar:        "POST_START_EXIT",
		logMessage:    "post-start process",
		servicePrefix: "arc-",
	},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     ServiceFailure,
		Desc:     "Verify service failures are logged as expected",
		Contacts: []string{"mutexlox@google.com", "cros-monitoring-forensics@chromium.org"},
		Attr:     []string{"informational"},
		Data:     []string{platformCrash.TestCert},
	})
}

func ServiceFailure(ctx context.Context, s *testing.State) {
	if err := localCrash.SetUpCrashTest(); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer localCrash.TearDownCrashTest()

	if err := metrics.SetConsent(ctx, s.DataPath(platformCrash.TestCert), true); err != nil {
		s.Fatal("Failed to set consent: ", err)
	}

	for _, tt := range testParams {
		failingServiceName := tt.servicePrefix + "failing-service"

		oldFiles, err := crash.GetCrashes(localCrash.SystemCrashDir)
		if err != nil {
			s.Fatal("Failed to get original crashes: ", err)
		}

		// Restart anomaly detector to clear its cache of recently seen service
		// failures and ensure this one is logged.
		if err := localCrash.RestartAnomalyDetector(ctx); err != nil {
			s.Fatal("Failed to restart anomaly detector: ", err)
		}

		if err := upstart.StartJob(ctx, failingServiceName, fmt.Sprintf("%s=1", tt.envVar)); err != nil {
			// Ignore error; it's expected.
			// (upstart exits nonzero if a job fails in pre-start)
		}

		expectedLogMsg := fmt.Sprintf("%s %s", failingServiceName, tt.logMessage)

		// "base" gives the prefix of the expected crash files on disk, which
		// will use underscores rather than dashes.
		base := strings.Replace(tt.servicePrefix+"service_failure_"+failingServiceName, "-", "_", -1)
		expectedRegexes := []string{base + `\.\d{8}\.\d{6}\.0\.log`, base + `\.\d{8}\.\d{6}\.0\.meta`}

		files, err := localCrash.WaitForCrashFiles(ctx, localCrash.SystemCrashDir, oldFiles, expectedRegexes)
		if err != nil {
			s.Errorf("%s: Couldn't find expected files: %v", tt.name, err)
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
}
