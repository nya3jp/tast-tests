// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/crash"
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

	s.Log("Getting the crash dir path for native crash dumps in ARCVM")
	user := cr.User()
	androidDataDir, err := arc.AndroidDataDir(user)
	if err != nil {
		s.Fatal("Failed to get android-data dir: ", err)
	}
	crashDir := filepath.Join(androidDataDir, crashReportsDirPathInAndroid)

	s.Log("Waiting for crash files to become present")
	const pollingTimeout = 10 * time.Second // The time to wait for dump files. Typically they appear in a few seconds.
	// We cannot use `crash.WaitForCrashFiles` because it requires that .meta file has `done=1` as its content.
	err = testing.Poll(ctx, func(c context.Context) error {
		// list files in `crashDir`
		files, err := ioutil.ReadDir(crashDir)
		if err != nil {
			return testing.PollBreak(err)
		}

		// check the existence of .dmp file and .meta file
		expectedFileNameRegexes := []string{
			`\d+\.\d+\.sh\.dmp`,
			`\d+\.\d+\.sh\.meta`,
		}
		var missingFileNameRegexes []string
		for _, re := range expectedFileNameRegexes {
			match := false
			for _, fi := range files {
				match, err = regexp.MatchString(re, fi.Name())
				if err != nil {
					return testing.PollBreak(errors.Wrapf(err, "invalid regexp %s", re))
				}
				if match {
					break
				}
			}
			if !match {
				missingFileNameRegexes = append(missingFileNameRegexes, re)
			}
		}

		if len(missingFileNameRegexes) != 0 {
			var filePaths []string
			for _, fi := range files {
				filePaths = append(filePaths, filepath.Join(crashDir, fi.Name()))
			}
			return errors.Errorf("no file matched %s (found %s)", strings.Join(missingFileNameRegexes, ", "), strings.Join(filePaths, ", "))
		}
		return nil
	}, &testing.PollOptions{Timeout: pollingTimeout})
	if err != nil {
		s.Fatal("Didn't find files: ", err)
	}
}
