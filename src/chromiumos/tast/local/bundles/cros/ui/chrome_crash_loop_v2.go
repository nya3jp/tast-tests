// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/chromecrash"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/debugd"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

// chromeCrashLoopParams contains the test parameters which are different between the various tests.
type chromeCrashLoopV2Params struct {
	handler chromecrash.CrashHandler
	consent crash.ConsentType
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeCrashLoopV2,
		Desc:         "Checks that if Chrome crashes repeatedly when logged in, it does an immediate crash upload",
		Contacts:     []string{"iby@chromium.org", "cros-telemetry@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "memfd_create"},
		Timeout:      2 * time.Minute,
		Params: []testing.Param{{
			Name: "breakpad",
			Val: chromeCrashLoopV2Params{
				handler: chromecrash.Breakpad,
				consent: crash.RealConsent,
			},
			ExtraSoftwareDeps: []string{"breakpad", "metrics_consent"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name: "breakpad_mock_consent",
			Val: chromeCrashLoopV2Params{
				handler: chromecrash.Breakpad,
				consent: crash.MockConsent,
			},
			ExtraSoftwareDeps: []string{"breakpad"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name: "crashpad",
			Val: chromeCrashLoopV2Params{
				handler: chromecrash.Crashpad,
				consent: crash.RealConsent,
			},
			ExtraSoftwareDeps: []string{"crashpad", "metrics_consent"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name: "crashpad_mock_consent",
			Val: chromeCrashLoopV2Params{
				handler: chromecrash.Crashpad,
				consent: crash.MockConsent,
			},
			ExtraSoftwareDeps: []string{"crashpad"},
			ExtraAttr:         []string{"informational"},
		}},
	})
}

// ChromeCrashLoopV2 tests the crash-loop-mode crash reporter system. If Chrome
// crashes often enough to log the user out, a crash report will be generated
// and immediately sent to crash_sender; check that crash_sender correctly receives
// the crash report. This the V2 version, a rewrite to avoid some of the intractable
// problems with the previous version.
// TODO(b/202795944): Remove old version once this is out of "informational".
func ChromeCrashLoopV2(ctx context.Context, s *testing.State) {
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

	// Give enough time for the debugd test mode switch back & other cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	d, err := debugd.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to debugd: ", err)
	}
	if err = d.SetCrashSenderTestMode(ctx, true); err != nil {
		s.Fatal("Failed to set crash sender test mode: ", err)
	}
	defer d.SetCrashSenderTestMode(cleanupCtx, false)

	params := s.Param().(chromeCrashLoopV2Params)
	opt := crash.WithMockConsent()
	extraArgs := append(chromecrash.GetExtraArgs(params.handler, params.consent))
	// For real consent, we need to first open a non-crashing version of Chrome
	// long enough to log in and set up consent.
	if params.consent == crash.RealConsent {
		cr, err := chrome.New(ctx, chrome.CrashNormalMode(), chrome.ExtraArgs(extraArgs...))
		if err != nil {
			s.Fatal("chrome.New() failed: ", err)
		}
		defer cr.Close(cleanupCtx)
		opt = crash.WithConsent(cr)
	}
	if err := crash.SetUpCrashTest(ctx, opt); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest(cleanupCtx)

	// Now that we are logged in and consent is set up, tell session manager to
	// crash-loop the browser. Don't use chrome.New; the browser won't be up
	// long enough for chrome.New to connect to it.
	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		s.Fatal("NewSessionManager failed: ", err)
	}

	// Note that session_manager always automatically restarts Chrome when it
	// crashes (until the too-many-restarts count is reached, and session_manager
	// logs out and exits). However, after the call to EnableChromeTesting(), it
	// will always restart it with with testing flag, so Chrome will always just
	// crash again. Once the too-many-restarts count is reached, session_manager
	// will exit and forget about the testing flags, so we don't need to do
	// anything to undo the EnableChromeTesting call.
	testing.ContextLog(ctx, "Switching Chrome to crash loop & waiting for test-success file")
	extraArgs = append(extraArgs, "--crash-test")
	if _, err := sm.EnableChromeTesting(ctx, true, extraArgs, []string{}); err != nil {
		s.Fatal("Start-crash-looping-Chrome call failed: ", err)
	}
	if err := testing.Poll(ctx, func(c context.Context) error {
		_, err := os.Stat(chromecrash.TestModeSuccessfulFile)
		return err
	}, nil); err != nil {
		s.Error("Test-successful file not found: ", err)
	}

	// Clean up success file at the end.
	os.Remove(chromecrash.TestModeSuccessfulFile)
}
