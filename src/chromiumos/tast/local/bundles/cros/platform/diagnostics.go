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
		Desc: "Tests 'diag' command line utility",
		Contacts: []string{
			"mathewk@chromium.org", // Test author
			"pmoy@chromium.org",    // diag tool author
		},
		SoftwareDeps: []string{"diagnostics"},
	})
}

// Diagnostics runs and verifies all diagnostics routines that the "diag"
// command line utility defines. This command interacts with the
// "wilco_dtc_supported" deamon by calling it using grpc.
//
// The diag command has two actions it can run. 'get_routines' returns a list of
// all routines that diag supports and 'run_routine' starts running a routine
// and waits until the routine completes and prints the result.
//
// This test verifies that 'get_routines' returns a valid list of routines and
// then runs the "urandom" routine and checks that it passes.
//
// diag is currently only used on the Wilco platform.
func Diagnostics(ctx context.Context, s *testing.State) {
	// Run the diag command with arguments
	runDiag := func(args ...string) string {
		cmd := testexec.CommandContext(ctx, "diag", args...)
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
		ran := ""
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
			case "Routine":
				ran = value
			case "Status":
				status = value
			case "Progress":
				i, err := strconv.Atoi(value)
				if err != nil {
					s.Fatalf("Unable to parse %q value %q as int: %v", key, value, err)
				}
				progress = i
			}
		}

		if ran != routine {
			s.Fatalf("Got routine %q; want %q", ran, routine)
		}

		if status != "Passed" {
			s.Fatalf("%q routine has status %q; want \"Passed\"", routine, status)
		}

		if progress != 100 {
			s.Fatalf("%q routine has progress %d; want 100", routine, progress)
		}
	}

	if err := upstart.EnsureJobRunning(ctx, "wilco_dtc_supportd"); err != nil {
		s.Fatal("Failed to start diagnostic daemon: ", err)
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
