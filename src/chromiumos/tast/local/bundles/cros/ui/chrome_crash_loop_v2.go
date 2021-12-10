// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/ui/chromecrash"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/debugd"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/syslog"
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
// problems with the previous version. TODO(b/202795944) Remove old version
func ChromeCrashLoopV2(ctx context.Context, s *testing.State) {
	// Give enough time for the debugd test mode switch back & other cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	params := s.Param().(chromeCrashLoopV2Params)
	r, err := syslog.NewReader(ctx, syslog.Program(syslog.CrashSender))
	if err != nil {
		s.Fatal("Could not start watching system message file: ", err)
	}
	defer r.Close()

	d, err := debugd.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to debugd: ", err)
	}
	err = d.SetCrashSenderTestMode(ctx, true)
	if err != nil {
		s.Fatal("Failed to set crash sender test mode: ", err)
	}
	defer d.SetCrashSenderTestMode(cleanupCtx, false)

	// Note that we need to first open a non-crashing version of Chrome long
	// enough to log in and set up consent.
	extraArgs := append(chromecrash.GetExtraArgs(params.handler, params.consent))
	cr, err := chrome.New(ctx, chrome.CrashNormalMode(), chrome.ExtraArgs(extraArgs...))
	if err != nil {
		s.Fatal("chrome.New() failed: ", err)
	}
	defer cr.Close(cleanupCtx)

	opt := crash.WithMockConsent()
	if params.consent == crash.RealConsent {
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

	testing.ContextLog(ctx, "Switching Chrome to crash loop & waiting for test-success message")
	extraArgs = append(extraArgs, "--crash-test")
	if _, err := sm.EnableChromeTesting(ctx, true, extraArgs, []string{}); err != nil {
		s.Fatal("Start-crash-looping-Chrome call failed: ", err)
	}

	if _, err := r.Wait(ctx, time.Minute, func(e *syslog.Entry) bool {
		return strings.Contains(e.Content, chromecrash.CrashSenderTestModeSuccessful)
	}); err != nil {
		s.Error("Test-successful message not found: ", err)
	}
}
