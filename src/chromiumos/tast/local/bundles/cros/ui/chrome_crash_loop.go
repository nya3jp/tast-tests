// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/chromecrash"
	"chromiumos/tast/local/chrome"
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

func ChromeCrashLoop(ctx context.Context, s *testing.State) {
	err := metrics.SetConsent(ctx, s.DataPath(chromecrash.TestCert))
	if err != nil {
		s.Fatal("SetConsent failed: ", err)
	}

	d, err := debugd.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to debugd: ", err)
	}
	err = d.SetCrashSenderTestMode(ctx, true)
	if err != nil {
		s.Fatal("Failed to set crash sender test mode: ", err)
	}
	defer d.SetCrashSenderTestMode(ctx, false)

	w, err := syslog.NewWatcher(syslog.MessageFile)
	if err != nil {
		s.Fatal("Could not start watching system message file: ", err)
	}
	defer w.Close()

	cr, err := chrome.New(ctx, chrome.CrashNormalMode(), chrome.KeepState())
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	// restartTries should match BrowserJob::kRestartTries in browser_job.cc.
	const restartTries = 5
	crashLoopModeUsed := false
	for i := 0; i < restartTries; i++ {
		s.Log("Killing chrome restart #", i)
		dumps, err := chromecrash.KillAndGetCrashFiles(ctx)
		if err != nil {
			s.Fatal("Couldn't kill Chrome or get dumps: ", err)
		}

		if len(dumps) == 0 {
			testing.ContextLog(ctx, "No dumps found; this should be the crash-loop upload. Polling for success message")
			crashLoopModeUsed = true
			if err = testing.Poll(ctx, func(ctx context.Context) error {
				if hasMessage, err := w.HasMessage(testModeSuccessful); err != nil {
					return testing.PollBreak(err)
				} else if !hasMessage {
					return errors.New("test success message not found in /var/log/messages")
				}
				return nil
			}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
				s.Error("Test-successful message not found: ", err)
			}
		}
	}

	if !crashLoopModeUsed {
		s.Error("Crash-loop mode never used")
	}
}
