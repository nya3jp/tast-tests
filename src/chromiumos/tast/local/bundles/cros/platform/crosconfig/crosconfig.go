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

// TastDir defines where all of the input files and golden databases are located
// on the device under test.
const TastDir = "/usr/local/tmp/chromeos-config/tast/"

// GoldenFile is a JSON struct for reading and writing the golden database file. A new
// golden database output file will be created and added to the output bundle making it
// easier to update golden database in build configuration.
type GoldenFile struct {
	DeviceName string         `json:"device_name"`
	Records    []GoldenRecord `json:"records"`
}

// GoldenRecord is a JSON struct for each command and its ouput.
type GoldenRecord struct {
	CommandKey string `json:"command_key"`
	Value      string `json:"value"`
}

// CompareGoldenOutput compares the existing golden database to the newly built output structure.
func CompareGoldenOutput(
	output GoldenFile, goldenFilename string, s *testing.State) (bool, string) {
	b, err := ioutil.ReadFile(goldenFilename)
	if err != nil {
		s.Error("Failed to read golden database file: ", err)
		return false, err.Error()
	}

	var golden GoldenFile
	err = json.Unmarshal(b, &golden)
	if err != nil {
		s.Error("Failed to unmarshal golden database", err)
		return false, err.Error()
	}

	// Compare the two structures and produce diffs if doesn't match.
	var diffs string
	var eq = cmp.Equal(output, golden)
	if !eq {
		diffs := cmp.Diff(output, golden)
		path := filepath.Join(s.OutDir(), "errors.txt")
		s.Error("cros_config output didn't match golden; see errors.txt")
		err = ioutil.WriteFile(path, []byte(diffs), 0644)
		if err != nil {
			s.Error("Failed to write error output: ", err)
		}
	}

	return eq, diffs
}

// GetDeviceIndentity determines the DUT's name by running 'mosys platform name'.
func GetDeviceIndentity(ctx context.Context, s *testing.State) (string, error) {
	// Running 'mosys platform name' to get the correct device name to determine
	// commands to run and golden database to check.
	c := BuildCommand(ctx, "mosys", "platform name")
	out, err := c.Output()
	if err != nil {
		c.DumpLog(ctx)
		return "", err
	}
	deviceName := strings.ToLower(strings.Trim(string(out), "\n"))

	return deviceName, nil
}

// BuildCommands builds the testexec.Cmds from the JSON input file for the
// device to be tested.
func BuildCommands(ctx context.Context, deviceFilter string, s *testing.State) ([]*testexec.Cmd, error) {
	b, err := ioutil.ReadFile(
		filepath.Join(TastDir, "cros_config_test_commands.json"))
	if err != nil {
		return nil, err
	}
	// JSON struct for each command to run.
	commandRecs := struct {
		ChromeOs struct {
			Devices []struct {
				DeviceName    string `json:"device-name"`
				CommandGroups []struct {
					Args []string `json:"args"`
					Name string   `json:"name"`
				} `json:"command-groups"`
			} `json:"devices"`
		} `json:"chromeos"`
	}{}

	err = json.Unmarshal(b, &commandRecs)
	if err != nil {
		return nil, err
	}

	var commands []*testexec.Cmd
	for _, device := range commandRecs.ChromeOs.Devices {
		if device.DeviceName == deviceFilter {
			for _, cg := range device.CommandGroups {
				for _, a := range cg.Args {
					c := BuildCommand(ctx, cg.Name, a)
					commands = append(commands, c)
				}
			}
		}
	}

	return commands, nil
}

// BuildCommand creates a testexec.Cmd for the configuration line.
func BuildCommand(ctx context.Context, binary string, line string) *testexec.Cmd {
	args := strings.Split(strings.Trim(line, " "), " ")
	return testexec.CommandContext(ctx, binary, args...)
}
