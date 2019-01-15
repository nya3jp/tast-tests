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
		Func: Telemetry,
		Desc: "Tests 'telem' command line utility",
		Contacts: []string{
			"ncrews@chromium.org", // Test author
			"pmoy@chromium.org",   // telem tool author
		},
		SoftwareDeps: []string{"diagnostics"},
		Attr:         []string{"informational"},
	})
}

// Telemetry performs integration tests for the "telem" command line tool.
// telem receives its information from libtelem, procfs, and
// wilco_dtc_supportd. Each of these systems has its own unit tests,
// so this test ensures all the plumbing is working throughout the stack.
//
// telem supports two command-line arguments, --group and --item.
//  --item retrieves a single telemetry item and displays its value.
//  --group retrieves a number of related telemetry items and
//    displays each of their values.
//
// Example usage:
// "telem --item=memfree" will return "memfree: some_value"
// "telem --group=disk"   will return "memtotal: 3814\nmemfree: 2423\n"
//
// This series of tests calls telem with all of the currently implemented items
// and groups, and verifies the results are in a reasonable range.
//
// telem is currently only used on the Wilco platform.
func Telemetry(ctx context.Context, s *testing.State) {
	// Actually run the telem command with its single argument.
	runTelem := func(arg string) string {
		cmd := testexec.CommandContext(ctx, "telem", arg)
		s.Logf(`Running "telem %s"...`, arg)
		out, err := cmd.Output()
		if err != nil {
			cmd.DumpLog(ctx)
			s.Fatalf(`Failed to run "telem %s": %v`, arg, err)
		}
		s.Logf("Got %q as result", out)
		return string(out)
	}

	// The telem command returns an item in the form "<name>: <value>\n".
	// Run the command and extract the <value> as a string.
	getItem := func(name string) string {
		arg := fmt.Sprintf("--item=%s", name)
		raw := runTelem(arg)
		parts := strings.SplitN(raw, ": ", 2)
		if len(parts) != 2 {
			s.Fatalf(`"telem %s" returned %q; want format "<name>: <value>"`, arg, raw)
		}
		val := parts[1]
		if !strings.HasSuffix(val, "\n") {
			s.Fatalf(`"telem %s" returned %q; should end with a newline`, arg, val)
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

	// Given the name of a group, return all the items in that group as
	// a map of <name>:<value>. The telem command returns data in the form:
	// "<name1>: <value1>\n<name2>: <value2>\n<name3>: <value3>\n<nameN>: <valueN>\n"
	getGroup := func(name string) map[string]string {
		arg := fmt.Sprintf("--group=%s", name)
		raw := runTelem(arg)
		lines := strings.Split(raw, "\n")
		if len(lines) < 2 {
			s.Fatalf(`Got lines %v from "telem --group=%s"; expected at least 2`, lines, name)
		}
		if lines[len(lines)-1] != "" {
			s.Fatalf(`Got %q as last line from "telem --group=%s"; should be empty`, lines[len(lines)-1], name)
		}
		// Throw out the last line, it's empty.
		lines = lines[:len(lines)-1]

		// Build a map, one entry for each line.
		items := make(map[string]string)
		for _, line := range lines {
			parts := strings.SplitN(line, ": ", 2)
			if len(parts) != 2 {
				s.Fatalf(`One of lines in group %q was %q; want format "<name>: <value>"`, name, line)
			}
			items[parts[0]] = parts[1]
		}
		return items
	}

	// Used to define a valid numerical range for some items.
	type numRange struct{ min, max int }

	// Get an item, parse to int, verify that it's in range.
	testNumericalItem := func(name string, val int, validRange numRange) {
		if val < validRange.min || val > validRange.max {
			s.Fatalf("Got %s=%d, needed to be in [%d, %d]", name, val, validRange.min, validRange.max)
		}
	}

	// This returns in the unique format:
	// "idle_time_per_cpu: \n<value>\n<value>\n<value>\n<value>\n"
	// So we treat this item by itself.
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

	// Ensure each individual item is in range.
	for name, vr := range validRanges {
		val := attemptAtoi(name, getItem(name))
		testNumericalItem(name, val, vr)
	}

	// Collect all items in the "disk" group, ensure they're all in range.
	items := getGroup("disk")
	for name, val := range items {
		r, ok := validRanges[name]
		if !ok {
			s.Errorf("Unexpected name %q in disk group", name)
		}
		n := attemptAtoi(name, val)
		testNumericalItem(name, n, r)
	}

	// The idle_time_per_cpu item is weird, so do it by itself. This is a
	// single item that returns multiple lines. All other items return only
	// one line, and all other groups contain a label for each value.
	testIdleTimePerCPU()
}
