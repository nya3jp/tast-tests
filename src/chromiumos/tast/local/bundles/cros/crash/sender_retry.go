// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"os"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"chromiumos/tast/local/bundles/cros/crash/sender"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SenderRetry,
		Desc: "Check that crash_sender failures are retried",
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

func SenderRetry(ctx context.Context, s *testing.State) {
	crashDir, err := sender.SetUp(ctx, s.PreValue().(*chrome.Chrome))
	if err != nil {
		s.Fatal("Setup failed: ", err)
	}
	defer sender.TearDown()
	defer os.RemoveAll(crashDir)

	const basename = "some_program.1.2.3"
	exp, err := sender.AddFakeMinidumpCrash(ctx, crashDir, basename)
	if err != nil {
		s.Fatal("Failed to add a fake minidump crash: ", err)
	}

	// Simulate a failed upload.
	if err := sender.EnableMock(false); err != nil {
		s.Fatal("Failed to set mock mode: ", err)
	}

	got, err := sender.Run(ctx, crashDir)
	if err != nil {
		s.Fatal("Failed to run crash_sender: ", err)
	}
	// On simulating a failed upload, crash_sender sets the image type to "mock-fail".
	// See GetImageType in crash_sender_util.cc.
	exp.ImageType = "mock-fail"
	want := []*sender.SendResult{{
		Success: false,
		Data:    *exp,
	}}
	if diff := cmp.Diff(got, want, cmpopts.IgnoreFields(sender.SendResult{}, "Schedule")); diff != "" {
		s.Log("Results mismatch (-got +want): ", diff)
		s.Errorf("crash_sender sent unexpected %d results; see logs for diff", len(got))
	}

	// Simulate a successful upload.
	if err := sender.EnableMock(true); err != nil {
		s.Fatal("Failed to set mock mode: ", err)
	}

	got, err = sender.Run(ctx, crashDir)
	if err != nil {
		s.Fatal("Failed to run crash_sender: ", err)
	}
	exp.ImageType = ""
	want = []*sender.SendResult{{
		Success: true,
		Data:    *exp,
	}}
	if diff := cmp.Diff(got, want, cmpopts.IgnoreFields(sender.SendResult{}, "Schedule")); diff != "" {
		s.Log("Results mismatch (-got +want): ", diff)
		s.Errorf("crash_sender sent unexpected %d results; see logs for diff", len(got))
	}
}
