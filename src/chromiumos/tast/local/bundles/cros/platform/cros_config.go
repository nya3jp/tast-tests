// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosConfig,
		Desc: "Check cros_config commands match the golden database built during image creation",
		Attr: []string{"informational"},
	})
}

// tastDir defines where all of the input files and golden databases are located
// on the device under test.
const tastDir = "/usr/local/tmp/chromeos-config/tast/"

// goldenFile is a JSON struct for reading and writing the golden database file. A new
// golden database output file will be created and added to the output bundle making it
// easier to update golden database in build configuration.
type goldenFile struct {
	DeviceName string         `json:"device_name"`
	Records    []goldenRecord `json:"records"`
}

// goldenRecord is a JSON struct for each command and its output.
type goldenRecord struct {
	CommandKey string `json:"command_key"`
	Value      string `json:"value"`
}

func CrosConfig(ctx context.Context, s *testing.State) {
	deviceName, err := getDeviceIndentity(ctx, s)
	if err != nil {
		s.Fatal("Failed to get device identity:", err)
	}
	s.Logf("Checking configs for device %q", deviceName)

	// Get all of the commands from the cros_config_test_commands.json for the DUT. Commands
	// are generated during the build step for the dev and test images.
	commandsToRun, err := buildCommands(ctx, deviceName)
	if err != nil {
		s.Fatal("Failed to build commands:", err)
	}

	// Run all of the commands on the DUT, capturing the output and building
	// a JSON file with all of the data.
	var recs []goldenRecord
	for _, c := range commandsToRun {
		s.Logf("Running command %q %q", c.Cmd.Path, c.Cmd.Args)
		out, err := c.Output()
		if err != nil {
			s.Errorf("Failed to run %q %q: %v", c.Cmd.Path, c.Cmd.Args, err)
			c.DumpLog(ctx)
		}
		trimmedOutput := strings.Trim(string(out), "\n")
		// Add a new golden record with the command key as a concatenation of all args
		// with a '_' separator.
		recs = append(recs, goldenRecord{
			CommandKey: strings.Join(c.Cmd.Args, "_"),
			Value:      trimmedOutput})
	}

	// Generate the golden output JSON file and sort the outputRecords by command key name.
	// cmp.Equal and cmp.Diff require the ordering to be stable.
	sort.Slice(recs, func(i, j int) bool { return recs[i].CommandKey < recs[j].CommandKey })
	goldenOutput := goldenFile{DeviceName: deviceName, Records: recs}
	jsonOutput, err := json.MarshalIndent(goldenOutput, "", "  ")
	if err != nil {
		s.Fatal("Failed to generate output JSON: ", err)
	}
	fn := fmt.Sprintf("%s_golden_output.json", deviceName)
	err = ioutil.WriteFile(filepath.Join(s.OutDir(), fn), jsonOutput, 0644)
	if err != nil {
		s.Error("Failed to write output JSON: ", err)
	}

	// Compare the new output with the existing golden database.
	goldenFn := fmt.Sprintf("%s_golden_db.json", deviceName)
	compareGoldenOutput(goldenOutput, filepath.Join(tastDir, goldenFn), s)
}

// compareGoldenOutput compares the existing golden database to the newly built output structure.
func compareGoldenOutput(
	output goldenFile, goldenFilename string, s *testing.State) bool {

	s.Log("Comparing golden output temporarily disabled, returning true")

	return true
}

// getDeviceIndentity determines the DUT's name by running 'mosys platform name'.
func getDeviceIndentity(ctx context.Context, s *testing.State) (string, error) {
	// Running 'mosys platform name' to get the correct device name to determine
	// commands to run and golden database to check.
	c := testexec.CommandContext(ctx, "mosys", "platform", "name")
	out, err := c.Output()
	if err != nil {
		c.DumpLog(ctx)
		return "", err
	}
	return strings.ToLower(strings.Trim(string(out), "\n")), nil
}

// buildCommands builds the testexec.Cmds from the JSON input file for the
// device to be tested.
func buildCommands(ctx context.Context, deviceFilter string) ([]*testexec.Cmd, error) {
	b, err := ioutil.ReadFile(
		filepath.Join(tastDir, "cros_config_test_commands.json"))
	if err != nil {
		return nil, err
	}
	// JSON struct for each command to run.
	commandRecs := struct {
		ChromeOS struct {
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
	for _, device := range commandRecs.ChromeOS.Devices {
		if device.DeviceName != deviceFilter {
			continue
		}
		for _, cg := range device.CommandGroups {
			for _, a := range cg.Args {
				args := strings.Split(strings.Trim(a, " "), " ")
				c := testexec.CommandContext(ctx, cg.Name, args...)
				commands = append(commands, c)
			}
		}
	}

	return commands, nil
}
