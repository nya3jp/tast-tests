// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         NativeCrash,
		Desc:         "Test handling of a native binary crash",
		Contacts:     []string{"kimiyuki@google.com", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          arc.Booted(),
		Params: []testing.Param{{
			Name:              "real_consent",
			ExtraSoftwareDeps: []string{"android_vm", "metrics_consent"},
			Val:               crash.RealConsent,
		}, {
			Name:              "mock_consent",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val:               crash.MockConsent,
		}},
	})
}

func NativeCrash(ctx context.Context, s *testing.State) {
	const (
		crashReportsDirPathInAndroid = "/data/vendor/arc_native_crash_reports"
	)

	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome

	opt := crash.WithMockConsent()
	if s.Param().(crash.ConsentType) == crash.RealConsent {
		opt = crash.WithConsent(cr)
	}

	if err := crash.SetUpCrashTest(ctx, opt); err != nil {
		s.Fatal("Failed to set up crash test: ", err)
	}
	defer crash.TearDownCrashTest(ctx)

	s.Log("Making crash")
	cmd := a.Command(ctx, "/system/bin/sh", "-c", "kill -SEGV $$")
	if err := cmd.Run(); err != nil {
		// The shell returns 139 (= 128 + 11) when it's terminated by SIGSEGV (= 11).
		if cmd.ProcessState.ExitCode() != 139 {
			s.Fatal("Failed to crash: ", err)
		}
	} else {
		s.Fatal("Failed to crash: the process has successfully finished without crashing")
	}

	s.Log("Getting crash dir path")
	user := cr.User()
	path, err := cryptohome.UserPath(ctx, user)
	if err != nil {
		s.Fatal("Couldn't get user path: ", err)
	}
	crashDir := filepath.Join(path, "/crash")

	s.Log("Waiting for crash files to become present")
	// Wait files like sh.20200420.204845.664107.log in crashDir
	const stem = `sh\.\d{8}\.\d{6}\.\d+`
	files, err := crash.WaitForCrashFiles(ctx, []string{crashDir}, []string{
		stem + crash.MinidumpExt, stem + crash.MetadataExt,
	})
	if err != nil {
		s.Fatal("Failed to find files: ", err)
	}
	defer crash.RemoveAllFiles(ctx, files)
}
