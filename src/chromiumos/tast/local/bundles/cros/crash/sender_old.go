// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"os"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"chromiumos/tast/local/bundles/cros/crash/sender"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SenderOld,
		Desc: "Check that old minidump crashes are uploaded",
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

func SenderOld(ctx context.Context, s *testing.State) {
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

	// Change timestamps to pretend that crash dumps were generated 25 hours ago.
	ts := time.Now().Add(-25 * time.Hour)
	for _, fp := range []string{exp.MetadataPath, exp.PayloadPath} {
		if err := os.Chtimes(fp, ts, ts); err != nil {
			s.Fatal("Failed to change file timestamp: ", err)
		}
	}

	got, err := sender.Run(ctx, crashDir)
	if err != nil {
		s.Fatal("Failed to run crash_sender: ", err)
	}
	want := []*sender.SendResult{{
		Success: true,
		Data:    *exp,
	}}
	if diff := cmp.Diff(got, want, cmpopts.IgnoreFields(sender.SendResult{}, "Schedule")); diff != "" {
		s.Log("Results mismatch (-got +want): ", diff)
		s.Errorf("crash_sender sent unexpected %d results; see logs for diff", len(got))
	}
}
