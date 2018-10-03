// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosConfig,
		Desc: "Check cros_config commands match the golden file built during image creation",
		Attr: []string{"informational"},
	})
}

// crosConfigDir defines where all of the input files and golden files are located
// on the device under test.
const crosConfigDir = "/usr/local/tmp/chromeos-config/tast/"

// deviceRecords is a JSON struct for reading and writing the golden file. A new
// golden file output file will be created and added to the output dir making it
// easier to update golden file in build configuration.
type deviceRecords struct {
	DeviceName string         `json:"device_name"`
	Records    []cmdAndOutput `json:"records"`
}

// cmdAndOutput is a JSON struct for each command and its output.
type cmdAndOutput struct {
	CommandKey string `json:"command_key"`
	Value      string `json:"value"`
}

func CrosConfig(ctx context.Context, s *testing.State) {
	cmdFile := filepath.Join(crosConfigDir, "cros_config_test_commands.json")
	// Check to see if the commands file exists.
	_, err := os.Stat(cmdFile)
	if os.IsNotExist(err) {
		s.Log("No commands file, ignoring")
		return
	}

	deviceName, err := getDeviceIdentity(ctx)
	if err != nil {
		s.Fatal("Failed to get device identity: ", err)
	}
	s.Logf("Checking configs for device %q", deviceName)

	// Get all of the commands from the cros_config_test_commands.json for the DUT. Commands
	// are generated during the build step for the dev and test images.
	commandsToRun, err := buildCommands(ctx, cmdFile, deviceName)
	if err != nil {
		s.Fatal("Failed to build commands: ", err)
	}

	// Run all of the commands on the DUT, capturing the output and building
	// a JSON file with all of the data.
	var recs []cmdAndOutput
	for _, c := range commandsToRun {
		s.Logf("Running command %q", c.Cmd.Args)
		out, err := c.Output()
		if err != nil {
			s.Errorf("Failed to run %q: %v", c.Cmd.Args, err)
			c.DumpLog(ctx)
			continue
		}
		recs = append(recs, cmdAndOutput{buildCommandKey(c), strings.TrimSpace(string(out))})
	}

	sort.Slice(recs, func(i, j int) bool { return recs[i].CommandKey < recs[j].CommandKey })
	goldenOutput := deviceRecords{deviceName, recs}
	jsonOutput, err := json.MarshalIndent(goldenOutput, "", "  ")
	if err != nil {
		s.Fatal("Failed to generate output JSON: ", err)
	}
	fn := fmt.Sprintf("%s_output.json", deviceName)
	err = ioutil.WriteFile(filepath.Join(s.OutDir(), fn), jsonOutput, 0644)
	if err != nil {
		s.Error("Failed to write output JSON: ", err)
	}

	// Compare the new output with the existing golden file.
	goldenFn := fmt.Sprintf("%s_golden_file.json", deviceName)
	compareGoldenOutput(s, goldenOutput, filepath.Join(crosConfigDir, goldenFn))
}

// buildCommandKey strips off the leading 'sh -c' from the Cmd and replaces all of the
// spaces in the actual command with colons to build the key for the golden files.
func buildCommandKey(c *testexec.Cmd) string {
	args := c.Cmd.Args
	// Make sure we strip off the leading "sh -c" so our lookup key will only be
	// the actual command and args, not any extras to run the command.
	if len(c.Cmd.Args) > 2 && c.Cmd.Args[0] == "sh" && c.Cmd.Args[1] == "-c" {
		args = c.Cmd.Args[2:]
	}
	// args is a []string that has strings with spaces, so join then replace all
	// spaces with the delimiter ':'.
	return strings.Replace(strings.Join(args, " "), " ", ":", -1)
}

// getDeviceIdentity determines the DUT's name by running 'mosys platform name'.
func getDeviceIdentity(ctx context.Context) (string, error) {
	// NOTE: we are using some of the config programs to determine
	// device identity that we are trying to test.
	c := testexec.CommandContext(ctx, "mosys", "platform", "name")
	out, err := c.Output()
	if err != nil {
		c.DumpLog(ctx)
		return "", err
	}
	return strings.ToLower(strings.TrimSpace(string(out))), nil
}

// buildCommands builds the testexec.Cmds from the JSON input file for the
// device to be tested.
func buildCommands(ctx context.Context, cmdPath string, deviceFilter string) ([]*testexec.Cmd, error) {
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

	f, err := os.Open(cmdPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&commandRecs); err != nil {
		return nil, err
	}

	var commands []*testexec.Cmd
	for _, device := range commandRecs.ChromeOS.Devices {
		if device.DeviceName != deviceFilter {
			continue
		}
		for _, cg := range device.CommandGroups {
			for _, a := range cg.Args {
				commands = append(commands, testexec.CommandContext(ctx, "sh", "-c", testexec.ShellEscape(cg.Name)+" "+a))
			}
		}
	}

	return commands, nil
}

// buildRecordsMap builds a map of the records and a list of the keys.
func buildRecordsMap(recs []cmdAndOutput) (map[string]string, []string) {
	m := make(map[string]string)
	keys := make([]string, 0, len(recs))
	for _, rec := range recs {
		m[rec.CommandKey] = rec.Value
		keys = append(keys, rec.CommandKey)
	}
	return m, keys
}

// compareGoldenOutput compares the existing golden file to the newly built output structure.
func compareGoldenOutput(s *testing.State, output deviceRecords, goldenFilename string) {
	b, err := ioutil.ReadFile(goldenFilename)
	if os.IsNotExist(err) {
		s.Logf("WARNING: No existing golden file %v, ignoring", goldenFilename)
		return
	} else if err != nil {
		s.Fatalf("Failed to read golden file %q: %v", goldenFilename, err)
	}
	var golden deviceRecords
	if err = json.Unmarshal(b, &golden); err != nil {
		s.Fatalf("Failed to unmarshal golden file %q: %v", goldenFilename, err)
	}

	// Do a deep compare of the two JSON files, if they match all is good.
	if reflect.DeepEqual(output, golden) {
		s.Log("All keys and values matched")
		return
	}

	// Verify the device names match.
	if output.DeviceName != golden.DeviceName {
		s.Fatalf("Tested device %q; want %q", output.DeviceName, golden.DeviceName)
	}

	outputMap, outputKeys := buildRecordsMap(output.Records)
	goldenMap, _ := buildRecordsMap(golden.Records)

	// Look for keys only in the new output map and missing from the golden.
	var missingGoldenKeys []string
	for _, k := range outputKeys {
		if _, ok := goldenMap[k]; !ok {
			missingGoldenKeys = append(missingGoldenKeys, k)
		}
	}

	// Iterate through the golden map checking all of the keys and values. Value differences
	// will generate an error returned in the list of errors.
	var missingOutputKeys []string
	var matchingComparisons int
	for k, v := range goldenMap {
		ov, ok := outputMap[k]
		if !ok {
			missingOutputKeys = append(missingOutputKeys, k)
			continue
		}
		if v != ov {
			s.Errorf("Key %q is %q; want %q", k, ov, v)
			continue
		}
		matchingComparisons++
	}

	if len(missingOutputKeys) > 0 {
		s.Logf("WARNING: keys %v only exist in golden file, ignoring", missingOutputKeys)
	}
	if len(missingGoldenKeys) > 0 {
		s.Logf("WARNING: keys %v only exist in new output file, ignoring", missingGoldenKeys)
	}
	s.Logf("Matched %d keys and values", matchingComparisons)
}
