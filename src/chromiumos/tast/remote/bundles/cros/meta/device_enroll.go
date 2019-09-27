// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/remote/bundles/cros/meta/tastrun"
	"chromiumos/tast/remote/bundles/cros/policy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     DeviceEnroll,
		Desc:     "Demonstrates clearing a TPM and enrolling a device",
		Contacts: []string{"tast-owners@google.com"},
		Attr:     []string{"informational"},
	})
}

// DeviceEnroll Uses the run_tests from "meta" as an example framework for a test
// which can both reboot a device, and run a local test.
func DeviceEnroll(ctx context.Context, s *testing.State) {
	d, ok := dut.FromContext(ctx)
	if !ok {
		s.Fatal("Failed to get DUT")
	}
	if !d.Connected(ctx) {
		s.Error("Not initially connected to DUT")
	}
	// This clears the TPM (including a reboot) if owned.
	policy.ClearTPMIfOwned(ctx, s, true)

	// Test setup
	resultsDir := filepath.Join(s.OutDir(), "subtest_results")
	flags := []string{
		"-build=true",
		"-resultsdir=" + resultsDir,
	}
	testNames := []string{
		"example.Enroll",
	}

	// Run the local tests
	stdout, _, err := tastrun.Run(ctx, s, "run", flags, testNames)
	if err != nil {
		lines := strings.Split(strings.TrimSpace(string(stdout)), "\n")
		s.Fatalf("Failed to run tast: %v (last line: %q)", err, lines[len(lines)-1])
	}

	// These are subsets of the testing.Error and TestResult structs.
	type testError struct {
		Reason string `json:"reason"`
	}
	type testResult struct {
		Name   string      `json:"name"`
		Errors []testError `json:"errors"`
	}
	var results []testResult
	rf, err := os.Open(filepath.Join(resultsDir, "results.json"))
	if err != nil {
		s.Fatal("Couldn't open results file: ", err)
	}
	defer rf.Close()

	if err = json.NewDecoder(rf).Decode(&results); err != nil {
		s.Fatalf("Couldn't decode results from %v: %v", rf.Name(), err)
	}
	expResults := []testResult{
		testResult{"example.Enroll", nil},
	}
	if !reflect.DeepEqual(results, expResults) {
		s.Errorf("Got results %+v; want %+v", results, expResults)
	}

	// Clear the TPM after the test
	policy.ClearTPMIfOwned(ctx, s, true)
}
