// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package telemetry

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

func runAndMaybeCrash(ctx context.Context, s *testing.State, testCmd string) []byte {
	cmd := testexec.CommandContext(ctx, "bash", "-c", testCmd)
	out, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		s.Fatal(fmt.Sprintf("Failed to run 'bash -c %s'", testCmd), err)
	}
	return out
}

// the telem command returns an item in the form "<name>: <value>\n".
// Run the command and extract the <value>
func getItem(ctx context.Context, s *testing.State, name string) string {
	cmd := fmt.Sprintf("telem --item=%s", name)
	raw := string(runAndMaybeCrash(ctx, s, cmd))
	val := strings.Split(raw, ": ")[1]
	// trim trailing newline
	return val[:len(val)-1]
}

func attemptAtoi(s *testing.State, name, val string) int {
	i, err := strconv.Atoi(val)
	if err != nil {
		s.Fatalf("Got %s=%s, unable to parse to int", name, val)
	}
	return i
}

func getIntegerItem(ctx context.Context, s *testing.State, name string) int {
	val := getItem(ctx, s, name)
	return attemptAtoi(s, name, val)
}

// Given the name of a group, return all the items in that group as a map of name:value
// The telem command returns data in the form
// "<name1>: <value1>\n<name2>: <value2>\n<name3>: <value3>\n<nameN>: <valueN>\n"
func getGroup(ctx context.Context, s *testing.State, name string) map[string]string {
	cmd := fmt.Sprintf("telem --group=%s", name)
	raw := string(runAndMaybeCrash(ctx, s, cmd))
	lines := strings.Split(raw, "\n")
	// throw out the last line, it's empty
	lines = lines[:len(lines)-1]

	// build a map, one entry for each line
	items := make(map[string]string)
	for _, line := range lines {
		pair := strings.Split(line, ": ")
		items[pair[0]] = pair[1]
	}
	return items
}

func testNumericalItem(s *testing.State, name string, val int, validRange []int) {
	min, max := validRange[0], validRange[1]
	if val < min || val > max {
		s.Fatalf("Got %s=%s, needed to be in [%d, %d]", name, val, min, max)
	}
}

// This returns in format:
// "idle_time_per_cpu: \n<value>\n<value>\n<value>\n<value>\n"
// So there's a weird leading newline to trim
func testIdleTimePerCPU(ctx context.Context, s *testing.State) {
	const name = "idle_time_per_cpu"
	validRange := []int{1, 1000000000}

	raw := getItem(ctx, s, name)
	lines := strings.Split(raw, "\n")
	// throw out the first line, it's empty
	for _, line := range lines[1:] {
		val := attemptAtoi(s, name, line)
		testNumericalItem(s, name, val, validRange)
	}
}

func WilcoTelemetry(ctx context.Context, s *testing.State) {
	// All of the items tested are described here:
	// https://docs.google.com/document/d/1f0hMkh_7prj5oSbLnt7BlYt4LfHc59jfCUhT0gTY_ng/edit
	validRanges := map[string][]int{
		"memtotal":          []int{0, 16000},
		"memfree":           []int{0, 16000},
		"runnable_entities": []int{1, 1000},
		"existing_entities": []int{1, 1000},
		"idle_time_total":   []int{1, 1000000000},
	}

	// ensure each individual item is in range
	for name, vr := range validRanges {
		val := getIntegerItem(ctx, s, name)
		testNumericalItem(s, name, val, vr)
	}

	// collect all the items in the "disk" group, ensure they're all in range
	items := getGroup(ctx, s, "disk")
	for name, val := range items {
		i := attemptAtoi(s, name, val)
		testNumericalItem(s, name, i, validRanges[name])
	}

	// the idle_time_per_cpu item is weird, do it by itself
	testIdleTimePerCPU(ctx, s)
}
