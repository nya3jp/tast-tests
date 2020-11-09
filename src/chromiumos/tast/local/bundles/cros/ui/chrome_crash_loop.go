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
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

// testModeSuccessful is the special message that crash_sender logs if it
// successfully got the crash report. MUST MATCH kTestModeSuccessful in crash_sender_util.cc
const testModeSuccessful = "Test Mode: Logging success and exiting instead of actually uploading"

// chromeCrashLoopParams contains the test parameters which are different between the various tests.
type chromeCrashLoopParams struct {
	handler chromecrash.CrashHandler
	consent crash.ConsentType
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeCrashLoop,
		Desc:         "Checks that if Chrome crashes repeatedly when logged in, it does an immediate crash upload",
		Contacts:     []string{"iby@chromium.org", "cros-telemetry@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "memfd_create"},
		Params: []testing.Param{{
			Name: "breakpad",
			Val: chromeCrashLoopParams{
				handler: chromecrash.Breakpad,
				consent: crash.RealConsent,
			},
			ExtraSoftwareDeps: []string{"breakpad", "metrics_consent"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name: "breakpad_mock_consent",
			Val: chromeCrashLoopParams{
				handler: chromecrash.Breakpad,
				consent: crash.MockConsent,
			},
			ExtraSoftwareDeps: []string{"breakpad"},
		}, {
			Name: "crashpad",
			Val: chromeCrashLoopParams{
				handler: chromecrash.Crashpad,
				consent: crash.RealConsent,
			},
			ExtraSoftwareDeps: []string{"crashpad", "metrics_consent"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name: "crashpad_mock_consent",
			Val: chromeCrashLoopParams{
				handler: chromecrash.Crashpad,
				consent: crash.MockConsent,
			},
			ExtraSoftwareDeps: []string{"crashpad"},
			ExtraAttr:         []string{"informational"},
		}},
	})
}

// ChromeCrashLoop tests the crash-loop-mode crash reporter system. If Chrome
// crashes often enough to log the user out, a crash report will be generated
// and immediately sent to crash_sender; check that crash_sender correctly receives
// the crash report.
func ChromeCrashLoop(ctx context.Context, s *testing.State) {
	params := s.Param().(chromeCrashLoopParams)
	r, err := syslog.NewReader(ctx, syslog.Program("crash_sender"))
	if err != nil {
		s.Fatal("Could not start watching system message file: ", err)
	}
	defer r.Close()

	// Only Browser processes cause logouts and thus invoke the crash loop handler.
	ct, err := chromecrash.NewCrashTester(ctx, chromecrash.Browser, chromecrash.MetaFile)
	if err != nil {
		s.Fatal("NewCrashTester failed: ", err)
	}
	defer ct.Close()

	extraArgs := chromecrash.GetExtraArgs(params.handler, params.consent)
	cr, err := chrome.New(ctx, chrome.CrashNormalMode(), chrome.ExtraArgs(extraArgs...))
	if err != nil {
		s.Fatal("chrome.New() failed: ", err)
	}
	defer cr.Close(ctx)

	opt := crash.WithMockConsent()
	if params.consent == crash.RealConsent {
		opt = crash.WithConsent(cr)
	}
	if err := crash.SetUpCrashTest(ctx, opt); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest(ctx)

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

		dumps, err := ct.KillAndGetCrashFiles(ctx)
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
		if hasChromeDump {
			continue
		}

		testing.ContextLog(ctx, "No Chrome dumps found; this should be the crash-loop upload. Polling for success message")
		crashLoopModeUsed = true
		if _, err := r.Wait(ctx, time.Minute, func(e *syslog.Entry) bool {
			return strings.Contains(e.Content, testModeSuccessful)
		}); err != nil {
			s.Error("Test-successful message not found: ", err)
		} else {
			// Success! Don't keep trying to crash Chrome; session_manager restarts
			// after a crash loop, so we'll have lost all our test arguments (in
			// particular, the mock consent arguments for breakpad) and future
			// itertions of this loop will fail.
			break
		}
	}

	if !crashLoopModeUsed {
		s.Error("Crash-loop mode never used")
	}
}
