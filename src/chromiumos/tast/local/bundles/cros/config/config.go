// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"

	"github.com/google/go-cmp/cmp"
	"gopkg.in/yaml.v2"
)

// ### const TastDir = "/usr/share/chromeos-config/tast/"
const TastDir = "/tmp/tast/"

func init() {
	testing.AddTest(&testing.Test{
		Func: Config,
		Desc: "Check config commands",
		Attr: []string{"informational"},
		Data: []string{
			"common_commands.yaml",
		},
	})
}

type T struct {
	Devices []struct {
		DeviceName string   `yaml:"device-name"`
		Mosys      []string `yaml:"mosys"`
		CrosConfig []string `yaml:"cros_config"`
	} `yaml:"devices"`
}

type GoldenDBFile struct {
	DeviceName string      `json:"device_name"`
	Records    []GoldenRec `json:"records"`
}

type GoldenRec struct {
	CommandKey string `json:"command_key"`
	Value      string `json:"value"`
}

type Command struct {
	Binary string
	Args   []string
}

func (c Command) Key() string {
	return c.Binary + "_" + strings.Join(c.Args, "_")
}

func Config(ctx context.Context, s *testing.State) {
	var deviceName string
	var devicesToProcess []string
	devicesToProcess, deviceName = DetermineDevicesToProcess(s)
	s.Logf("Processing configs for devices: %q, this device %q", devicesToProcess, deviceName)

	// Get all of the commands from the common_commands.yaml.
	bytes, err := ioutil.ReadFile(s.DataPath("common_commands.yaml"))
	if err != nil {
		s.Fatal("Failed to read commands data file: ", err)
	}

	// Build a map of all commands to run on the DUT.
	var commandsToRun map[string]Command = make(map[string]Command)

	var allCommands []Command = BuildCommands(bytes, devicesToProcess, s)
	for _, c := range allCommands {
		commandsToRun[c.Key()] = c
	}

	// Get all of the device specific commands from the device_specific_commands.yaml.
	// This file is optional, do not fail if it doesn't exist.
	// ### add defer closes ...
	bytes, err = ioutil.ReadFile(filepath.Join(TastDir, "device_specific_commands.yaml"))
	if err != nil {
		s.Log("Failed to read device specific commands data file: ", err)
	}

	var deviceCommands []Command = BuildCommands(bytes, devicesToProcess, s)
	for _, c := range deviceCommands {
		commandsToRun[c.Key()] = c
	}

	// Run all of the commands on the DUT, capturing the output and building
	// a JSON file with all of the data.
	var outputRecords []GoldenRec
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
		rec := GoldenRec{CommandKey: k, Value: trimmedOutput}
		outputRecords = append(outputRecords, rec)
	}

	// Generate the golden output json file, sort the outputRecords by command key name cmp.Equal and cmp.Diff
	// require the ordering to be stable.
	sort.Slice(outputRecords, func(i, j int) bool { return outputRecords[i].CommandKey < outputRecords[j].CommandKey })
	goldenOutput := GoldenDBFile{DeviceName: deviceName, Records: outputRecords}
	jsonOutput, err := json.MarshalIndent(goldenOutput, "", "  ")
	if err != nil {
		s.Fatal("Failed to generate output JSON: ", err)
	}
	s.Logf("Golden output: %q", string(jsonOutput))
	err = ioutil.WriteFile(filepath.Join(s.OutDir(), "golden_output.json"), jsonOutput, 0644)
	if err != nil {
		s.Error("Failed to write output JSON: ", err)
	}

	// Compare the new output with the existing golden database.
	CompareGoldenOutput(goldenOutput, filepath.Join(TastDir, deviceName+"_golden_db.json"), s)
}

func CompareGoldenOutput(output GoldenDBFile, goldenFilename string, s *testing.State) {
	s.Logf("Trying to read Golden DB File: %q", goldenFilename)
	bytes, err := ioutil.ReadFile(goldenFilename)
	if err != nil {
		s.Error("Failed to read Golden DB File: ", err)
	}
	var golden GoldenDBFile
	json.Unmarshal(bytes, &golden)
	// Do the comparison of the output with the existing golden database.
	// ### increment metrics
	if output.DeviceName != golden.DeviceName {
		s.Errorf("Failed device name comparison new=%q <> golden=%q", output.DeviceName, golden.DeviceName)
	} else {
		s.Logf("device name comparison new=%q == golden=%q", output.DeviceName, golden.DeviceName)
	}

	structEq := cmp.Equal(output, golden)
	s.Logf("New output struct matches existing golden struct: %t", structEq)
	if !structEq {
		diff := cmp.Diff(output, golden)
		s.Errorf("Failed record comparison diff: %q", diff)
	}
}

func DetermineDevicesToProcess(s *testing.State) ([]string, string) {
	// Determine all of the device's configs to process.
	var deviceName string = "Unknown"
	var devicesToProcess []string
	devicesToProcess = append(devicesToProcess, "all")

	// Read the device name from the tast tests. This name will drive
	// any device specific commands, monitoring and golden DB naming.
	bytes, err := ioutil.ReadFile(filepath.Join(TastDir, "device_name.txt"))
	if err != nil {
		s.Error("Failed to read device_name.txt file: ", err)
	} else {
		deviceName = strings.Trim(string(bytes), "\n")
		s.Logf("### Adding device: %q", deviceName)
		devicesToProcess = append(devicesToProcess, deviceName)
	}

	return devicesToProcess, deviceName
}

func BuildCommands(b []byte, devicesToProcess []string, s *testing.State) []Command {
	var commands []Command

	t := T{}
	err := yaml.Unmarshal(b, &t)
	if err != nil {
		s.Fatal("Failed to unmarshall yaml to struct: ", err)
	}

	for _, v := range t.Devices {
		if Contains(devicesToProcess, v.DeviceName) {
			for _, args := range v.Mosys {
				commands = append(commands, BuildCommand("mosys", args))
			}
			for _, args := range v.CrosConfig {
				commands = append(commands, BuildCommand("cros_config", args))
			}
		}
	}

	return commands
}

func BuildCommand(binary string, line string) Command {
	arr := strings.Split(strings.Trim(line, "\n"), " ")
	return Command{Binary: binary, Args: arr}
}

func Contains(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}
