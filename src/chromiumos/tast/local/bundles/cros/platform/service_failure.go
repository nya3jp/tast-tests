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
	defer crash.TearDownCrashTest()

	// Restart anomaly detector to clear its --testonly-send-all flag at the end of execution.
	defer crash.RestartAnomalyDetector(ctx)

	for _, tt := range testParams {
		//  This is in a func so that we can use `defer` to run cleanup at the end of each iteration.
		// TODO(mutexlox): Avoid repetition of the tt.name parameter by using testing.State.Run
		func() {
			failingServiceName := tt.servicePrefix + "failing-service"

			oldFiles, err := crash.GetCrashes(crash.SystemCrashDir)
			if err != nil {
				s.Fatalf("%s: failed to get original crashes: %v", tt.name, err)
			}

			// Restart anomaly detector to clear its cache of recently seen service
			// failures and ensure this one is logged.
			if err := crash.RestartAnomalyDetectorWithSendAll(ctx, true); err != nil {
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
			logRegex := base + `\.\d{8}\.\d{6}\.0\.log`
			expectedRegexes := []string{logRegex, base + `\.\d{8}\.\d{6}\.0\.meta`}

			files, err := crash.WaitForCrashFiles(ctx, []string{crash.SystemCrashDir}, oldFiles, expectedRegexes)
			if err != nil {
				s.Errorf("%s: couldn't find expected files: %v", tt.name, err)
				// Skip to next iteration
				return
			}
			defer crash.RemoveAllFiles(ctx, files)

			logs := files[logRegex]
			if len(logs) != 1 {
				s.Errorf("Multiple service failures found. Leaving for debugging: %s", strings.Join(logs, ", "))
				crash.MoveFilesToOut(ctx, s.OutDir(), logs...)
			} else {
				contents, err := ioutil.ReadFile(logs[0])
				if err != nil {
					s.Errorf("%s: couldn't read log file: %v", tt.name, err)
				}
				if !strings.Contains(string(contents), expectedLogMsg) {
					s.Errorf("%s: didn't find expected log contents: `%s`. Leaving %s for debugging", tt.name, expectedLogMsg, logs[0])
					crash.MoveFilesToOut(ctx, s.OutDir(), logs[0])
				}
			}
		}()
	}
}
