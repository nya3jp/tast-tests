// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crosconfig verifies chromeos-config binaries and files.
package crosconfig

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// TODO: Replace the TastDir constant with "/usr/share/chromeos-config/tast/" once the eclass
// is updated to write the YAML and golden DB to that directory. Using /tmp/tast is great
// for testing while working through the error processing, because the
// /usr/share/chromeos-config is on a read-only filesystem.

// TastDir defines where all of the input files are located.
const TastDir = "/tmp/tast/"

// GoldenDBFile JSON struct for reading and writing the golden database file.
type GoldenDBFile struct {
	DeviceName string      `json:"device_name"`
	Records    []GoldenRec `json:"records"`
}

// GoldenRec JSON struct for each command and its ouput.
type GoldenRec struct {
	CommandKey string `json:"command_key"`
	Value      string `json:"value"`
}

// CommandRec JSON struct for each command to run.
type CommandRec struct {
	Chromeos struct {
		Devices []struct {
			DeviceName    string `json:"device-name"`
			CommandGroups []struct {
				Args []string `json:"args"`
				Name string   `json:"name"`
			} `json:"command-groups"`
		} `json:"devices"`
	} `json:"chromeos"`
}

// CompareGoldenOutput compares the existing golden database to the newly built output structure.
func CompareGoldenOutput(
	output GoldenDBFile, goldenFilename string, s *testing.State) (bool, string) {
	b, err := ioutil.ReadFile(goldenFilename)
	if err != nil {
		s.Error("Failed to read Golden DB File: ", err)
	}

	var golden GoldenDBFile
	// TODO: add error check.
	json.Unmarshal(b, &golden)

	// Compare the two structures and produce diffs if doesn't match.
	var diffs string
	var eq = cmp.Equal(output, golden)
	if !eq {
		diffs := cmp.Diff(output, golden)
		path := filepath.Join(s.OutDir(), "errors.txt")
		s.Errorf("Failed diffs: %q", diffs)
		s.Errorf("Failed comparision, check %q for the errors", path)
		err = ioutil.WriteFile(path, []byte(diffs), 0644)
		if err != nil {
			s.Error("Failed to write error output: ", err)
		}
	}

	return eq, diffs
}

// DetermineDeviceToProcess determines the DUT's name by running 'mosys platform name'.
func DetermineDeviceToProcess(ctx context.Context, s *testing.State) string {
	// Running 'mosys platform name' to get the correct device name to determine
	// commands to run and golden database to check.
	// TODO: Figure out a better way to determine this without running mosys.
	c := BuildCommand(ctx, "mosys", "platform name")
	out, err := c.Output()
	if err != nil {
		c.DumpLog(ctx)
		s.Fatalf("Failed to determine the device name", err)
		// p.Set(testFailures, 1.0)
	}
	deviceName := strings.ToLower(strings.Trim(string(out), "\n"))

	s.Logf("Processing device %q", deviceName)

	return deviceName
}

// BuildCommands builds the CommandRec structs from the JSON configuration for the devices to be tested.
func BuildCommands(ctx context.Context, b []byte, deviceToProcess string, s *testing.State) []*testexec.Cmd {
	var commandRecs CommandRec
	err := json.Unmarshal(b, &commandRecs)
	if err != nil {
		s.Fatalf("Failed to unmarshal json commands", err)
	}

	var commands []*testexec.Cmd

	for _, device := range commandRecs.Chromeos.Devices {
		if device.DeviceName == deviceToProcess {
			for _, cg := range device.CommandGroups {
				for _, a := range cg.Args {
					c := BuildCommand(ctx, cg.Name, a)
					commands = append(commands, c)
				}
			}
		}
	}

	return commands
}

// BuildCommand creates a testexec.Cmd for the configuration line.
func BuildCommand(ctx context.Context, binary string, line string) *testexec.Cmd {
	arr := strings.Split(strings.Trim(line, "\n"), " ")
	c := testexec.CommandContext(ctx, binary, arr...)
	return c
}

// Contains determines if the string array contains the passed in string.
func Contains(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}
