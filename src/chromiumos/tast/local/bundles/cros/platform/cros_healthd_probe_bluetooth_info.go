// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"encoding/csv"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosHealthdProbeBluetoothInfo,
		Desc: "Checks that cros_healthd can fetch Bluetooth info",
		Contacts: []string{
			"jschettler@google.com",
			"pmoy@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"diagnostics"},
	})
}

func CrosHealthdProbeBluetoothInfo(ctx context.Context, s *testing.State) {
	if err := upstart.EnsureJobRunning(ctx, "cros_healthd"); err != nil {
		s.Fatal("Failed to start cros_healthd: ", err)
	}

	b, err := testexec.CommandContext(ctx, "telem", "--category=bluetooth").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Command failed: ", err)
	}

	// Log output to file for debugging.
	path := filepath.Join(s.OutDir(), "command_output.txt")
	if err := ioutil.WriteFile(path, b, 0644); err != nil {
		s.Errorf("Failed to write output to %s: %v", path, err)
	}

	lines, err := csv.NewReader(strings.NewReader(string(b))).ReadAll()
	if err != nil {
		s.Fatal("Failed to parse output: ", err)
	}

	if len(lines) < 1 {
		s.Fatal("Output contains no lines")
	}

	// Verify the headers are correct.
	want := []string{"name", "address", "powered", "num_connected_devices"}
	got := lines[0]
	if !reflect.DeepEqual(want, got) {
		s.Fatalf("Incorrect headers: got %v; want %v", got, want)
	}

	// Verify each line of Bluetooth info contains valid values.
	for _, line := range lines[1:] {
		if len(line) != 4 {
			s.Errorf("Wrong number of values: got %v, want 4", len(line))
			continue
		}

		if line[0] == "" {
			s.Error("Empty name")
		}

		if line[1] == "" {
			s.Error("Empty address")
		}

		if line[2] != "true" && line[2] != "false" {
			s.Errorf("Invalid %s: %v", want[2], line[2])
		}

		if num, err := strconv.Atoi(line[3]); err != nil {
			s.Errorf("Failed to convert %q to integer: %v", want[3], err)
		} else if num < 0 {
			s.Errorf("Invalid %s: %v", want[3], num)
		}
	}
}
