// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
    "context"
    "encoding/json"
    // "io/ioutil"
    "os"
    "path/filepath"
    "reflect"
    "strings"

    "chromiumos/tast/remote/bundles/cros/meta/tastrun"
    "chromiumos/tast/testing"
    "chromiumos/tast/dut"

    "chromiumos/tast/remote/bundles/cros/policy"


)

func init() {
    testing.AddTest(&testing.Test{
        Func:     Fexample,
        Desc:     "Demonstrates connecting to and disconnecting from DUT",
        Contacts: []string{"tast-owners@google.com"},

    })
}

func Fexample(ctx context.Context, s *testing.State) {
    d, ok := dut.FromContext(ctx)
    if !ok {
        s.Fatal("Failed to get DUT")
    }
    if !d.Connected(ctx) {
        s.Error("Not initially connected to DUT")
    }

    
    policy.ClearTPMIfOwned(ctx, s, true)



    resultsDir := filepath.Join(s.OutDir(), "subtest_results")
    // const (
    //     localVarValue  = "a_local_var"
    //     remoteVarValue = "a_remote_var"
    // )
    flags := []string{
        "-build=true",
        "-resultsdir=" + resultsDir,
    }
    // This test executes tast with -build=false to run already-installed copies of these helper tests.
    // If it is run manually with "tast run -build=true", the tast-remote-tests-cros package should be
    // built for the host and tast-local-tests-cros should be deployed to the DUT first.
    testNames := []string{
        "ui.ChromeLogin",
    }
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
        testResult{"ui.ChromeLogin", nil},
    }
    if !reflect.DeepEqual(results, expResults) {
        s.Errorf("Got results %+v; want %+v", results, expResults)
    }
    policy.ClearTPMIfOwned(ctx, s, true)


}
