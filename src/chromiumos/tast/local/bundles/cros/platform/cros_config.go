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

	"chromiumos/tast/local/bundles/cros/platform/crosconfig"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosConfig,
		Desc: "Check config commands",
		Attr: []string{"informational"},
	})
}

func CrosConfig(ctx context.Context, s *testing.State) {
	deviceName := crosconfig.DetermineDeviceToProcess(ctx, s)
	s.Logf("Processing check configs for device %q", deviceName)

	// Get all of the commands from the cros_config_test_commands.json for the DUT.
	b, err := ioutil.ReadFile(
		filepath.Join(crosconfig.TastDir, "cros_config_test_commands.json"))
	if err != nil {
		s.Fatal("Failed to read commands data file", err)
	}

	var commandsToRun []*testexec.Cmd = crosconfig.BuildCommands(ctx, b, deviceName, s)

	// Run all of the commands on the DUT, capturing the output and building
	// a JSON file with all of the data.
	var recs []crosconfig.GoldenRec
	for _, c := range commandsToRun {
		s.Logf("Running command %q %q", c.Cmd.Path, c.Cmd.Args)
		out, err := c.Output()
		if err != nil {
			s.Errorf("Failed to run ... ", err)
			c.DumpLog(ctx)
			// p.Set(testFailures, 1.0)
		}
		trimmedOutput := strings.Trim(string(out), "\n")
		// Add a new golden record with the command key as a concatenation of all args with a '_' separator.
		rec := crosconfig.GoldenRec{CommandKey: strings.Join(c.Cmd.Args, "_"), Value: trimmedOutput}
		recs = append(recs, rec)
	}

	// Generate the golden output json file, sort the outputRecords by command key name.
	// cmp.Equal and cmp.Diff require the ordering to be stable.
	sort.Slice(recs, func(i, j int) bool { return recs[i].CommandKey < recs[j].CommandKey })
	goldenOutput := crosconfig.GoldenDBFile{DeviceName: deviceName, Records: recs}
	jsonOutput, err := json.MarshalIndent(goldenOutput, "", "  ")
	if err != nil {
		s.Fatal("Failed to generate output JSON: ", err)
	}
	fn := strings.Join([]string{deviceName, "golden_output_json"}, "_")
	err = ioutil.WriteFile(filepath.Join(s.OutDir(), fn), jsonOutput, 0644)
	if err != nil {
		s.Error("Failed to write output JSON: ", err)
	}

	// Compare the new output with the existing golden database.
	goldenFn := strings.Join([]string{deviceName, "golden_db.json"}, "_")
	crosconfig.CompareGoldenOutput(
		goldenOutput, filepath.Join(crosconfig.TastDir, goldenFn), s)
}
