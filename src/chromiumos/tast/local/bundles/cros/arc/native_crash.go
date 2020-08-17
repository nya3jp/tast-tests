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
	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome

	opt := crash.WithMockConsent()
	useConsent := s.Param().(crash.ConsentType)
	if useConsent == crash.RealConsent {
		opt = crash.WithConsent(cr)
	}

	if err := crash.SetUpCrashTest(ctx, opt); err != nil {
		s.Fatal("Couldn't set up crash test: ", err)
	}
	defer crash.TearDownCrashTest(ctx)

	s.Log("Starting binary")
	cmd := a.Command(ctx, "/system/bin/sleep", "30")
	if err := cmd.Start(); err != nil {
		s.Fatal("Failed to start a native binary to be crashed: ", err)
	}

	s.Log("Making crash")
	// Use `pkill` in ARCVM. We cannot use `cmd.Signal(syscall.SIGSEGV)` here. The `sleep` command is executed in the guest via `adb shell ...` command of the host. `cmd.Signal` sends signals to the `adb` command instead of `sleep` command.
	killCmd := a.Command(ctx, "/vendor/bin/pkill", "-SEGV", "-fx", "/system/bin/sleep 30")
	if err := killCmd.Run(); err != nil {
		s.Fatal("Failed to run kill command: ", err)
	}
	if err := cmd.Wait(); err != nil {
		s.Log("Succeeded to crash: ", err)
	} else {
		s.Error("Failed to crash")
	}

	s.Log("Getting crash dir path")
	user := cr.User()
	androidDataDir, err := arc.AndroidDataDir(user)
	if err != nil {
		s.Fatal("Failed to get android-data dir: ", err)
	}
	crashDir := filepath.Join(androidDataDir, "/data/vendor/arc_native_crash_reports")

	s.Log("Waiting for crash files to become present")
	// We cannot use `crash.WaitForCrashFiles` because it requires that .meta file has `done=1` as its content.
	err = testing.Poll(ctx, func(c context.Context) error {
		// list files in `crashDir`
		files, err := ioutil.ReadDir(crashDir)
		if err != nil {
			return err
		}
		var filePaths []string
		for _, fi := range files {
			filePaths = append(filePaths, filepath.Join(crashDir, fi.Name()))
		}

		// check the existence of .dmp file and .meta file
		regexes := []string{
			`\d+\.\d+\.sleep\.dmp`,
			`\d+\.\d+\.sleep\.meta`,
		}
		var missing []string
		for _, re := range regexes {
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
				missing = append(missing, re)
			}
		}

		if len(missing) != 0 {
			return errors.Errorf("no file matched %s (found %s)", strings.Join(missing, ", "), strings.Join(filePaths, ", "))
		}
		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second})
	if err != nil {
		s.Fatal("Didn't find files: ", err)
	}
}
