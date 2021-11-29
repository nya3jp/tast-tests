// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"os"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/chromecrash"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/debugd"
	"chromiumos/tast/testing"
)

// chromeCrashLoopParams contains the test parameters which are different between the various tests.
type chromeCrashLoopParams struct {
	handler chromecrash.CrashHandler
	consent crash.ConsentType
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeCrashLoop,
		LacrosStatus: testing.LacrosVariantUnneeded,
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
		}},
	})
}

// ChromeCrashLoop tests the crash-loop-mode crash reporter system. If Chrome
// crashes often enough to log the user out, a crash report will be generated
// and immediately sent to crash_sender; check that crash_sender correctly receives
// the crash report.
// DEPRECATED: This test has persistent issues where unrelated Chrome crashes
// make the test seem flaky. See ChromeCrashLoopV2 for a rewrite that removes
// this problem. See b/202795944 for more.
// TODO(b/202795944): Remove this version once ChromeCrashLoopV2 is out of "informational".
func ChromeCrashLoop(ctx context.Context, s *testing.State) {
	params := s.Param().(chromeCrashLoopParams)

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

	// Clean up success file at the end.
	defer os.Remove(chromecrash.TestModeSuccessfulFile)

	// Ensure success file isn't left over from previous test. By default, we
	// don't print out the error here, nor do we fail the test because of errors
	// from os.Remove. In the vast majority of cases, the success file won't exist
	// and we'll get an error about trying to remove an non-existent file. (It's
	// fine that the success file doesn't exist; that's what we expect.) Instead
	// of checking the error on this line, we just check on the next line that the
	// file doesn't exist. As long as the file doesn't exist after this line,
	// we're OK with it either being deleted here *or* it not having existed in
	// the first place. However, we do save the error object -- if the file still
	// exists, we want to add it into the error message saying that the remove
	// failed.
	removeErr := os.Remove(chromecrash.TestModeSuccessfulFile)
	if _, err := os.Stat(chromecrash.TestModeSuccessfulFile); err == nil {
		s.Fatal(chromecrash.TestModeSuccessfulFile, " still exists. Remove failed with: ", removeErr)
	} else if !errors.Is(err, os.ErrNotExist) {
		s.Fatal("Could not stat ", chromecrash.TestModeSuccessfulFile, ": ", err)
	}

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

		testing.ContextLog(ctx, "No Chrome dumps found; this should be the crash-loop upload. Polling for success file")
		crashLoopModeUsed = true
		if err := testing.Poll(ctx, func(c context.Context) error {
			_, err := os.Stat(chromecrash.TestModeSuccessfulFile)
			return err
		}, nil); err != nil {
			s.Error("Test-successful file not found: ", err)
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
