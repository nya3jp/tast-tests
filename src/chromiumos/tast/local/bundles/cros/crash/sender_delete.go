// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"os"
	"path/filepath"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"chromiumos/tast/local/crash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SenderDelete,
		Desc: "Check that crash_sender's --delete_crashes flag works",
		Contacts: []string{
			"mutexlox@chromium.org",
			"cros-telemetry@google.com",
		},
		Attr: []string{"group:mainline"},
	})
}

func SenderDelete(ctx context.Context, s *testing.State) {
	if err := crash.SetUpCrashTest(ctx, crash.FilterCrashes(crash.FilterInIgnoreAllCrashes), crash.WithMockConsent()); err != nil {
		s.Fatal("Setup failed: ", err)
	}
	defer crash.TearDownCrashTest(ctx)

	const basename = "some_program.1.2.3"
	exp, err := crash.AddFakeMinidumpCrash(ctx, basename)
	if err != nil {
		s.Fatal("Failed to add a fake minidump crash: ", err)
	}

	got, err := crash.RunSenderNoDelete(ctx)
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

	metaFile := filepath.Join(crash.SystemCrashDir, basename+".meta")
	if _, err := os.Stat(metaFile); err != nil {
		if os.IsNotExist(err) {
			s.Errorf("%s.meta was removed by crash_sender: %v", basename, err)
		} else {
			s.Errorf("Failed to stat %s.meta: %v", basename, err)
		}
	}
	if _, err := os.Stat(filepath.Join(crash.SystemCrashDir, basename+".alreadyuploaded")); err != nil {
		s.Error("crash_sender did not mark .meta file as uploaded")
	}
}
