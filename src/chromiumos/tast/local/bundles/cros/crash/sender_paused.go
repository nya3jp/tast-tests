// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"os"

	"chromiumos/tast/local/bundles/cros/crash/sender"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SenderPaused,
		Desc: "Check that crash_sender is paused during crash tests",
		Contacts: []string{
			"mutexlox@chromium.org",
			"iby@chromium.org",
			"cros-monitoring-forensics@google.com",
			"nya@chromium.org", // ported to Tast
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "metrics_consent"},
		Pre:          chrome.LoggedIn(),
	})
}

func SenderPaused(ctx context.Context, s *testing.State) {
	crashDir, err := sender.SetUp(ctx, s.PreValue().(*chrome.Chrome))
	if err != nil {
		s.Fatal("Setup failed: ", err)
	}
	defer sender.TearDown()
	defer os.RemoveAll(crashDir)

	const basename = "some_program.1.2.3"
	if _, err := sender.AddFakeMinidumpCrash(ctx, crashDir, basename); err != nil {
		s.Fatal("Failed to add a fake minidump crash: ", err)
	}

	// Run crash_sender without ignoring the pause file. This should fail because
	// sender.SetUp creates the pause file.
	if _, err := sender.RunNoIgnorePauseFile(ctx, crashDir); err == nil {
		s.Fatal("crash_sender succeeded unexpectedly")
	}
	s.Log("crash_sender failed as expected")
}
