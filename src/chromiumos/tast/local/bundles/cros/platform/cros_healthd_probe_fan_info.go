// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"os"
	"reflect"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosHealthdProbeFanInfo,
		Desc: "Checks that cros_healthd can fetch fan info",
		Contacts: []string{
			"jschettler@google.com",
			"pmoy@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"diagnostics"},
	})
}

func getNumFans(ctx context.Context) (int, error) {
	// Check to see if a Google EC exists. If it does, use ectool to get the
	// number of fans that should be reported. Otherwise, return 0 if the device
	// does not have a Google EC.
	if _, err := os.Stat("/sys/class/chromeos/cros_ec"); os.IsNotExist(err) {
		return 0, nil
	}

	b, err := testexec.CommandContext(ctx, "ectool", "pwmgetnumfans").Output(testexec.DumpLogOnError)
	if err != nil {
		return 0, errors.Wrap(err, "failed to run ectool command")
	}

	numFans, err := strconv.Atoi(strings.ReplaceAll(strings.TrimSpace(string(b)), "Number of fans = ", ""))
	if err != nil {
		return 0, errors.Wrap(err, "failed to get number of fans")
	}

	return numFans, nil
}

func CrosHealthdProbeFanInfo(ctx context.Context, s *testing.State) {
	// Get the number of fans reported by ectool to determine how many lines of
	// output to expect.
	numFans, err := getNumFans(ctx)
	if err != nil {
		s.Fatal("Failed to get number of fans: ", err)
	}

	lines, err := croshealthd.FetchTelemetry(ctx, croshealthd.CategoryFan, s.OutDir())
	if err != nil {
		s.Fatal("Faled to get fan telemetry info: ", err)
	}

	if len(lines) != numFans+1 {
		s.Fatalf("Incorrect number of output lines: got %d; want %d", len(lines), numFans+1)
	}

	// Verify the headers are correct.
	want := []string{"speed_rpm"}
	got := lines[0]
	if !reflect.DeepEqual(want, got) {
		s.Fatalf("Incorrect headers: got %v; want %v", got, want)
	}

	// Verify each line of fan info contains valid values.
	for _, line := range lines[1:] {
		if len(line) != 1 {
			s.Errorf("Wrong number of values: got %v, want 1", len(line))
			continue
		}

		if speed, err := strconv.Atoi(line[0]); err != nil {
			s.Errorf("Failed to convert %q to integer: %v", want[0], err)
		} else if speed < 0 {
			s.Errorf("Invalid %s: %v", want[0], speed)
		}
	}
}
