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

	s.Log("Starting a binary")
	// Use `sleep` command as a target to crash. This command does nothing and is easy to send signals. The argument should not be so large to prevent that it blocks too long when `pkill` failed to kill this command.
	cmd := a.Command(ctx, "/system/bin/sleep", "10")
	if err := cmd.Start(); err != nil {
		s.Fatal("Failed to run sleep command: ", err)
	}

	s.Log("Making crash")
	// Use `pkill` in ARCVM. We cannot use `cmd.Signal(syscall.SIGSEGV)` here. The `sleep` command is executed in the guest via `adb shell`. `cmd.Signal` sends signals to the `adb` command instead of `sleep` command.
	killCmd := a.Command(ctx, "/vendor/bin/pkill", "-SEGV", "-fx", "/system/bin/sleep 10")
	if err := killCmd.Run(); err != nil {
		s.Fatal("Failed to run kill command: ", err)
	}
	if err := cmd.Wait(); err != nil {
		s.Log("Succeeded to crash: ", err)
	} else {
		// `err == nil` means that `sleep` command has successfully finished without crashing.
		s.Fatal("Failed to crash")
	}

	s.Log("Getting the crash dir path for native crash dumps in ARCVM")
	user := cr.User()
	androidDataDir, err := arc.AndroidDataDir(user)
	if err != nil {
		s.Fatal("Failed to get android-data dir: ", err)
	}
	crashDir := filepath.Join(androidDataDir, crashReportsDirPathInAndroid)

	s.Log("Waiting for crash files to become present")
	const pollingTimeout = 10 * time.Second // The time to wait dump files. Typically they appears in a few seconds.
	// We cannot use `crash.WaitForCrashFiles` because it requires that .meta file has `done=1` as its content.
	err = testing.Poll(ctx, func(c context.Context) error {
		// list files in `crashDir`
		files, err := ioutil.ReadDir(crashDir)
		if err != nil {
			return err
		}

		// check the existence of .dmp file and .meta file
		expectedFileNameRegexes := []string{
			`\d+\.\d+\.sleep\.dmp`,
			`\d+\.\d+\.sleep\.meta`,
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
