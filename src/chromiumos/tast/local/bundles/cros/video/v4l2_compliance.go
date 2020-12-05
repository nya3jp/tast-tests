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

// regExp is the regexp to find the summary result from the binary log.
var regExp = regexp.MustCompile(`Total.*: \d+, Succeeded: \d+, Failed: \d+, Warnings: \d+`)

// v4l2Test is used to describe the config used to run each v4l2Test.
type v4l2Test struct {
	command []string      // The command path to be run. This should be relative to /usr/local/bin.
	timeout time.Duration // Timeout to run the v4l2Test.
}

func init() {
	testing.AddTest(&testing.Test{
		Func: V4l2Compliance, // default option: -d /dev/video, /usr/local/bin/v4l2-compliance
		Desc: "Runs v4l2 compatible tests",
		Contacts: []string{
			"stevecho@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		SoftwareDeps: []string{"arm"},
		Params: []testing.Param{{
			// -s: Enable the streaming tests.
			// -f: Test streaming all available formats.
			// -a: Do streaming tests for all inputs or outputs.
			// -v: Turn on verbose reporting.
			Name: "encoder",
			Val:  v4l2Test{command: []string{"v4l2-compliance", "-d", "/dev/video-enc", "-s", "-f", "-a", "-v"}, timeout: 20 * time.Second},
		}, {
			Name: "decoder",
			Val:  v4l2Test{command: []string{"v4l2-compliance", "-d", "/dev/video-dec0", "-s", "-f", "-a", "-v"}, timeout: 20 * time.Second},
		}},
		Timeout: 10 * time.Second,
	})
}

func V4l2Compliance(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(v4l2Test)
	runTestStart(ctx, s, testOpt.timeout, testOpt.command[0], testOpt.command[1:]...)
}

// runTestStart runs the exe binary test.
func runTestStart(ctx context.Context, s *testing.State, t time.Duration, exe string, args ...string) {
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
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Errorf("%s: returns the error code - %v", exe, err)
	}

	contents, err := ioutil.ReadFile(logFile)
	if err != nil {
		s.Logf("Failed to read file %s", logFile)
	}

	matches := regExp.FindAllStringSubmatch(string(contents), -1)
	if len(matches) != 1 {
		s.Logf("Found %d matches for summary result; want 1", len(matches))
	}

	s.Logf("%s", matches) // prints out summary result from last line using regex
}
