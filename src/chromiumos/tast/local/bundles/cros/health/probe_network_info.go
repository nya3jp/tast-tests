// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strings"

	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ProbeNetworkInfo,
		Desc: "Check that we can probe cros_healthd for network info",
		Contacts: []string{
			"tbegin@google.com",
			"pmoy@google.com",
			"khegde@google.com",
		},
		// TODO(b/178274447): Test is flaky due to Chrome bootstrapping
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

func ProbeNetworkInfo(ctx context.Context, s *testing.State) {
	b, err := croshealthd.RunTelem(ctx, croshealthd.TelemCategoryNetwork, s.OutDir())
	if err != nil {
		s.Fatal("Failed to run telem command: ", err)
	}

	// Helper function to write the result from telem to a file.
	writeResultToFile := func() {
		if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "network_health_telem.txt"),
			b, 0644); err != nil {
			s.Error("Unable to write network_health_telem.txt file: ", err)
		}
	}

	// Every system should have the field headers and at least one network
	// devices.
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) < 2 {
		writeResultToFile()
		s.Fatal("Could not find any lines of network info")
	}

	// Verify the header keys are correct.
	header := []string{"type", "state", "guid", "name", "signal_strength", "mac_address"}
	got := strings.Split(lines[0], ",")
	if !reflect.DeepEqual(got, header) {
		writeResultToFile()
		s.Fatalf("Incorrect NetworkInfo keys: got %v; want %v", got, header)
	}

	// Verify that all network devices have the correct number of fields.
	for _, line := range lines[1:] {
		vals := strings.Split(line, ",")
		if len(vals) != len(header) {
			writeResultToFile()
			s.Fatalf("Unexpected number of fields in network structure: got: %v, want: %v, fields: %v",
				len(vals), len(header), vals)
		}
	}
}
