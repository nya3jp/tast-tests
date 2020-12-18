// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// igtTest is used to describe the config used to run each test.
type igtTest struct {
	exe string // The test executable name.
}

// resultSummary is a summary of results from an igt test log.
type resultSummary struct {
	passed  int // number of passed subtests
	failed  int // number of failed subtests
	skipped int // number of skipped subtests
}

var subtestResultRegex = regexp.MustCompile("^Subtest .*: ([A-Z]+)")

func init() {
	testing.AddTest(&testing.Test{
		Func: IGT,
		Desc: "Verifies igt-gpu-tools test binaries run successfully",
		Contacts: []string{
			"ddavenport@chromium.org",
			"chromeos-gfx@google.com",
		},
		SoftwareDeps: []string{"drm_atomic", "igt"},
		Params: []testing.Param{{
			Name: "kms_atomic",
			Val: igtTest{
				exe: "kms_atomic",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_addfb_basic",
			Val: igtTest{
				exe: "kms_addfb_basic",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_plane",
			Val: igtTest{
				exe: "kms_plane",
			},
			Timeout: 5 * time.Minute,
		}},
		Attr:    []string{"group:graphics", "graphics_perbuild"},
		Fixture: "gpuWatchDog",
	})
}

func summarizeLog(f *os.File) resultSummary {
	var r resultSummary
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if m := subtestResultRegex.FindStringSubmatch(scanner.Text()); m != nil {
			switch m[1] {
			case "SKIP":
				r.skipped++
			case "FAIL":
				r.failed++
			case "SUCCESS":
				r.passed++
			}
		}
	}
	return r
}

func IGT(ctx context.Context, s *testing.State) {
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to stop ui job: ", err)
	}

	testOpt := s.Param().(igtTest)

	f, err := os.Create(filepath.Join(s.OutDir(), filepath.Base(testOpt.exe)+".txt"))
	if err != nil {
		s.Fatal("Failed to create a log file: ", err)
	}
	defer f.Close()

	exePath := filepath.Join("/usr/local/libexec/igt-gpu-tools", testOpt.exe)
	cmd := testexec.CommandContext(ctx, exePath)
	cmd.Stdout = f
	cmd.Stderr = f
	err = cmd.Run()

	if err != nil {
		s.Errorf("Error running %s: %v", exePath, err)
	}

	// Reset the file to the beginning so the log can be read out again.
	f.Seek(0, 0)
	results := summarizeLog(f)
	summary := fmt.Sprintf("Ran %d subtests with %d failures and %d skipped",
		results.passed+results.failed, results.failed, results.skipped)

	if results.passed+results.failed+results.skipped == 0 {
		s.Error("No tests were run")
	} else if err != nil {
		s.Error(summary)
	} else {
		s.Log(summary)
	}
}
