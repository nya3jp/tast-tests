// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"

	"chromiumos/tast/remote/bundles/cros/meta/tastrun"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RunTests,
		Desc: "Verifies that Tast can run tests",
		Attr: []string{"informational"},
	})
}

func RunTests(s *testing.State) {
	testNames := []string{
		"meta.LocalFiles",
		"meta.LocalPanic",
		"meta.RemoteFiles",
	}
	resultsDir := filepath.Join(s.OutDir(), "subtest_results")
	_, err := tastrun.Run(s, "run", []string{"-build=false", "-resultsdir=" + resultsDir}, testNames)
	if err != nil {
		s.Fatal("Failed to run tast: ", err)
	}

	// These are subsets of the testing.Error and TestResult structs.
	type testError struct{}
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
		testResult{"meta.LocalFiles", nil},
		testResult{"meta.LocalPanic", []testError{testError{}}},
		testResult{"meta.RemoteFiles", nil},
	}
	if !reflect.DeepEqual(results, expResults) {
		s.Errorf("Got results %+v; want %+v", results, expResults)
	}

	// LocalFiles and RemoteFiles copy their data files to the output directory.
	// These filenames and corresponding contents are hardcoded in the tests.
	for p, v := range map[string]string{
		"meta.LocalFiles/local_files_internal.txt": "This is an internal data file.\n",
		"meta.LocalFiles/local_files_external.txt": "This is an external data file.\n",
		"meta.RemoteFiles/remote_files_data.txt":   "This is a data file for a remote test.\n",
	} {
		p = filepath.Join(resultsDir, "tests", p)
		if b, err := ioutil.ReadFile(p); err != nil {
			s.Errorf("Failed to read output file %v: %v", p, err)
		} else if string(b) != v {
			s.Errorf("Output file %v contains %q instead of %q", p, string(b), v)
		}
	}
}
