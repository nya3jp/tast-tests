// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"

	"chromiumos/tast/local/bundles/cros/platform/cros_config"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosConfig,
		Desc: "Check config commands",
		Attr: []string{"informational"},
		Data: []string{
			"cros_config_common_commands.yaml",
		},
	})
}

func CrosConfig(ctx context.Context, s *testing.State) {
	var deviceName string
	var devicesToProcess []string

	devicesToProcess, deviceName = cros_config.DetermineDevicesToProcess(
		filepath.Join(cros_config.TastDir, "device_name.txt"), s)
	s.Logf("Processing configs for devices: %q, this device %q", devicesToProcess, deviceName)

	// Get all of the commands from the cros_config_common_commands.yaml.
	bytes, err := ioutil.ReadFile(s.DataPath("cros_config_common_commands.yaml"))
	if err != nil {
		s.Fatal("Failed to read commands data file: ", err)
	}

	// Build a map of all commands to run on the DUT.
	var commandsToRun map[string]cros_config.Command = make(map[string]cros_config.Command)

	var allCommands []cros_config.Command = cros_config.BuildCommands(bytes, devicesToProcess, s)
	for _, c := range allCommands {
		commandsToRun[c.Key()] = c
	}

	// Get all of the device specific commands from the device_specific_commands.yaml.
	// This file is optional, do not fail if it doesn't exist.
	// ### add defer closes ...
	bytes, err = ioutil.ReadFile(filepath.Join(cros_config.TastDir, "device_specific_commands.yaml"))
	if err != nil {
		s.Log("Failed to read device specific commands data file: ", err)
	}

	var deviceCommands []cros_config.Command = cros_config.BuildCommands(bytes, devicesToProcess, s)
	for _, c := range deviceCommands {
		commandsToRun[c.Key()] = c
	}

	// Run all of the commands on the DUT, capturing the output and building
	// a JSON file with all of the data.
	var outputRecords []cros_config.GoldenRec
	for k, c := range commandsToRun {
		s.Logf("Running command: %q -> %q", k, c)
		cmd := testexec.CommandContext(ctx, c.Binary, c.Args...)
		out, err := cmd.Output()
		if err != nil {
			cmd.DumpLog(ctx)
			s.Error("Failed to run command: ", err)
			// p.Set(testFailures, 1.0)
		}
		trimmedOutput := strings.Trim(string(out), "\n")
		s.Logf("Command output: %q -> %q", k, trimmedOutput)
		rec := cros_config.GoldenRec{CommandKey: k, Value: trimmedOutput}
		outputRecords = append(outputRecords, rec)
	}

	// Generate the golden output json file, sort the outputRecords by command key name cmp.Equal and cmp.Diff
	// require the ordering to be stable.
	sort.Slice(outputRecords, func(i, j int) bool { return outputRecords[i].CommandKey < outputRecords[j].CommandKey })
	goldenOutput := cros_config.GoldenDBFile{DeviceName: deviceName, Records: outputRecords}
	jsonOutput, err := json.MarshalIndent(goldenOutput, "", "  ")
	if err != nil {
		s.Fatal("Failed to generate output JSON: ", err)
	}
	err = ioutil.WriteFile(filepath.Join(s.OutDir(), "golden_output.json"), jsonOutput, 0644)
	if err != nil {
		s.Error("Failed to write output JSON: ", err)
	}

	// Compare the new output with the existing golden database.
	cros_config.CompareGoldenOutput(
		goldenOutput, filepath.Join(cros_config.TastDir, deviceName+"_golden_db.json"), s)
}
