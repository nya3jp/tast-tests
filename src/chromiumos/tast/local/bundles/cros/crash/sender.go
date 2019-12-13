// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"chromiumos/tast/local/bundles/cros/crash/sender"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Sender,
		Desc: "Basic test to check that minidump crashes are uploaded",
		Contacts: []string{
			"nya@chromium.org", // ported to Tast
			"cros-monitoring-forensics@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

func Sender(ctx context.Context, s *testing.State) {
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

	got, err := sender.Run(ctx, crashDir)
	if err != nil {
		s.Fatal("Failed to run crash_sender: ", err)
	}
	want := []*sender.SendResult{{
		Success: true,
		Data:    *exp,
	}}
	if diff := cmp.Diff(got, want, cmpopts.IgnoreFields(sender.SendResult{}, "Schedule")); diff != "" {
		s.Log("Results mismatch (-got +want):\n", diff)
		s.Errorf("crash_sender sent unexpected %d results; see logs for diff", len(got))
	}

	// Check that the scheduled upload time is reasonable.
	if len(got) == 1 {
		r := got[0]
		d := r.Schedule.Sub(time.Now())
		const limit = time.Hour
		if d >= limit {
			s.Error("Scheduled time was too later: got %v, want <%v", d, limit)
		}
	}

	// Check that the metadata was removed.
	if _, err := os.Stat(filepath.Join(crashDir, basename+".meta")); err == nil {
		s.Errorf("%s.meta was not removed by crash_sender", basename)
	} else if !os.IsNotExist(err) {
		s.Errorf("Failed to stat %s.meta: %v", basename, err)
	}

	// Check that the sent report is counted for rate limiting.
	if cnt, err := sender.CountSentReports(); err != nil {
		s.Error("Failed to count sent reports: ", err)
	} else if cnt != 1 {
		s.Errorf("Found %d sent reports(s); want 1", cnt)
	}
}
