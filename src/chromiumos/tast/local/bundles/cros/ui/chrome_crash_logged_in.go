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

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeCrashLoggedIn,
		Desc:         "Checks that Chrome writes crash dumps while logged in",
		Contacts:     []string{"iby@chromium.org", "chromeos-ui@google.com"},
		SoftwareDeps: []string{"chrome", "metrics_consent"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name: "browser",
			Val:  chromecrash.Browser,
		}, {
			Name: "gpu_process",
			Val:  chromecrash.GPUProcess,
		}, {
			Name: "broker",
			Val:  chromecrash.Broker,
		}},
	})
}

func ChromeCrashLoggedIn(ctx context.Context, s *testing.State) {
	ptype := s.Param().(chromecrash.ProcessType)
	ct, err := chromecrash.NewCrashTester(ptype, chromecrash.MetaFile)
	if err != nil {
		s.Fatal("NewCrashTester failed: ", err)
	}
	defer ct.Close()

	cr, err := chrome.New(ctx, chrome.CrashNormalMode(), chrome.KeepState(), chrome.ExtraArgs(chromecrash.VModuleFlag))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	if err := crash.SetUpCrashTest(ctx, crash.WithConsent(cr)); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest()

	files, err := ct.KillAndGetCrashFiles(ctx)
	if err != nil {
		s.Fatalf("Couldn't kill Chrome %s process or get files: %v", ptype, err)
	}

	if err = chromecrash.FindCrashFilesIn(chromecrash.CryptohomeCrashPattern, files); err != nil {
		s.Errorf("Crash files weren't written to cryptohome after crashing the %s process: %v", ptype, err)
	}
}
