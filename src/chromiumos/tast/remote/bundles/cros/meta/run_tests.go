// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"chromiumos/tast/remote/bundles/cros/meta/tastrun"
	"chromiumos/tast/testing"
)

// These are subsets of the testing.Error and TestResult structs.
type testError struct {
	Reason string `json:"reason"`
}
type testResult struct {
	Name   string      `json:"name"`
	Errors []testError `json:"errors"`
}

// runTestsParam is a parameter to the meta.RunTests test.
type runTestsParam struct {
	Tests   []string
	Results []testResult
	Files   map[string]string
}

const (
	localVarValue  = "a_local_var"
	remoteVarValue = "a_remote_var"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     RunTests,
		Desc:     "Verifies that Tast can run tests",
		Contacts: []string{"nya@chromium.org", "tast-owners@google.com"},
		Attr:     []string{"group:mainline", "group:meta"},
		Params: []testing.Param{{
			Name: "files",
			Val: runTestsParam{
				Tests: []string{
					"meta.LocalFiles",
					"meta.RemoteFiles",
				},
				Results: []testResult{
					{Name: "meta.LocalFiles"},
					{Name: "meta.RemoteFiles"},
				},
				Files: map[string]string{
					"meta.LocalFiles/local_files_internal.txt": "This is an internal data file.\n",
					"meta.LocalFiles/local_files_external.txt": "This is an external data file.\n",
					"meta.RemoteFiles/remote_files_data.txt":   "This is a data file for a remote test.\n",
				},
			},
		}, {
			Name: "panic",
			Val: runTestsParam{
				Tests: []string{
					"meta.LocalPanic",
				},
				Results: []testResult{
					{Name: "meta.LocalPanic", Errors: []testError{{"Panic: intentionally panicking"}}},
				},
			},
		}, {
			Name: "vars",
			Val: runTestsParam{
				Tests: []string{
					"meta.LocalVars",
					"meta.RemoteVars",
				},
				Results: []testResult{
					{Name: "meta.LocalVars"},
					{Name: "meta.RemoteVars"},
				},
				Files: map[string]string{
					"meta.LocalVars/var.txt":  localVarValue,
					"meta.RemoteVars/var.txt": remoteVarValue,
				},
			},
		}},
	})
}

func RunTests(ctx context.Context, s *testing.State) {
	param := s.Param().(runTestsParam)
	resultsDir := filepath.Join(s.OutDir(), "subtest_results")
	flags := []string{
		"-build=false",
		"-resultsdir=" + resultsDir,
		"-var=meta.LocalVars.var=" + localVarValue,
		"-var=meta.RemoteVars.var=" + remoteVarValue,
	}
	// This test executes tast with -build=false to run already-installed copies of these helper tests.
	// If it is run manually with "tast run -build=true", the tast-remote-tests-cros package should be
	// built for the host and tast-local-tests-cros should be deployed to the DUT first.
	stdout, _, err := tastrun.Run(ctx, s, "run", flags, param.Tests)
	if err != nil {
		lines := strings.Split(strings.TrimSpace(string(stdout)), "\n")
		s.Fatalf("Failed to run tast: %v (last line: %q)", err, lines[len(lines)-1])
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
	if !reflect.DeepEqual(results, param.Results) {
		s.Errorf("Got results %+v; want %+v", results, param.Results)
	}

	// Some tests copy their data files to the output directory.
	// These filenames and corresponding contents are hardcoded in the tests.
	for p, v := range param.Files {
		p = filepath.Join(resultsDir, "tests", p)
		if b, err := ioutil.ReadFile(p); err != nil {
			s.Errorf("Failed to read output file %v: %v", p, err)
		} else if string(b) != v {
			s.Errorf("Output file %v contains %q instead of %q", p, string(b), v)
		}
	}
}
