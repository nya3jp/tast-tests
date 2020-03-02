// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SenderRateLimit,
		Desc: "Check that crash_sender enforces the daily limit of crash report upload",
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

func SenderRateLimit(ctx context.Context, s *testing.State) {
	// Expected range of daily limit of crash uploads.
	const (
		minRuns = 8
		maxRuns = 100
	)

	cr := s.PreValue().(*chrome.Chrome)
	if err := crash.SetUpCrashTest(ctx, crash.WithConsent(cr)); err != nil {
		s.Fatal("Setup failed: ", err)
	}
	defer crash.TearDownCrashTest()

	// Continue uploading crash reports until we hit the rate limit.
	runs := 0
	for {
		if runs >= maxRuns {
			s.Fatalf("crash_sender did not hit the rate limit after %d runs", runs)
		}

		s.Logf("Iteration #%d", runs)

		basename := fmt.Sprintf("some_program.0.0.%d", runs)
		if _, err := crash.AddFakeMinidumpCrash(ctx, basename); err != nil {
			s.Fatal("Failed to add a fake minidump crash: ", err)
		}

		got, err := crash.RunSender(ctx)
		if err != nil {
			s.Fatal("Failed to run crash_sender: ", err)
		}

		if len(got) != 1 {
			s.Fatalf("Unexpected number of results: got %d, want 1", len(got))
		}
		if !got[0].Success {
			break
		}

		runs++

		rs, err := crash.ListSendRecords()
		if err != nil {
			s.Fatal("Failed to get send records: ", err)
		}
		if len(rs) != runs {
			s.Fatalf("Send records are not correctly saved: got %d, want %d", len(rs), runs)
		}

		s.Log("Fake upload succeeded; continuing until we hit the rate limit")
	}

	if runs < minRuns {
		s.Fatalf("crash_sender hit the rate limit after %d runs; want >=%d", runs, minRuns)
	}
	s.Logf("crash_sender hit the rate limit after %d runs", runs)

	// Change the timestamp of one send record to 25 hours ago.
	rs, err := crash.ListSendRecords()
	if err != nil {
		s.Fatal("Failed to get send records: ", err)
	}
	if len(rs) == 0 {
		s.Fatal("No send record found")
	}
	fn := filepath.Join(crash.SendRecordDir, rs[0].Name())
	ts := time.Now().Add(-25 * time.Hour)
	if err := os.Chtimes(fn, ts, ts); err != nil {
		s.Fatal("Failed to change the timestamp of a send record file: ", err)
	}

	// Attempt crash_sender again. It should succeed this time.
	s.Logf("Iteration #%d (after modifying send record timestamp)", runs)

	got, err := crash.RunSender(ctx)
	if err != nil {
		s.Fatal("Failed to run crash_sender: ", err)
	}

	if len(got) != 1 {
		s.Fatalf("Unexpected number of results: got %d, want 1", len(got))
	}
	if !got[0].Success {
		s.Error("crash_sender still fails to upload a crash dump after modifying send record timestamp")
	}
}
