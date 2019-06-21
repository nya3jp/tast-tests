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
		Func: CrosHealthdProbeBlockDevices,
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

func calcDeviceLines(csvContent string) int {
	// The output of cros_healthd probe commands is just a CSV with a line of
	// headers. This function returns the number of rows of data in the CSV.
	lineCount := -1
	for _, line := range strings.Split(csvContent, "\n") {
		if len(strings.TrimSpace(line)) > 0 {
			lineCount++
		}
	}
	if lineCount > 0 {
		return lineCount
	}
	return 0
}

func CrosHealthdProbeBlockDevices(ctx context.Context, s *testing.State) {
	// For now we are only testing probe_block_devices because that is all
	// that's currently implemented.
	b, err := testexec.CommandContext(ctx, "cros_healthd", "--probe_block_devices").Output(testexec.DumpLogOnError)

	if err != nil {
		s.Fatal("Failed to run 'cros_healthd --probe_block_devices': ", err)
	}

	// The theory here is that every board should have at least one
	// non-removable block device showing up in this list.
	if calcDeviceLines(string(b)) == 0 {
		s.Fatal("Could not find non-removable block storage device by running 'cros_healthd --probe_block_devices'")
	}
}
