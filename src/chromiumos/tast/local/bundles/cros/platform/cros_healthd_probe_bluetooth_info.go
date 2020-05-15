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

	"chromiumos/tast/local/bluetooth"
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

type bluetoothAdapter struct {
	name    string
	address string
	powered bool
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

	if len(lines) < 2 {
		s.Fatalf("Wrong number of output lines: got %v; want 2+", len(lines))
	}

	// Verify the headers are correct.
	want := []string{"name", "address", "powered", "num_connected_devices"}
	got := lines[0]
	if !reflect.DeepEqual(want, got) {
		s.Fatalf("Incorrect headers: got %v; want %v", got, want)
	}

	// Get Bluetooth adapter values to compare to the output of cros_healthd.
	adapters, err := bluetooth.Adapters(ctx)
	if err != nil {
		s.Fatal("Unable to get Bluetooth adapters: ", err)
	}

	if len(adapters) != 1 {
		s.Fatalf("Unexpected Bluetooth adapters count: got %v; want 1", len(adapters))
	}

	adapter := adapters[0]
	var btAdapter bluetoothAdapter
	if btAdapter.name, err = adapter.Name(ctx); err != nil {
		s.Fatal("Unable to get name property value: ", err)
	}

	if btAdapter.address, err = adapter.Address(ctx); err != nil {
		s.Fatal("Unable to get address property value: ", err)
	}

	if btAdapter.powered, err = adapter.Powered(ctx); err != nil {
		s.Fatal("Unable to get powered property value: ", err)
	}

	// Verify the values are correct.
	vals := lines[1]
	if len(vals) != 4 {
		s.Fatalf("Wrong number of values: got %v; want 4", len(vals))
	}

	if vals[0] != btAdapter.name {
		s.Errorf("Invalid name: got %v; want %v", vals[0], btAdapter.name)
	}

	if vals[1] != btAdapter.address {
		s.Errorf("Invalid address: got %v; want %v", vals[1], btAdapter.address)
	}

	var powered string
	if btAdapter.powered {
		powered = "true"
	} else {
		powered = "false"
	}
	if vals[2] != powered {
		s.Errorf("Invalid powered value: got %v; want %v", vals[2], powered)
	}

	if num, err := strconv.Atoi(vals[3]); err != nil {
		s.Errorf("Failed to convert %q (num_connected_devices) to integer: %v", vals[3], err)
	} else if num < 0 {
		s.Errorf("Invalid num_connected_devices: got %v; want 0+", num)
	}
}
