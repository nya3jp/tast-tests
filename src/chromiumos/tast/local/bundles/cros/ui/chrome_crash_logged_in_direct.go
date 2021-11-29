// Copyright 2019 The Chromium OS Authors. All rights reserved.
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

type chromeCrashLoggedInDirectParams struct {
	fileType chromecrash.CrashFileType
	handler  chromecrash.CrashHandler
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeCrashLoggedInDirect,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks that Chrome writes crash dumps while logged in; old version that does not invoke crash_reporter",
		Contacts:     []string{"iby@chromium.org", "chromeos-ui@google.com", "cros-telemetry@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Name: "breakpad",
			Val: chromeCrashLoggedInDirectParams{
				handler:  chromecrash.Breakpad,
				fileType: chromecrash.BreakpadDmp,
			},
			ExtraSoftwareDeps: []string{"breakpad"},
		}, {
			Name: "crashpad",
			Val: chromeCrashLoggedInDirectParams{
				handler:  chromecrash.Crashpad,
				fileType: chromecrash.MetaFile,
			},
			ExtraSoftwareDeps: []string{"crashpad"},
		}},
	})
}

// ChromeCrashLoggedInDirect tests that Chrome crashes that happen during tast
// tests are properly captured (that is, during tast tests which are testing
// something other than the crash system).
//
// The other Chrome crash tests cover cases that we expect to occur on end-user
// machines, by simulating user consent. This test covers the tast case, where
// we bypass consent by telling the crash system that we are in a test
// environment. In particular, breakpad goes through a very different code path
// which doesn't involve crash_reporter at all, and we want that to keep working.
//
// Note: The name is a misnomer; the 'Direct' refers to the old days when both
// breakpad and crashpad bypassed crash_reporter and wrote the crashes directly
// onto disk during this test. Crashpad no longer does that; the test should be
// named "TastMode". TODO(https://crbug.com/1201467): Rename to
// ChromeCrashLoggedInTastMode
func ChromeCrashLoggedInDirect(ctx context.Context, s *testing.State) {
	params := s.Param().(chromeCrashLoggedInDirectParams)
	ct, err := chromecrash.NewCrashTester(ctx, chromecrash.GPUProcess, params.fileType)
	if err != nil {
		s.Fatal("NewCrashTester failed: ", err)
	}
	defer ct.Close()

	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromecrash.GetExtraArgs(params.handler, crash.MockConsent)...))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	// We use crash.DevImage() here because this test still uses the testing
	// command-line flags on crash_reporter to bypass metrics consent and such.
	// Those command-line flags only work if the crash-test-in-progress does not
	// exist.
	if err := crash.SetUpCrashTest(ctx, crash.DevImage()); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest(ctx)

	var files []string
	if files, err = ct.KillAndGetCrashFiles(ctx); err != nil {
		s.Fatal("Couldn't kill Chrome or get dumps: ", err)
	}

	// Check we got the expected files. Tast tests always write to
	// /home/chronos/crash, not to cryptohome, so that the framework can retrieve
	// the crashes afterwards.
	if params.fileType == chromecrash.MetaFile {
		if err := chromecrash.FindCrashFilesIn(crash.LocalCrashDir, files); err != nil {
			s.Errorf("Crash files weren't written to %s after crashing process: %v", crash.LocalCrashDir, err)
		}
	} else {
		if err := chromecrash.FindBreakpadDmpFilesIn(crash.LocalCrashDir, files); err != nil {
			s.Errorf(".dmp files weren't written to %s after crashing process: %v", crash.LocalCrashDir, err)
		}
	}

}
