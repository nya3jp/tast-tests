// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WilcoDiagnostics,
		Desc: "Tests diagnostic interface for Wilco platform",
		Contacts: []string{
			"mathewk@chromium.org", // Test author
			"pmoy@chromium.org",    // diag tool author
		},
		SoftwareDeps: []string{"wilco_ec"},
		Attr:         []string{"informational"},
	})
}

// WilcoDiagnostics runs and verifies all diagnostics routines that the "diag"
// command line utility defines. This command interacts with the
// "wilco_dtc_supported" deamon by calling it using grpc.
//
// The diag command has two actions it can run. 'get_routines' returns a list of
// all routines that diag supports and 'run_routine' starts running a routine
// and waits until the routine completes and prints the result.
//
// This test verifies that 'get_routines' returns a valid list of rotines and
// then runs each routine and checks that the routine passes.
//
// diag is currently only relevant on the Wilco platform.
func WilcoDiagnostics(ctx context.Context, s *testing.State) {
	// Run the diag command with arguments
	runDiag := func(args ...string) string {
		cmd := testexec.CommandContext(ctx, "diag", args...)
		fullCmd := strings.Join(cmd.Args, " ")
		s.Logf("Running %q", fullCmd)
		out, err := cmd.Output()
		if err != nil {
			cmd.DumpLog(ctx)
			s.Fatalf("Failed to run %q: %v", fullCmd, err)
		}
		s.Logf("Got %q as result", out)
		return string(out)
	}

	// Get a list of all diagnostics routines
	getRoutines := func() []string {
		ret := []string{}
		re := regexp.MustCompile(`Available routine: (.*)`)
		routines := runDiag("--action=get_routines")

		for _, match := range re.FindAllStringSubmatch(routines, -1) {
			ret = append(ret, match[1])
		}

		return ret
	}

	// Test a given routine and ensure that it runs and passes
	testRoutine := func(routine string) {
		raw := runDiag("--action=run_routine", fmt.Sprintf("--routine=%s", routine))
		re := regexp.MustCompile(`\s*([^:]+): (.*)`)
		ran := ""
		status := ""
		progress := 0

		for _, match := range re.FindAllStringSubmatch(raw, -1) {
			key := match[1]
			value := match[2]

			s.Logf("%q: %q", key, value)

			if key == "Routine" {
				ran = value
			}
			if key == "Status" {
				status = value
			}
			if key == "Progress" {
				i, err := strconv.Atoi(value)
				if err != nil {
					s.Fatalf("Unable to parse %q value %q as int: %v", key, value, err)
				}
				progress = i
			}
		}

		if ran != routine {
			s.Fatalf("Unexpected routine ran. expected: %q, got: %q", routine, ran)
		}

		if status != "Passed" {
			s.Fatalf("%q routine did not pass. got: %q", routine, status)
		}

		if progress != 100 {
			s.Fatalf("%q routine did not complete. got: %d", routine, progress)
		}
	}

	routines := getRoutines()

	if len(routines) < 2 {
		s.Fatalf("Only %d routines found %v and at least 2 were expected", len(routines), routines)
	}

	for _, routine := range routines {
		testRoutine(routine)
	}
}
