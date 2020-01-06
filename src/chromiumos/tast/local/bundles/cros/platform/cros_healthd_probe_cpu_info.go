// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"strconv"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosHealthdProbeCPUInfo,
		Desc: "Check that we can probe cros_healthd for CPU info",
		Contacts: []string{
			"jschettler@google.com",
			"khegde@google.com",
			"pmoy@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"diagnostics"},
	})
}

func CrosHealthdProbeCPUInfo(ctx context.Context, s *testing.State) {
	b, err := testexec.CommandContext(ctx, "telem", "--category=cpu").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to run 'telem --category=cpu': ", err)
	}

	// Every board should have at least one CPU showing up in this list. The
	// output of the command is lines of CSVs beginning with a single line of
	// headers. Ignore the line containing the headers and verify at least one
	// other line with valid CPU info values exists. If more than one line
	// contains CPU info values, verify the values are valid for each line.
	lineCount := 0
	for _, line := range strings.Split(string(b), "\n") {
		if len(strings.TrimSpace(line)) > 0 {
			lineCount++
			if lineCount > 1 {
				vals := strings.Split(line, ",")
				if len(vals) != 3 {
					s.Fatalf("Wrong number of values: got %v, want 3", len(vals))
				}

				if vals[0] == "" {
					s.Fatal("Empty model name")
				}

				if vals[1] == "" {
					s.Fatal("Empty architecture")
				}

				i, err := strconv.Atoi(vals[2])
				if err != nil || i == 0 {
					s.Fatal("Invalid max clock speed")
				}
			}
		}
	}

	if lineCount < 2 {
		s.Fatal("Could not find any lines of CPU information")
	}
}
