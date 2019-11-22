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
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     CrosConfig,
		Desc:     "Check cros_config commands match the golden file built during image creation",
		Contacts: []string{"nednguyen@chromium.org", "shapiroc@chromium.org"},
		Attr:     []string{"group:mainline"},
	})
}

// crosConfigDir defines where all of the input files and golden files are located
// on the device under test.
const crosConfigDir = "/usr/local/tmp/chromeos-config/tast/"

// deviceRecords is a JSON struct for reading and writing the golden file. A new
// golden output file will be created and added to the output dir making it
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
		s.Logf("Running command %q", c)
		out, err := testexec.CommandContext(ctx, "sh", "-c", c).Output()
		if err != nil {
			s.Errorf("Failed to run %q: %v", c, err)
			continue
		}
		// Use ':' as key delim between commands for readability.
		recs = append(recs, cmdAndOutput{strings.Replace(c, " ", ":", -1), strings.TrimSpace(string(out))})
	}

	sort.Slice(recs, func(i, j int) bool { return recs[i].CommandKey < recs[j].CommandKey })
	output := deviceRecords{deviceName, recs}
	jsonOutput, err := json.MarshalIndent(output, "", "  ")
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
	compareGoldenOutput(s, output, filepath.Join(crosConfigDir, goldenFn))
}

// getDeviceIdentity determines the DUT's identity by getting /:name from cros_config.
// This identity will be expanded in the future to include SKU and whitelabels.
// See go/cros-domain-model for details and naming
// Returns output such as "eve", "scarlet", or "nautilus".
func getDeviceIdentity(ctx context.Context) (string, error) {
	// NOTE: we are using some of the config programs to determine
	// device identity that we are trying to test.
	c := testexec.CommandContext(ctx, "cros_config", "/", "name")
	out, err := c.Output()
	if err != nil {
		c.DumpLog(ctx)
		return "", err
	}
	return string(out), nil
}

// buildCommands returns the shell quoted command lines rom the JSON input file for the
// device to be tested.
func buildCommands(ctx context.Context, cmdPath string, deviceFilter string) ([]string, error) {
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

	var commands []string
	for _, device := range commandRecs.ChromeOS.Devices {
		if device.DeviceName != deviceFilter {
			continue
		}
		for _, cg := range device.CommandGroups {
			for _, a := range cg.Args {
				commands = append(commands, shutil.Escape(cg.Name)+" "+a)
			}
		}
	}

	return commands, nil
}

// buildRecordsMap builds a map of the records.
func buildRecordsMap(recs []cmdAndOutput) map[string]string {
	m := make(map[string]string)
	for _, rec := range recs {
		m[rec.CommandKey] = rec.Value
	}
	return m
}

// compareGoldenOutput compares the existing golden file to the newly built output structure.
func compareGoldenOutput(s *testing.State, output deviceRecords, goldenFilename string) {
	b, err := ioutil.ReadFile(goldenFilename)
	if os.IsNotExist(err) {
		// By design (go/cros-config-test-coverage), no golden file will only warn. This
		// is most likely a device in development and should not fail CQ runs.
		s.Logf("WARNING: No existing golden file %v, ignoring", goldenFilename)
		return
	}
	if err != nil {
		s.Fatalf("Failed to read golden file %v: %v", goldenFilename, err)
	}
	var golden deviceRecords
	if err = json.Unmarshal(b, &golden); err != nil {
		s.Fatalf("Failed to unmarshal golden file %v: %v", goldenFilename, err)
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

	outputMap := buildRecordsMap(output.Records)
	goldenMap := buildRecordsMap(golden.Records)

	// Look for keys only in the new output map and missing from the golden.
	var missingGoldenKeys []string
	for k := range outputMap {
		if _, ok := goldenMap[k]; !ok {
			missingGoldenKeys = append(missingGoldenKeys, k)
		}
	}

	// Iterate through the golden map checking all of the keys and values. Value differences
	// will generate an error returned in the list of errors.
	var missingOutputKeys []string
	var matchingComparisons int
	for k, v := range goldenMap {
		if ov, ok := outputMap[k]; !ok {
			missingOutputKeys = append(missingOutputKeys, k)
		} else if v != ov {
			s.Errorf("Key %q is %q; want %q", k, ov, v)
		} else {
			matchingComparisons++
		}
	}

	// By design (go/cros-config-test-coverage), missing keys will only warn. This
	// is most likely a device in development and should not fail CQ runs.
	if len(missingOutputKeys) > 0 {
		s.Logf("WARNING: keys %v only exist in golden file, ignoring", missingOutputKeys)
	}
	if len(missingGoldenKeys) > 0 {
		s.Logf("WARNING: keys %v only exist in new output file, ignoring", missingGoldenKeys)
	}
	s.Logf("Matched %d keys and values", matchingComparisons)
}
