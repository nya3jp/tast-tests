// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosHealthd,
		Desc: "Check that we can probe cros_healthd for various probe data points",
		Contacts: []string{
			"wbbradley@google.com",
			"pmoy@google.com",
			"khegde@google.com",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"diagnostics"},
	})
}

func calcOutputLines(output string) int {
	// Look for non-empty lines to count
	lines := strings.Split(output, "\n")
	lineCount := 0
	for _, line := range lines {
		if len(strings.TrimSpace(line)) > 0 {
			lineCount++
		}
	}
	return lineCount
}

func CrosHealthd(ctx context.Context, s *testing.State) {
	// For now we are only testing probe_block_devices because that is all that's currently implemented.
	cmd := testexec.CommandContext(ctx, "cros_healthd", "--probe_block_devices")
	b, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to run 'cros_healthd --probe_block_devices': ", err)
	}

	output := string(b)
	s.Log(output)
	lineCount := calcOutputLines(output)

	s.Logf("Number of lines of output is %d", lineCount)
	if lineCount < 2 {
		s.Fatalf("Could not find non-removable block storage device by running 'cros_healthd --probe_block_devices'; output was %q", output)
	} else {
		s.Log("Found non-removable block storage devices")
	}
}
