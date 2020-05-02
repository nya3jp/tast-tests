// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
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
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"diagnostics"},
	})
}

func verifyPhysicalCPU(lines []string) error {
	// Make sure we've received at least four lines. The first should be the
	// physical CPU header, followed by one line of keys, one line of values, and
	// one or more lines of logical CPU data.
	if len(lines) < 4 {
		return errors.New("could not find any lines of physical CPU info")
	}

	// Verify the first line is the correct header.
	actualHeader := lines[0]
	expectedHeader := "Physical CPU:"
	if actualHeader != expectedHeader {
		return errors.Errorf("Incorrect physical CPU header: got %v, want %v", actualHeader, expectedHeader)
	}

	// Verify the key is correct.
	want := "model_name"
	got := lines[1]
	if want != got {
		return errors.Errorf("Incorrect physical CPU key: got %v; want %v", got, want)
	}

	// Verify the value is reasonable.
	if lines[2] == "" {
		return errors.New("Empty model_name")
	}

	// Verify each logical CPU.
	start := 3
	for i := 4; i < len(lines); i++ {
		if lines[i] == "Logical CPU:" {
			err := verifyLogicalCPU(lines[start : i-1])
			if err != nil {
				return errors.Wrap(err, "failed to verify logical CPU")
			}
			start = i
		}
	}
	err := verifyLogicalCPU(lines[start:])
	if err != nil {
		return errors.Wrap(err, "failed to verify logical CPU")
	}

	return nil
}

func verifyLogicalCPU(lines []string) error {
	// Make sure we've received at least four lines. The first should be the
	// logical CPU header, followed by one line of keys, one line of values, and
	// one or more lines of C-state data.
	if len(lines) < 4 {
		return errors.New("could not find any lines of logical CPU info")
	}

	// Verify the first line is the correct header.
	actualHeader := lines[0]
	expectedHeader := "Logical CPU:"
	if actualHeader != expectedHeader {
		return errors.Errorf("Incorrect logical CPU header: got %v, want %v", actualHeader, expectedHeader)
	}

	// Verify the keys are correct.
	want := []string{"max_clock_speed_khz", "scaling_max_frequency_khz", "scaling_current_frequency_khz", "idle_time_user_hz"}
	got := strings.Split(lines[1], ",")
	if !reflect.DeepEqual(want, got) {
		return errors.Errorf("Incorrect logical CPU keys: got %v; want %v", got, want)
	}

	// Verify the values are reasonable.
	vals := strings.Split(lines[2], ",")
	if len(vals) != 4 {
		return errors.Errorf("Wrong number of logical CPU values: got %v, want 4", len(vals))
	}

	maxClockSpeed, err := strconv.Atoi(vals[0])
	if err != nil {
		return errors.Wrapf(err, "failed to convert %q to integer", want[0])
	} else if maxClockSpeed < 0 {
		return errors.Errorf("invalid %s: %v", want[0], maxClockSpeed)
	}

	scalingMaxFreq, err := strconv.Atoi(vals[1])
	if err != nil {
		return errors.Wrapf(err, "failed to convert %q to integer", want[1])
	} else if scalingMaxFreq < 0 {
		return errors.Errorf("invalid %s: %v", want[1], scalingMaxFreq)
	}

	scalingCurFreq, err := strconv.Atoi(vals[2])
	if err != nil {
		return errors.Wrapf(err, "failed to convert %q to integer", want[2])
	} else if scalingCurFreq < 0 {
		return errors.Errorf("invalid %s: %v", want[2], scalingCurFreq)
	}

	idleTime, err := strconv.Atoi(vals[3])
	if err != nil {
		return errors.Wrapf(err, "failed to convert %q to integer", want[3])
	} else if idleTime < 0 {
		return errors.Errorf("invalid %s: %v", want[3], idleTime)
	}

	return verifyCStates(lines[3:])
}

func verifyCStates(lines []string) error {
	// Make sure we've received at least three lines. The first should be the
	// C-state header, followed by one line of keys and one or more lines of
	// C-states.
	if len(lines) < 3 {
		return errors.New("could not find any lines of C-state info")
	}

	// Verify the first line is the correct header.
	actualHeader := lines[0]
	expectedHeader := "C-States:"
	if actualHeader != expectedHeader {
		return errors.Errorf("Incorrect C-state header: got %v, want %v", actualHeader, expectedHeader)
	}

	// Verify the keys are correct.
	want := []string{"name", "time_in_state_since_last_boot_us"}
	got := strings.Split(lines[1], ",")
	if !reflect.DeepEqual(want, got) {
		return errors.Errorf("Incorrect C-state keys: got %v; want %v", got, want)
	}

	// Verify each C-state.
	for _, line := range lines[2:] {
		vals := strings.Split(line, ",")
		if len(vals) != 2 {
			return errors.Errorf("Wrong number of C-state values: got %v, want 2", len(vals))
			continue
		}

		if vals[0] == "" {
			return errors.New("Empty name")
		}

		i, err := strconv.Atoi(vals[1])
		if err != nil {
			return errors.Wrap(err, "failed to convert time_in_state_since_last_boot_us to integer")
		} else if i < 0 {
			return errors.New("invalid time_in_state_since_last_boot_us")
		}
	}

	return nil
}

func CrosHealthdProbeCPUInfo(ctx context.Context, s *testing.State) {
	if err := upstart.EnsureJobRunning(ctx, "cros_healthd"); err != nil {
		s.Fatal("Failed to start cros_healthd: ", err)
	}

	b, err := testexec.CommandContext(ctx, "telem", "--category=cpu").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to run 'telem --category=cpu': ", err)
	}

	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "command_output.txt"), b, 0644); err != nil {
		s.Errorf("Failed to write output to %s: %v", filepath.Join(s.OutDir(), "command_output.txt"), err)
	}

	// Every board should have at least one physical CPU, which contains at
	// least one logical CPU.
	lines := strings.Split(string(b), "\n")

	if len(lines) < 3 {
		s.Fatal("Could not find any lines of CPU info")
	}

	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}

	// Ignore the last line, which is just a newline.
	lines = lines[:len(lines)-1]

	// Verify the overall CpuInfo keys are correct.
	want := []string{"num_total_threads", "architecture"}
	got := strings.Split(lines[0], ",")
	if !reflect.DeepEqual(want, got) {
		s.Fatalf("Incorrect CpuInfo keys: got %v; want %v", got, want)
	}

	// Verify the CpuInfo values are reasonable.
	vals := strings.Split(lines[1], ",")
	if len(vals) != 2 {
		s.Fatalf("Wrong number of values: got %v, want 2", len(vals))
	}

	numThreads, err := strconv.Atoi(vals[0])
	if err != nil {
		s.Error("Failed to convert num_total_threads to integer: ", err)
	} else if numThreads <= 0 {
		s.Error("Invalid num_total_threads")
	}

	if vals[1] == "" {
		s.Error("Empty architecture")
	}

	// Verify each physical CPU.
	start := 2
	for i := 3; i < len(lines); i++ {
		if lines[i] == "Physical CPU:" {
			err := verifyPhysicalCPU(lines[start : i-1])
			if err != nil {
				s.Error("Failed to verify physical CPU: ", err)
			}
			start = i
		}
	}
	err = verifyPhysicalCPU(lines[start:])
	if err != nil {
		s.Error("Failed to verify physical CPU: ", err)
	}
}
