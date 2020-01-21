// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"os"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/local/bundles/cros/crash/sender"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SenderNoConsent,
		Desc: "Check that crashes are not uploaded without consent",
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

func SenderNoConsent(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	crashDir, err := sender.SetUp(ctx, cr)
	if err != nil {
		s.Fatal("Setup failed: ", err)
	}
	defer sender.TearDown()
	defer os.RemoveAll(crashDir)

	// Revoke the consent.
	if err := crash.SetConsent(ctx, cr, false); err != nil {
		s.Fatal("Failed to revoke consent: ", err)
	}

	const basename = "some_program.1.2.3"
	if _, err := sender.AddFakeMinidumpCrash(ctx, crashDir, basename); err != nil {
		s.Fatal("Failed to add a fake minidump crash: ", err)
	}

	got, err := sender.Run(ctx, crashDir)
	if err != nil {
		s.Fatal("Failed to run crash_sender: ", err)
	}
	var want []*sender.SendResult
	if diff := cmp.Diff(got, want); diff != "" {
		s.Log("Results mismatch (-got +want): ", diff)
		s.Errorf("crash_sender sent unexpected %d results; see logs for diff", len(got))
	}
}
