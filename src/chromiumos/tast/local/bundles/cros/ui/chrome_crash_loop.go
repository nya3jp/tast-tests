// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/bundles/cros/ui/chromecrash"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/debugd"
	"chromiumos/tast/local/metrics"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

// testModeSuccessful is the special message that crash_sender logs if it
// successfully got the crash report. MUST MATCH kTestModeSuccessful in crash_sender_util.cc
const testModeSuccessful = "Test Mode: Logging success and exiting instead of actually uploading"

func init() {
	testing.AddTest(&testing.Test{
		Func:     ChromeCrashLoop,
		Desc:     "Checks that if Chrome crashes repeatedly when logged in, it does an immediate crash upload",
		Contacts: []string{"iby@chromium.org", "cros-monitoring-forensics@google.com"},
		Attr:     []string{"informational"},
		// chrome_internal because only official builds are even considered to have
		// metrics consent; see ChromeCrashReporterClient::GetCollectStatsConsent()
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Data:         []string{chromecrash.TestCert},
	})
}

// ChromeCrashLoop tests the crash-loop-mode crash reporter system. If Chrome
// crashes often enough to log the user out, a crash report will be generated
// and immediately sent to crash_sender; check that crash_sender correctly receives
// the crash report.
func ChromeCrashLoop(ctx context.Context, s *testing.State) {
	if err := crash.SetUpCrashTest(); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest()

	err := metrics.SetConsent(ctx, s.DataPath(chromecrash.TestCert))
	if err != nil {
		s.Fatal("SetConsent failed: ", err)
	}

	w, err := syslog.NewWatcher(syslog.MessageFile)
	if err != nil {
		s.Fatal("Could not start watching system message file: ", err)
	}
	defer w.Close()

	cr, err := chrome.New(ctx, chrome.CrashNormalMode(), chrome.KeepState())
	if err != nil {
		s.Fatal("chrome.New() failed: ", err)
	}
	defer cr.Close(ctx)

	d, err := debugd.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to debugd: ", err)
	}
	err = d.SetCrashSenderTestMode(ctx, true)
	if err != nil {
		s.Fatal("Failed to set crash sender test mode: ", err)
	}
	defer d.SetCrashSenderTestMode(ctx, false)

	// restartTries should match BrowserJob::kRestartTries in browser_job.cc.
	const restartTries = 5
	crashLoopModeUsed := false
	for i := 0; i < restartTries; i++ {
		s.Log("Killing chrome restart #", i)

		dumps, err := chromecrash.KillAndGetCrashFiles(ctx)
		if err != nil {
			s.Fatal("Couldn't kill Chrome or get dumps: ", err)
		}

		// Normally, crash_reporter will leave the crash dumps in the user crash
		// directory where KillAndGetCrashFiles() will see them. However, when
		// crash-loop mode is activated, the crash files are passed directly to
		// crash_sender without being written to disk, so KillAndGetCrashFiles() will
		// not find them. However, we must be careful to not be confused by other,
		// non-Chrome crashes that happen to occur during the test.
		hasChromeDump := false
		for _, dump := range dumps {
			if strings.Index(dump, "/chrome.") != -1 {
				hasChromeDump = true
				break
			}
		}
		if !hasChromeDump {
			testing.ContextLog(ctx, "No Chrome dumps found; this should be the crash-loop upload. Polling for success message")
			crashLoopModeUsed = true
			pollCtx, cancel := context.WithTimeout(ctx, time.Minute)
			defer cancel()
			if err := w.WaitForMessage(pollCtx, testModeSuccessful); err != nil {
				s.Error("Test-successful message not found: ", err)
			}
		}
	}

	if !crashLoopModeUsed {
		s.Error("Crash-loop mode never used")
	}
}
