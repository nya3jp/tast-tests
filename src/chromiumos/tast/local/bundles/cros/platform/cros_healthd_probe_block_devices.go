// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosHealthdProbeBlockDevices,
		Desc: "Check that we can probe cros_healthd for various probe data points",
		Contacts: []string{
			"pmoy@google.com",
			"khegde@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"diagnostics"},
	})
}

func CrosHealthdProbeBlockDevices(ctx context.Context, s *testing.State) {
	if err := upstart.EnsureJobRunning(ctx, "cros_healthd"); err != nil {
		s.Fatal("Failed to start cros_healthd: ", err)
	}

	// For now we are only testing probe_block_devices because that is all
	// that's currently implemented.
	// TODO(crbug.com/979210): narrow interface for testing
	b, err := testexec.CommandContext(ctx, "telem", "--category=storage").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to run 'telem --category=storage': ", err)
	}

	// The theory here is that every board should have at least one
	// non-removable block device showing up in this list.  The output of
	// cros_healthd probe commands is just a CSV with a line of headers.
	lineCount := 0
	for _, line := range strings.Split(string(b), "\n") {
		if len(strings.TrimSpace(line)) > 0 {
			lineCount++
			if lineCount > 1 {
				// We found at least 1 row of device information. End the test
				// immediately. This implies success.
				return
			}
		}
	}

	s.Fatal("Could not find any rows of device information")
}
