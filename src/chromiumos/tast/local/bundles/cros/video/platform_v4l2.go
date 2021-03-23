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

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// v4l2SummaryRegExp is the regexp to find the summary result from the binary log.
var v4l2SummaryRegExp = regexp.MustCompile(`Total.*: \d+, Succeeded: \d+, Failed: \d+, Warnings: \d+`)

func init() {
	testing.AddTest(&testing.Test{
		Func: PlatformV4L2,
		Desc: "Runs v4l2 compliance tests",
		Contacts: []string{
			"stevecho@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		SoftwareDeps: []string{"v4l2_codec"},
		Timeout:      2 * time.Minute,
		Params: []testing.Param{{
			// -v: Turn on verbose reporting.
			Name: "decoder",
			Val:  []string{"v4l2-compliance", "-d", "/dev/video-dec0", "-v"},
		}},
	})
}

// PlatformV4L2 runs v4l2-compliance binary test.
func PlatformV4L2(ctx context.Context, s *testing.State) {
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to stop ui job: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	command := s.Param().([]string)

	s.Log("Running ", shutil.EscapeSlice(command))

	logFile := filepath.Join(s.OutDir(), filepath.Base(command[0])+".txt")

	f, err := os.Create(logFile)
	if err != nil {
		s.Fatal("Failed to create a log file: ", err)
	}
	defer f.Close()

	cmd := testexec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Stdout = f
	cmd.Stderr = f
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		exitCode, ok := testexec.ExitCode(err)
		if !ok {
			s.Fatalf("Failed to run %s: %v", command[0], err)
		}

		contents, err := ioutil.ReadFile(logFile)
		if err != nil {
			s.Fatal("Failed to read the log file: ", err)
		}

		matches := v4l2SummaryRegExp.FindAllStringSubmatch(string(contents), -1)
		if matches == nil {
			s.Fatal("Failed to find matches for summary result")
		}
		if len(matches) != 1 {
			s.Fatalf("Found %d matches for summary result; want 1", len(matches))
		}

		if exitCode > 0 {
			s.Errorf("%s", matches)
		} else {
			s.Logf("%s", matches)
		}
	}
}
