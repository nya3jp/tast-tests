// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"os"

	"golang.org/x/sys/unix"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SenderLock,
		Desc: "Check that only one crash_sender runs at a time",
		Contacts: []string{
			"mutexlox@chromium.org",
			"iby@chromium.org",
			"cros-monitoring-forensics@google.com",
			"nya@chromium.org", // ported to Tast
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "metrics_consent"},
		Pre:          crash.ChromePreWithVerboseConsent(),
	})
}

func SenderLock(ctx context.Context, s *testing.State) {
	if err := crash.SetUpCrashTest(ctx, crash.WithConsent(s.PreValue().(*chrome.Chrome))); err != nil {
		s.Fatal("Setup failed: ", err)
	}
	defer crash.TearDownCrashTest()

	const basename = "some_program.1.2.3"
	if _, err := crash.AddFakeMinidumpCrash(ctx, basename); err != nil {
		s.Fatal("Failed to add a fake minidump crash: ", err)
	}

	// Obtain the crash_sender lock. This should prevent crash_sender from running.
	const lockPath = "/run/lock/crash_sender"
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		s.Fatal("Failed to obtain crash_sender lock: ", err)
	}
	defer f.Close()
	if err := unix.FcntlFlock(f.Fd(), unix.F_SETLK, &unix.Flock_t{Type: unix.F_WRLCK}); err != nil {
		s.Fatal("Failed to obtain crash_sender lock: ", err)
	}

	if _, err := crash.RunSender(ctx); err == nil {
		s.Fatal("crash_sender succeeded unexpectedly")
	}
	s.Log("crash_sender failed as expected")
}
