// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/crash"
	"chromiumos/tast/local/bundles/cros/ui/chromecrash"
	"chromiumos/tast/local/chrome"
	crash_lib "chromiumos/tast/local/crash"
	"chromiumos/tast/local/metrics"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ChromeCrashNotLoggedIn,
		Desc:     "Checks that Chrome writes crash dumps while not logged in",
		Contacts: []string{"iby@chromium.org", "chromeos-ui@google.com"},
		// chrome_internal because only official builds are even considered to have
		// metrics consent; see ChromeCrashReporterClient::GetCollectStatsConsent()
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Attr:         []string{"informational"},
		Data:         []string{chromecrash.TestCert},
	})
}

func ChromeCrashNotLoggedIn(ctx context.Context, s *testing.State) {
	if err := crash_lib.StartCrashTest(); err != nil {
		s.Fatal("StartCrashTest failed: ", err)
	}
	defer crash_lib.FinishCrashTest()

	err := metrics.SetConsent(ctx, s.DataPath(chromecrash.TestCert))
	if err != nil {
		s.Fatal("SetConsent failed: ", err)
	}

	cr, err := chrome.New(ctx, chrome.NoLogin(), chrome.CrashNormalMode(), chrome.KeepState())
	if err != nil {
		s.Fatal("Chrome startup failed: ", err)
	}
	defer cr.Close(ctx)

	files, err := chromecrash.KillAndGetCrashFiles(ctx)
	if err != nil {
		s.Fatal("Couldn't kill Chrome or get dumps: ", err)
	}

	// Not-logged-in Chrome crashes get logged to /home/chronos/crash, not the
	// default /var/spool/crash, since it still runs as user "chronos".
	if err = chromecrash.FindCrashFilesIn(crash.ChromeCrashDir, files); err != nil {
		s.Error("Crash files weren't written to /home/chronos/crash: ", err)
	}
}
