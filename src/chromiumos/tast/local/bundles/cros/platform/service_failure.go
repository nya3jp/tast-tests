// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"chromiumos/tast/crash"
	"chromiumos/tast/local/chrome"
	localCrash "chromiumos/tast/local/crash"
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
		Func:         ServiceFailure,
		Desc:         "Verify service failures are logged as expected",
		Contacts:     []string{"mutexlox@google.com", "cros-monitoring-forensics@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Pre:          chrome.LoggedIn(),
	})
}

func ServiceFailure(ctx context.Context, s *testing.State) {
	if err := localCrash.SetUpCrashTest(); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer localCrash.TearDownCrashTest()

	// Restart anomaly detector to clear its --testonly-send-all flag at the end of execution.
	defer localCrash.RestartAnomalyDetector(ctx)

	for _, tt := range testParams {
		// TODO(https://crbug.com/1007138): Avoid repetition of the tt.name parameter.
		failingServiceName := tt.servicePrefix + "failing-service"

		oldFiles, err := crash.GetCrashes(localCrash.SystemCrashDir)
		if err != nil {
			s.Fatalf("%s: failed to get original crashes: %v", tt.name, err)
		}

		// Restart anomaly detector to clear its cache of recently seen service
		// failures and ensure this one is logged.
		if err := localCrash.RestartAnomalyDetectorWithSendAll(ctx, true); err != nil {
			s.Fatalf("%s: failed to restart anomaly detector: %v", tt.name, err)
		}

		if err := upstart.StartJob(ctx, failingServiceName, fmt.Sprintf("%s=1", tt.envVar)); err != nil {
			// Ignore error; it's expected.
			// (upstart exits nonzero if a job fails in pre-start)
		}

		expectedLogMsg := fmt.Sprintf("%s %s", failingServiceName, tt.logMessage)

		// "base" gives the prefix of the expected crash files on disk, which
		// will use underscores rather than dashes.
		base := strings.Replace(tt.servicePrefix+"service_failure_"+failingServiceName, "-", "_", -1)
		relog := base + `\.\d{8}\.\d{6}\.0\.log`
		remeta := base + `\.\d{8}\.\d{6}\.0\.meta`
		expectedRegexes := []string{relog, remeta}

		files, err := localCrash.WaitForCrashFiles(ctx, []string{localCrash.SystemCrashDir}, oldFiles, expectedRegexes)
		if err != nil {
			s.Fatalf("%s: couldn't find expected files: %v", tt.name, err)
		}
		defer localCrash.CleanupCrashFiles(files)

		// Check log contents.
		if len(files[relog]) != 1 {
			s.Fatalf("thare are multiple log files: %s", strings.Join(files[relog], ", "))
		}
		log := files[relog][0]
		contents, err := ioutil.ReadFile(log)
		if err != nil {
			s.Fatalf("%s: couldn't read log file: %v", tt.name, err)
		}
		if !strings.Contains(string(contents), expectedLogMsg) {
			s.Errorf("%s: didn't find expected log contents: `%s`. Leaving %s for debugging", tt.name, expectedLogMsg, log)
		}
	}
}
