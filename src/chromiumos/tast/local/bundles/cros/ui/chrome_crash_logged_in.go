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
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeCrashLoggedIn,
		Desc:         "Checks that Chrome writes crash dumps while logged in",
		Contacts:     []string{"iby@chromium.org", "cros-monitoring-forensics@google.com"},
		SoftwareDeps: []string{"chrome", "metrics_consent"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:              "browser_breakpad",
			Val:               chromeCrashLoggedInParams{ptype: chromecrash.Browser, handler: chromecrash.Breakpad},
			ExtraSoftwareDeps: []string{"breakpad"},
		}, {
			Name: "browser_crashpad",
			Val:  chromeCrashLoggedInParams{ptype: chromecrash.Browser, handler: chromecrash.Crashpad},
		}, {
			Name:              "gpu_process_breakpad",
			Val:               chromeCrashLoggedInParams{ptype: chromecrash.GPUProcess, handler: chromecrash.Breakpad},
			ExtraSoftwareDeps: []string{"breakpad"},
		}, {
			Name: "gpu_process_crashpad",
			Val:  chromeCrashLoggedInParams{ptype: chromecrash.GPUProcess, handler: chromecrash.Crashpad},
		}, {
			Name:              "broker_breakpad",
			Val:               chromeCrashLoggedInParams{ptype: chromecrash.Broker, handler: chromecrash.Breakpad},
			ExtraSoftwareDeps: []string{"breakpad"},
		}, {
			Name: "broker_crashpad",
			Val:  chromeCrashLoggedInParams{ptype: chromecrash.Broker, handler: chromecrash.Crashpad},
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

	extraArgs := chromecrash.GetExtraArgs(params.handler)
	cr, err := chrome.New(ctx, chrome.CrashNormalMode(), chrome.KeepState(), chrome.ExtraArgs(extraArgs...))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	if err := crash.SetUpCrashTest(ctx, crash.WithConsent(cr)); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer func() {
		if err := crash.TearDownCrashTest(); err != nil {
			s.Error("Failed to tear down crash test: ", err)
		}
	}()

	files, err := ct.KillAndGetCrashFiles(ctx)
	if err != nil {
		s.Fatalf("Couldn't kill Chrome %s process or get files: %v", params.ptype, err)
	}

	if err = chromecrash.FindCrashFilesIn(chromecrash.CryptohomeCrashPattern, files); err != nil {
		s.Errorf("Crash files weren't written to cryptohome after crashing the %s process: %v", params.ptype, err)
	}
}
