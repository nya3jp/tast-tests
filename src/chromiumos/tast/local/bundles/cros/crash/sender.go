// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/crash/sender"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Sender,
		Desc: "Basic test of sending crash reports",
		Contacts: []string{
			"nya@chromium.org", // ported to Tast
			"cros-monitoring-forensics@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func Sender(ctx context.Context, s *testing.State) {
	// Leave some time for teardown.
	fullCtx := ctx
	ctx, cancel := ctxutil.Shorten(fullCtx, 5*time.Second)
	defer cancel()

	if err := crash.SetUpCrashTest(ctx); err != nil {
		s.Fatal("Setup failed: ", err)
	}
	defer func() {
		if err := crash.TearDownCrashTest(fullCtx); err != nil {
			s.Error("Teardown failed: ", err)
		}
	}()

	if err := sender.EnableMock(true); err != nil {
		s.Fatal("Failed to enable crash_sender mock: ", err)
	}
	defer func() {
		if err := sender.DisableMock(); err != nil {
			s.Error("Failed to disable crash_sender mock: ", err)
		}
	}()

	if err := sender.ResetSentReports(); err != nil {
		s.Fatal("Failed to reset crash_sender sent reports: ", err)
	}

	// Create a temporary crash dir to use with crash_sender.
	crashDir, err := ioutil.TempDir("", "crash.")
	if err != nil {
		s.Fatal("Failed to create a temporary crash dir: ", err)
	}
	defer os.RemoveAll(crashDir)

	const (
		basename = "fake.1.2.3"
		exec     = "fake"
		ver      = "my_ver"
	)
	exp, err := sender.AddFakeMinidumpCrash(ctx, crashDir, basename, exec, ver)
	if err != nil {
		s.Fatal("Failed to add a fake minidump crash: ", err)
	}

	got, err := sender.RunCrashSender(ctx, crashDir)
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

	// Check that the scheduled upload time was reasonable.
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
