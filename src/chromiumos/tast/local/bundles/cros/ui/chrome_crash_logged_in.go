// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/local/bundles/cros/ui/chromecrash"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/testing"
)

// chromeCrashLoggedInParams contains the test parameters which are different between the various tests.
type chromeCrashLoggedInParams struct {
	ptype   chromecrash.ProcessType
	handler chromecrash.CrashHandler
	consent crash.ConsentType
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeCrashLoggedIn,
		Desc:         "Checks that Chrome writes crash dumps while logged in",
		Contacts:     []string{"iby@chromium.org", "cros-monitoring-forensics@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name: "browser_breakpad",
			Val: chromeCrashLoggedInParams{
				ptype:   chromecrash.Browser,
				handler: chromecrash.Breakpad,
				consent: crash.RealConsent,
			},
			ExtraSoftwareDeps: []string{"breakpad", "metrics_consent"},
		}, {
			Name: "browser_breakpad_mock_consent",
			Val: chromeCrashLoggedInParams{
				ptype:   chromecrash.Browser,
				handler: chromecrash.Breakpad,
				consent: crash.MockConsent,
			},
			ExtraSoftwareDeps: []string{"breakpad"},
		}, {
			Name: "browser_crashpad",
			Val: chromeCrashLoggedInParams{
				ptype:   chromecrash.Browser,
				handler: chromecrash.Crashpad,
				consent: crash.RealConsent,
			},
			ExtraSoftwareDeps: []string{"metrics_consent"},
		}, {
			Name: "browser_crashpad_mock_consent",
			Val: chromeCrashLoggedInParams{
				ptype:   chromecrash.Browser,
				handler: chromecrash.Crashpad,
				consent: crash.MockConsent,
			},
		}, {
			Name: "gpu_process_breakpad",
			Val: chromeCrashLoggedInParams{
				ptype:   chromecrash.GPUProcess,
				handler: chromecrash.Breakpad,
				consent: crash.RealConsent,
			},
			ExtraSoftwareDeps: []string{"breakpad", "metrics_consent"},
		}, {
			Name: "gpu_process_breakpad_mock_consent",
			Val: chromeCrashLoggedInParams{
				ptype:   chromecrash.GPUProcess,
				handler: chromecrash.Breakpad,
				consent: crash.MockConsent,
			},
			ExtraSoftwareDeps: []string{"breakpad"},
		}, {
			Name: "gpu_process_crashpad",
			Val: chromeCrashLoggedInParams{
				ptype:   chromecrash.GPUProcess,
				handler: chromecrash.Crashpad,
				consent: crash.RealConsent,
			},
			ExtraSoftwareDeps: []string{"metrics_consent"},
		}, {
			Name: "gpu_process_crashpad_mock_consent",
			Val: chromeCrashLoggedInParams{
				ptype:   chromecrash.GPUProcess,
				handler: chromecrash.Crashpad,
				consent: crash.MockConsent,
			},
		}, {
			Name: "broker_breakpad_mock_consent",
			Val: chromeCrashLoggedInParams{
				ptype:   chromecrash.Broker,
				handler: chromecrash.Breakpad,
				consent: crash.MockConsent,
			},
			// If the gpu process is not sandboxed, it will not create a broker.
			ExtraSoftwareDeps: []string{"breakpad", "gpu_sandboxing"},
		}, {
			Name: "broker_crashpad_mock_consent",
			Val: chromeCrashLoggedInParams{
				ptype:   chromecrash.Broker,
				handler: chromecrash.Crashpad,
				consent: crash.MockConsent,
			},
			// If the gpu process is not sandboxed, it will not create a broker.
			ExtraSoftwareDeps: []string{"gpu_sandboxing"},
		}},
	})
}

func ChromeCrashLoggedIn(ctx context.Context, s *testing.State) {
	params := s.Param().(chromeCrashLoggedInParams)
	ct, err := chromecrash.NewCrashTester(params.ptype, chromecrash.MetaFile)
	if err != nil {
		s.Fatal("NewCrashTester failed: ", err)
	}
	defer ct.Close()

	extraArgs := chromecrash.GetExtraArgs(params.handler, params.consent)
	cr, err := chrome.New(ctx, chrome.CrashNormalMode(), chrome.KeepState(), chrome.ExtraArgs(extraArgs...))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	opt := crash.WithMockConsent()
	if params.consent == crash.RealConsent {
		opt = crash.WithConsent(cr)
	}
	if err := crash.SetUpCrashTest(ctx, opt); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest()

	files, err := ct.KillAndGetCrashFiles(ctx)
	if err != nil {
		s.Fatalf("Couldn't kill Chrome %s process or get files: %v", params.ptype, err)
	}

	if err = chromecrash.FindCrashFilesIn(chromecrash.CryptohomeCrashPattern, files); err != nil {
		s.Errorf("Crash files weren't written to cryptohome after crashing the %s process: %v", params.ptype, err)
	}
}
