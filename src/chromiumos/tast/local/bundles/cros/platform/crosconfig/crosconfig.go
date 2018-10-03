// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cronconfig verifies chromeos-config binaries and files.
package crosconfig

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"strings"

	"chromiumos/tast/testing"

	"github.com/google/go-cmp/cmp"

	"gopkg.in/yaml.v2"
)

// TODO: Replace the TastDir constant with "/usr/share/chromeos-config/tast/" once the eclass
// is updated to write the YAML and golden DB to that directory. Using /tmp/tast is great
// for testing while working through the error processing, because the
// /usr/share/chromeos-config is on a read-only filesystem.
const TastDir = "/tmp/tast/"

// Command struct to capture the binary to run with all of the arguments.
type Command struct {
	Binary string
	Args   []string
}

// Command function to build the lookup key used in the golden database.
func (c Command) Key() string {
	return c.Binary + "_" + strings.Join(c.Args, "_")
}

// JSON struct for reading and writing the golden database file.
type GoldenDBFile struct {
	DeviceName string      `json:"device_name"`
	Records    []GoldenRec `json:"records"`
}

// JSON struct for each command and its ouput.
type GoldenRec struct {
	CommandKey string `json:"command_key"`
	Value      string `json:"value"`
}

// Compare the existing golden database to the newly built output structure.
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
		s.Errorf("Failed comparision, check %q for the errors", path)
		err = ioutil.WriteFile(path, []byte(diffs), 0644)
		if err != nil {
			s.Error("Failed to write error output: ", err)
		}
	}

	return eq, diffs
}

// Read in the device_name.txt file to determine the DUT's name. Also add "all" to the list
// so that all of the common commands are run.
func DetermineDevicesToProcess(deviceNameFilename string, s *testing.State) ([]string, string) {
	// Determine all of the device's configs to process.
	deviceName := "Unknown"
	devicesToProcess := []string{"all"}

	// Read the device name from the tast tests. This name will drive
	// any device specific commands, monitoring and golden DB naming.
	b, err := ioutil.ReadFile(deviceNameFilename)
	if err != nil {
		s.Error("Failed to read device_name.txt file: ", err)
		return devicesToProcess, deviceName
	}

	deviceName = strings.Trim(string(b), "\n")
	s.Logf("Adding device %q", deviceName)
	devicesToProcess = append(devicesToProcess, deviceName)

	return devicesToProcess, deviceName
}

// Build the Command structs from the YAML configuration for the devices to be tested.
func BuildCommands(b []byte, devicesToProcess []string, s *testing.State) []Command {
	t := struct {
		Devices []struct {
			DeviceName string   `yaml:"device-name"`
			Mosys      []string `yaml:"mosys"`
			CrosConfig []string `yaml:"cros_config"`
		} `yaml:"devices"`
	}{}

	err := yaml.Unmarshal(b, &t)
	if err != nil {
		s.Fatal("Failed to unmarshal yaml to struct: ", err)
	}

	var commands []Command
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

// Create a Command struct for the configuration line.
func BuildCommand(binary string, line string) Command {
	arr := strings.Split(strings.Trim(line, "\n"), " ")
	return Command{Binary: binary, Args: arr}
}

// Determine if the string array contains the passed in string.
func Contains(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}
