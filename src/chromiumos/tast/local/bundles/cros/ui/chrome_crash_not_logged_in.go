// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"os/exec"
	"time"

	"chromiumos/tast/crash"
	"chromiumos/tast/local/bundles/cros/ui/chromecrash"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeCrashNotLoggedIn,
		Desc:         "Checks that Chrome writes crash dumps while not logged in",
		Contacts:     []string{"chromeos-ui@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"informational"},
	})
}

func ChromeCrashNotLoggedIn(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx,
		chrome.NoLogin(),
		chrome.CrashNormalMode(),
		chrome.ExtraArgs("--enable-crash-reporter-for-testing"))
	if err != nil {
		s.Fatal("Chrome startup failed: ", err)
	}
	defer cr.Close(ctx)

	testing.ContextLog(ctx, "Running metric_client to set up consent")
	err = exec.Command("/usr/bin/metrics_client", "-C").Run()
	if err != nil {
		s.Fatal("Error setting metrics consent: ", err)
	}

	// Sleep briefly as a speculative workaround for Chrome hangs that are occasionally seen
	// when this test sends SIGSEGV to Chrome soon after it starts: https://crbug.com/906690
	const delay = 3 * time.Second
	s.Logf("Sleeping %v to wait for Chrome to stabilize", delay)
	if err := testing.Sleep(ctx, delay); err != nil {
		s.Fatal("Timed out while waiting for Chrome startup: ", err)
	}

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
