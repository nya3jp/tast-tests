// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WilcoTelemetry,
		Desc:         "Tests telemetry interface for Wilco platform",
		SoftwareDeps: []string{"wilco_ec"},
		Attr:         []string{"informational"},
	})
}

// WilcoTelemetry performs integration tests for the "telem" command line tool
// that pmoy@google.com has been writing. This series of tests calls telem with
// all of the currently implemented items and groups. telem then communicates
// with libtelem, diagnosticsd, and procfs, and parses the results.
// Each of these systems has its own unit tests, so this is just an end-to-end
// test to ensure all the plumbing is is working between them.
//
// telem supports two command-line arguments, --group and --item.
// --item retrieves a single telemetry item and displays its value to the console.
// --group retrieves a number of related telemetry items and
//   displays each of their values to the console.
//
// Example usage:
// 'telem --item=memfree' will give back: 'memfree: some_value'
// 'telem --group=disk'   will give back  'memtotal: 3814\nmemfree: 2423\n'
//
// More documentation for telem can be found at
// docs.google.com/document/d/1f0hMkh_7prj5oSbLnt7BlYt4LfHc59jfCUhT0gTY_ng
//
// telem is currently only relevant on the Wilco platform, but is planned to
// be generalized later for telemetry work on future platforms as well.
func WilcoTelemetry(ctx context.Context, s *testing.State) {
	// Actually run the telem command with its single argument
	runTelem := func(arg string) string {
		cmd := testexec.CommandContext(ctx, "telem", arg)
		fullCmd := strings.Join(cmd.Args, " ")
		s.Logf("Running %q...", fullCmd)
		out, err := cmd.Output()
		if err != nil {
			cmd.DumpLog(ctx)
			s.Fatalf("Failed to run %q: %v", fullCmd, err)
		}
		s.Logf("Got %q as result", out)
		return string(out)
	}

	// The telem command returns an item in the form "<name>: <value>\n".
	// Run the command and extract the <value>.
	getItem := func(name string) string {
		arg := fmt.Sprintf("--item=%s", name)
		raw := runTelem(arg)
		parts := strings.SplitN(raw, ": ", 2)
		if len(parts) != 2 {
			s.Fatalf("telem returned bad output %q; want format '<name>: <value>'", raw)
		}
		val := parts[1]
		if !strings.HasSuffix(val, "\n") {
			s.Fatalf("got item=%q; should end with a newline", val)
		}
		return strings.TrimSuffix(val, "\n")
	}

	attemptAtoi := func(name, val string) int {
		i, err := strconv.Atoi(val)
		if err != nil {
			s.Fatalf("Unable to parse %q value %q as int: %v", name, val, err)
		}
		return i
	}

	// Given the name of a group, return all the items in that group as a map of name:value
	// The telem command returns data in the form
	// "<name1>: <value1>\n<name2>: <value2>\n<name3>: <value3>\n<nameN>: <valueN>\n"
	getGroup := func(name string) map[string]string {
		arg := fmt.Sprintf("--group=%s", name)
		raw := runTelem(arg)
		lines := strings.Split(raw, "\n")
		if len(lines) < 2 {
			s.Fatalf("only got back the lines %v from running telem with --group=%s; expected at least 2 of them", lines, name)
		}
		if lines[len(lines)-1] != "" {
			s.Fatalf("Got the last line %q from running telem with --group=%s; last line should be empty", lines[len(lines)-1], name)
		}
		// throw out the last line, it's empty
		lines = lines[:len(lines)-1]

		// build a map, one entry for each line
		items := make(map[string]string)
		for _, line := range lines {
			parts := strings.SplitN(line, ": ", 2)
			if len(parts) != 2 {
				s.Fatalf("telem returned bad output %q; want format '<name>: <value>'", raw)
			}
			items[parts[0]] = parts[1]
		}
		return items
	}

	// Used to define a valid numerical range for some items
	type numRange struct{ min, max int }

	// Get an item, parse to int, verify that it's in range
	testNumericalItem := func(name string, val int, validRange numRange) {
		if val < validRange.min || val > validRange.max {
			s.Fatalf("Got %s=%d, needed to be in [%d, %d]", name, val, validRange.min, validRange.max)
		}
	}

	// This returns in format:
	// "idle_time_per_cpu: \n<value>\n<value>\n<value>\n<value>\n"
	// So there's a weird leading newline to trim
	testIdleTimePerCPU := func() {
		const name = "idle_time_per_cpu"
		validRange := numRange{1, 1000000000}

		// Trim leading newline
		raw := strings.TrimSpace(getItem(name))
		lines := strings.Split(raw, "\n")
		if len(lines) < 1 {
			s.Fatalf("telem returned no data for item %q", name)
		}
		for _, line := range lines {
			val := attemptAtoi(name, line)
			testNumericalItem(name, val, validRange)
		}
	}

	validRanges := map[string]numRange{
		"memtotal":          numRange{0, 16000},
		"memfree":           numRange{0, 16000},
		"runnable_entities": numRange{1, 1000},
		"existing_entities": numRange{1, 1000},
		"idle_time_total":   numRange{1, 1000000000},
	}

	// ensure each individual item is in range
	for name, vr := range validRanges {
		val := attemptAtoi(name, getItem(name))
		testNumericalItem(name, val, vr)
	}

	// collect all the items in the "disk" group, ensure they're all in range
	items := getGroup("disk")
	for name, val := range items {
		r, ok := validRanges[name]
		if !ok {
			s.Errorf("Unexpected name %q in disk group", name)
		}
		n := attemptAtoi(name, val)
		testNumericalItem(name, n, r)
	}

	// The idle_time_per_cpu item is weird, do it by itself. This is a
	// single item that returns multiple lines. All other items return only
	// one line, and all other groups contain a label for each value.
	testIdleTimePerCPU()
}
