// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/local/bundles/cros/ui/chromecrash"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/metrics"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ChromeCrashLoggedIn,
		Desc:     "Checks that Chrome writes crash dumps while logged in",
		Contacts: []string{"iby@chromium.org", "chromeos-ui@google.com"},
		// chrome_internal because only official builds are even considered to have
		// metrics consent; see ChromeCrashReporterClient::GetCollectStatsConsent()
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Attr:         []string{"group:mainline"},
		Data:         []string{chromecrash.TestCert},
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
	s.Fatal("Deliberate error")

	if err := crash.SetUpCrashTest(); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest()

	err := metrics.SetConsent(ctx, s.DataPath(chromecrash.TestCert), true)
	if err != nil {
		s.Fatal("SetConsent failed: ", err)
	}

	cr, err := chrome.New(ctx, chrome.CrashNormalMode(), chrome.KeepState(), chrome.ExtraArgs(chromecrash.VModuleFlag))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	ptype := s.Param().(chromecrash.ProcessType)
	files, err := chromecrash.KillAndGetCrashFiles(ctx, ptype)
	if err != nil {
		s.Fatalf("Couldn't kill Chrome %s process or get files: %v", ptype, err)
	}

	if err = chromecrash.FindCrashFilesIn(chromecrash.CryptohomeCrashPattern, files); err != nil {
		s.Errorf("Crash files weren't written to cryptohome after crashing the %s process: %v", ptype, err)
	}
}
