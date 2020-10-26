// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Diagnostics,
		Desc: "Tests 'cros-health-tool diag' command line invocation",
		Contacts: []string{
			"pmoy@chromium.org", // cros_healthd tool author
		},
		SoftwareDeps: []string{"diagnostics"},
		Attr:         []string{"group:mainline"},
	})
}

// Diagnostics runs and verifies that the 'cros-health-tool diag' command-line
// invocation can get a list of supported routines from cros_healthd, and run
// the urandom routine, which should be supported on every platform that
// cros_healthd runs on. Other routines are not tested, because they could
// flake.
//
// The diag command has two actions it can run. 'get_routines' returns a list of
// all routines that diag supports and 'run_routine' starts running a routine
// and waits until the routine completes and prints the result.
//
// This test verifies that 'get_routines' returns a valid list of routines and
// then runs the "urandom" routine and checks that it passes.
func Diagnostics(ctx context.Context, s *testing.State) {
	// Run the diag command with arguments
	runDiag := func(diag_args ...string) string {
		args := append([]string{"diag"}, diag_args...)
		cmd := testexec.CommandContext(ctx, "cros-health-tool", args...)
		s.Logf("Running %q", shutil.EscapeSlice(cmd.Args))
		out, err := cmd.Output()
		if err != nil {
			cmd.DumpLog(ctx)
			s.Fatalf("Failed to run %q: %v", shutil.EscapeSlice(cmd.Args), err)
		}
		return string(out)
	}

	// Get a list of all diagnostics routines
	getRoutines := func() []string {
		var ret []string
		routines := runDiag("--action=get_routines")
		re := regexp.MustCompile(`Available routine: (.*)`)

		for _, line := range strings.Split(strings.TrimSpace(routines), "\n") {
			match := re.FindStringSubmatch(line)
			if match != nil {
				ret = append(ret, match[1])
			}
		}

		return ret
	}

	// Test a given routine and ensure that it runs and passes
	testRoutine := func(routine string, args ...string) {
		raw := runDiag(append([]string{
			"--action=run_routine", "--routine=" + routine}, args...)...)
		re := regexp.MustCompile(`([^:]+): (.*)`)
		status := ""
		progress := 0

		for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
			match := re.FindStringSubmatch(line)

			if match == nil {
				continue
			}

			key := match[1]
			value := match[2]

			s.Logf("%q: %q", key, value)

			switch key {
			case "Status":
				status = value
			case "Progress":
				// Look for just the last progress value. Diag prints a single
				// line for the progress, which may contain carriage returns.
				// The line will be formatted as follows, where # is any int:
				// #\rProgress: #\rProgress: #\rProgress: # ... \rProgress: #
				// Slicing value after the last space should give us the final
				// progress percent.
				percent := value[strings.LastIndex(value, " ")+1:]
				i, err := strconv.Atoi(percent)
				if err != nil {
					s.Logf("Failed to parse progress status: %q", value)
					s.Fatalf("Unable to parse %q value %q as int: %v", key, percent, err)
				}
				progress = i
			}
		}

		if status != "Passed" {
			s.Fatalf("%q routine has status %q; want \"Passed\"", routine, status)
		}

		if progress != 100 {
			s.Fatalf("%q routine has progress %d; want 100", routine, progress)
		}
	}

	if err := upstart.EnsureJobRunning(ctx, "cros_healthd"); err != nil {
		s.Fatal("Failed to start diagnostics daemon: ", err)
	}

	routines := getRoutines()

	if len(routines) < 2 {
		s.Fatalf("Found %d routine(s) %v; want >=2", len(routines), routines)
	}

	// Only test the urandom routine. Other routines could fail and the CQ
	// should not be blocked in that case. This will test the end to end
	// interaction between "diag" and "wilco_dtc_supportd"
	testRoutine("urandom", "--length_seconds=2")
}
