// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// v4l2Test is used to describe the config used to run each v4l2Test.
type v4l2Test struct {
	command []string      // The command path to be run. This should be relative to /usr/local/bin.
	timeout time.Duration // Timeout to run the v4l2Test.
}

func init() {
	testing.AddTest(&testing.Test{
		Func: V4l2Compliance, // default option: -d /dev/video, /usr/local/bin/v4l2-compliance
		Desc: "Verifies v4l2 compatible tests",
		Contacts: []string{
			"stevecho@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		Params: []testing.Param{{
			// -s: Enable the streaming tests.
			// -f: Test streaming all available formats.
			// -a: Do streaming tests for all inputs or outputs.
			// -v: Turn on verbose reporting.
			Name:      "video_encoder",
			Val:       v4l2Test{command: []string{"v4l2-compliance", "-d", "/dev/video-enc", "-s", "-f", "-a", "-v"}, timeout: 20 * time.Second},
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "video_decoder",
			Val:       v4l2Test{command: []string{"v4l2-compliance", "-d", "/dev/video-dec0", "-s", "-f", "-a", "-v"}, timeout: 20 * time.Second},
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "v4l2_ctl",
			Val:       v4l2Test{command: []string{"v4l2-ctl", "--all"}, timeout: 20 * time.Second},
			ExtraAttr: []string{"informational"},
		}},
		Timeout: 20 * time.Minute,
		Attr:    []string{"group:mainline"},
	})
}

func V4l2Compliance(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(v4l2Test)
	runTestStart(ctx, s, testOpt.timeout, testOpt.command[0], testOpt.command[1:]...)
}

// runTestStart runs the exe binary test. This method may be called several times as long as setUp() has been invoked beforehand.
func runTestStart(ctx context.Context, s *testing.State, t time.Duration, exe string, args ...string) {
	// regExp is the regexp to find the summary result from the binary log.
	var regExp = regexp.MustCompile(`Total.*: \d+, Succeeded: \d+, Failed: \d+, Warnings: \d+`)

	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to stop ui job: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	s.Log("Running ", shutil.EscapeSlice(append([]string{exe}, args...)))

	// logFile = file path + name
	logFile := filepath.Join(s.OutDir(), filepath.Base(exe)+".txt")

	f, err := os.Create(logFile)
	if err != nil {
		s.Fatal("Failed to create a log file: ", err)
	}
	defer f.Close()

	ctx, cancel := context.WithTimeout(ctx, t)
	defer cancel()

	cmd := testexec.CommandContext(ctx, exe, args...)
	cmd.Stdout = f
	cmd.Stderr = f
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)

		// Please check more details about test failures at /tmp/tast/results/latest/tests/video.V4l2Compliance.v4l2_compliance/v4l2-compliance.txt
		s.Errorf("%s: returns the error code - %v", exe, err)
	}

	contents, err := ioutil.ReadFile(logFile)
	if err != nil {
		s.Logf("Failed to read file %s", logFile)
	}
	s.Logf("%s", contents) // prints out entire log from v4l2-compatible

	matches := regExp.FindAllStringSubmatch(string(contents), -1)
	if len(matches) != 1 {
		s.Logf("Found %d matches for summary result; want 1", len(matches))
	}

	s.Logf("%s", matches) // prints out summary result from last line using regex
}
