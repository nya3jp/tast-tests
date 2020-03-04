// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
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
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "metrics_consent"},
		Pre:          chrome.LoggedIn(),
	})
}

func ServiceFailure(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	if err := crash.SetUpCrashTest(ctx, crash.WithConsent(cr)); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer func() {
		if err := crash.TearDownCrashTest(); err != nil {
			s.Error("Failed to tear down crash test: ", err)
		}
	}()

	// Restart anomaly detector to clear its --testonly-send-all flag at the end of execution.
	defer crash.RestartAnomalyDetector(ctx)

	for _, tt := range testParams {
		s.Run(ctx, tt.name, func(sctx context.Context, ss *testing.State) {
			failingServiceName := tt.servicePrefix + "failing-service"

			oldFiles, err := crash.GetCrashes(crash.SystemCrashDir)
			if err != nil {
				ss.Fatal("Failed to get original crashes: ", err)
			}

			// Restart anomaly detector to clear its cache of recently seen service
			// failures and ensure this one is logged.
			if err := crash.RestartAnomalyDetectorWithSendAll(sctx, true); err != nil {
				ss.Fatal("Failed to restart anomaly detector: ", err)
			}

			if err := upstart.StartJob(sctx, failingServiceName, fmt.Sprintf("%s=1", tt.envVar)); err != nil {
				// Ignore error; it's expected.
				// (upstart exits nonzero if a job fails in pre-start)
			}

			expectedLogMsg := fmt.Sprintf("%s %s", failingServiceName, tt.logMessage)

			// "base" gives the prefix of the expected crash files on disk, which
			// will use underscores rather than dashes.
			base := strings.Replace(tt.servicePrefix+"service_failure_"+failingServiceName, "-", "_", -1)
			logRegex := base + `\.\d{8}\.\d{6}\.0\.log`
			expectedRegexes := []string{logRegex, base + `\.\d{8}\.\d{6}\.0\.meta`}

			files, err := crash.WaitForCrashFiles(sctx, []string{crash.SystemCrashDir}, oldFiles, expectedRegexes)
			if err != nil {
				ss.Fatal("Couldn't find expected files: ", err)
			}
			defer crash.RemoveAllFiles(sctx, files)

			logs := files[logRegex]
			if len(logs) != 1 {
				ss.Error("Multiple service failures found. Leaving for debugging: ", strings.Join(logs, ", "))
				crash.MoveFilesToOut(sctx, ss.OutDir(), logs...)
			} else {
				contents, err := ioutil.ReadFile(logs[0])
				if err != nil {
					ss.Error("Couldn't read log file: ", err)
				}
				if !strings.Contains(string(contents), expectedLogMsg) {
					ss.Errorf("Didn't find expected log contents: `%s`. Leaving for debugging: %s", expectedLogMsg, logs[0])
					crash.MoveFilesToOut(sctx, ss.OutDir(), logs[0])
				}
			}
		})
	}
}
