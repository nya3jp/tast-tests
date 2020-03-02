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
		Func: SenderKcrash,
		Desc: "Check that kernel crash dumps are uploaded",
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

func SenderKcrash(ctx context.Context, s *testing.State) {
	if err := crash.SetUpCrashTest(ctx, crash.WithConsent(s.PreValue().(*chrome.Chrome))); err != nil {
		s.Fatal("Setup failed: ", err)
	}
	defer crash.TearDownCrashTest()

	const basename = "some_kernel.1.2.3"
	exp, err := crash.AddFakeKernelCrash(ctx, basename)
	if err != nil {
		s.Fatal("Failed to add a fake kernel crash: ", err)
	}

	got, err := crash.RunSender(ctx)
	if err != nil {
		s.Fatal("Failed to run crash_sender: ", err)
	}
	want := []*crash.SendResult{{
		Success: true,
		Data:    *exp,
	}}
	if diff := cmp.Diff(got, want, cmpopts.IgnoreFields(crash.SendResult{}, "Schedule")); diff != "" {
		s.Log("Results mismatch (-got +want): ", diff)
		s.Errorf("crash_sender sent unexpected %d results; see logs for diff", len(got))
	}
}
