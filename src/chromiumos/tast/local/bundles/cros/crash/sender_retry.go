// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
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
	if err := crash.SetUpCrashTest(ctx, crash.WithConsent(s.PreValue().(*chrome.Chrome))); err != nil {
		s.Fatal("Setup failed: ", err)
	}
	defer func() {
		if err := crash.TearDownCrashTest(); err != nil {
			s.Error("Failed to tear down crash test: ", err)
		}
	}()

	const basename = "some_program.1.2.3"
	exp, err := crash.AddFakeMinidumpCrash(ctx, basename)
	if err != nil {
		s.Fatal("Failed to add a fake minidump crash: ", err)
	}

	// Simulate a failed upload.
	if err := crash.EnableMockSending(false); err != nil {
		s.Fatal("Failed to set mock mode: ", err)
	}

	got, err := crash.RunSender(ctx)
	if err != nil {
		s.Fatal("Failed to run crash_sender: ", err)
	}
	// On simulating a failed upload, crash_sender sets the image type to "mock-fail".
	// See GetImageType in crash_sender_util.cc.
	exp.ImageType = "mock-fail"
	want := []*crash.SendResult{{
		Success: false,
		Data:    *exp,
	}}
	if diff := cmp.Diff(got, want, cmpopts.IgnoreFields(crash.SendResult{}, "Schedule")); diff != "" {
		s.Log("Results mismatch (-got +want): ", diff)
		s.Errorf("crash_sender sent unexpected %d results; see logs for diff", len(got))
	}

	// Simulate a successful upload.
	if err := crash.EnableMockSending(true); err != nil {
		s.Fatal("Failed to set mock mode: ", err)
	}

	got, err = crash.RunSender(ctx)
	if err != nil {
		s.Fatal("Failed to run crash_sender: ", err)
	}
	exp.ImageType = ""
	want = []*crash.SendResult{{
		Success: true,
		Data:    *exp,
	}}
	if diff := cmp.Diff(got, want, cmpopts.IgnoreFields(crash.SendResult{}, "Schedule")); diff != "" {
		s.Log("Results mismatch (-got +want): ", diff)
		s.Errorf("crash_sender sent unexpected %d results; see logs for diff", len(got))
	}
}
