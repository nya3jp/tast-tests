// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/chromecrash"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/testing"
)

// chromeCrashNotLoggedInParams contains the test parameters which are different between the various tests.
type chromeCrashNotLoggedInParams struct {
	ptype   chromecrash.ProcessType
	handler chromecrash.CrashHandler
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeCrashNotLoggedIn,
		Desc:         "Checks that Chrome writes crash dumps while not logged in",
		Contacts:     []string{"iby@chromium.org", "cros-monitoring-forensics@google.com"},
		SoftwareDeps: []string{"chrome", "metrics_consent"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:              "browser_breakpad",
			Val:               chromeCrashNotLoggedInParams{ptype: chromecrash.Browser, handler: chromecrash.Breakpad},
			ExtraSoftwareDeps: []string{"breakpad"},
		}, {
			Name: "browser_crashpad",
			Val:  chromeCrashNotLoggedInParams{ptype: chromecrash.Browser, handler: chromecrash.Crashpad},
		}, {
			Name:              "gpu_process_breakpad",
			Val:               chromeCrashNotLoggedInParams{ptype: chromecrash.GPUProcess, handler: chromecrash.Breakpad},
			ExtraSoftwareDeps: []string{"breakpad"},
		}, {
			Name: "gpu_process_crashpad",
			Val:  chromeCrashNotLoggedInParams{ptype: chromecrash.GPUProcess, handler: chromecrash.Crashpad},
		}, {
			Name:              "broker_breakpad",
			Val:               chromeCrashNotLoggedInParams{ptype: chromecrash.Broker, handler: chromecrash.Breakpad},
			ExtraSoftwareDeps: []string{"breakpad"},
		}, {
			Name: "broker_crashpad",
			Val:  chromeCrashNotLoggedInParams{ptype: chromecrash.Broker, handler: chromecrash.Crashpad},
		}},
	})
}

func ChromeCrashNotLoggedIn(ctx context.Context, s *testing.State) {
	params := s.Param().(chromeCrashNotLoggedInParams)
	ct, err := chromecrash.NewCrashTester(params.ptype, chromecrash.MetaFile)
	if err != nil {
		s.Fatal("NewCrashTester failed: ", err)
	}
	defer ct.Close()

	extraArgs := chromecrash.GetExtraArgs(params.handler)

	// We need to be logged in to set up consent, but then log out for the actual test.
	if err := func() error {
		cr, err := chrome.New(ctx, chrome.CrashNormalMode(), chrome.KeepState(), chrome.ExtraArgs(extraArgs...))
		if err != nil {
			return errors.Wrap(err, "chrome startup failed")
		}
		defer cr.Close(ctx)
		if err := crash.SetUpCrashTest(ctx, crash.WithConsent(cr)); err != nil {
			return errors.Wrap(err, "SetUpCrashTest failed")
		}
		return nil
	}(); err != nil {
		s.Fatal("Setting up crash test failed: ", err)
	}
	defer func() {
		if err := crash.TearDownCrashTest(); err != nil {
			s.Error("Failed to tear down crash test: ", err)
		}
	}()

	cr, err := chrome.New(ctx, chrome.NoLogin(), chrome.CrashNormalMode(), chrome.KeepState(), chrome.ExtraArgs(extraArgs...))
	if err != nil {
		s.Fatal("Chrome startup failed: ", err)
	}
	defer cr.Close(ctx)

	files, err := ct.KillAndGetCrashFiles(ctx)
	if err != nil {
		s.Fatalf("Couldn't kill Chrome %s process or get files: %v", params.ptype, err)
	}

	// Not-logged-in Chrome crashes get logged to /home/chronos/crash, not the
	// default /var/spool/crash, since it still runs as user "chronos".
	if err = chromecrash.FindCrashFilesIn(crash.LocalCrashDir, files); err != nil {
		s.Errorf("Crash files weren't written to /home/chronos/crash after crashing %s process: %v", params.ptype, err)
	}
}
