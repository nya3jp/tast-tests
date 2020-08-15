// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

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
	tests       []string
	wantResults []testResult
	wantFiles   map[string]string // special value exists/notExists is allowed
}

const (

	// exists can be set to runTestsParam.Files to indicate the file should exist
	// without specifying its exact content.
	exists = "*exists*"

	// notexists can be set to runTestsParam.Files to indicate the file should
	// not exist.
	notExists = "*notexists*"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     TraceReplayRemote,
		Desc:     "Verifies that Tast can run tests",
		Contacts: []string{"nya@chromium.org", "tast-owners@google.com"},
		Params: []testing.Param{{
			Name: "glxgears_stable",
			Val: runTestsParam{
				tests: []string{
					"graphics.TraceReplay.glxgears_stable",
				},
				wantResults: []testResult{
					{Name: "graphics.TraceReplay.glxgears_stable"},
				},
			},
			ExtraAttr: []string{"group:graphics", "graphics_trace"},
		}, {
			Name: "glxgears_unstable",
			Val: runTestsParam{
				tests: []string{
					"graphics.TraceReplay.glxgears_unstable",
				},
				wantResults: []testResult{
					{Name: "graphics.TraceReplay.glxgears_unstable"},
				},
			},
			ExtraAttr: []string{"group:graphics", "graphics_trace"},
		},
		},
	})
}

// TraceReplayRemote replays traces and reboot the machine.
func TraceReplayRemote(ctx context.Context, s *testing.State) {
	// Reboot device to cleanup the state.
	defer func() {
		s.Log("Rebooting DUT")
		if err := s.DUT().Reboot(ctx); err != nil {
			s.Fatal("Failed to reboot: ", err)
		}
	}()

	param := s.Param().(runTestsParam)
	resultsDir := filepath.Join(s.OutDir(), "subtest_results")
	flags := []string{
		"-build=true",
		"-resultsdir=" + resultsDir,
	}
	stdout, _, err := tastrun.Run(ctx, s, "run", flags, param.tests)
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
	if !reflect.DeepEqual(results, param.wantResults) {
		s.Errorf("Got results %+v; want %+v", results, param.wantResults)
	}

	// Some tests copy their data files to the output directory.
	// These filenames and corresponding contents are hardcoded in the tests.
	for p, v := range param.wantFiles {
		p = filepath.Join(resultsDir, "tests", p)
		b, err := ioutil.ReadFile(p)
		if v == notExists {
			if err == nil {
				s.Errorf("Output file %v exists unexpectedly", p)
			} else if !os.IsNotExist(err) {
				s.Errorf("Failed to read output file %v: %v", p, err)
			}
			continue
		}
		if err != nil {
			s.Errorf("Failed to read output file %v: %v", p, err)
			continue
		}
		if v == exists {
			continue
		}
		if string(b) != v {
			s.Errorf("Output file %v contains %q instead of %q", p, string(b), v)
		}
	}
}
