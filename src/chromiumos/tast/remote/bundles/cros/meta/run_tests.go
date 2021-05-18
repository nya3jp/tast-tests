// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"chromiumos/tast/remote/bundles/cros/meta/tastrun"
	"chromiumos/tast/testing"
)

// runTestsParam is a parameter to the meta.RunTests test.
type runTestsParam struct {
	tests       []string
	wantResults []tastrun.TestResult
	wantFiles   map[string]string // special value exists/notExists is allowed
}

const (
	localVarValue  = "a_local_var"
	remoteVarValue = "a_remote_var"

	// exists can be set to runTestsParam.Files to indicate the file should exist
	// without specifying its exact content.
	exists = "*exists*"

	// notexists can be set to runTestsParam.Files to indicate the file should
	// not exist.
	notExists = "*notexists*"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     RunTests,
		Desc:     "Verifies that Tast can run tests",
		Contacts: []string{"nya@chromium.org", "tast-owners@google.com"},
		Params: []testing.Param{{
			Name: "faillog",
			Val: runTestsParam{
				tests: []string{
					"meta.LocalFail",
					"meta.LocalPass",
					"meta.RemoteFail",
					"meta.RemotePass",
				},
				wantResults: []tastrun.TestResult{
					{Name: "meta.LocalFail", Errors: []tastrun.TestError{{Reason: "Failed"}}},
					{Name: "meta.LocalPass"},
					{Name: "meta.RemoteFail", Errors: []tastrun.TestError{{Reason: "Failed"}}},
					{Name: "meta.RemotePass"},
				},
				wantFiles: map[string]string{
					"tests/meta.LocalFail/faillog/ps.txt":  exists,
					"tests/meta.LocalPass/faillog/ps.txt":  notExists,
					"tests/meta.RemoteFail/faillog/ps.txt": exists,
					"tests/meta.RemotePass/faillog/ps.txt": notExists,
				},
			},
			ExtraAttr: []string{"group:mainline", "informational"},
		}, {
			Name: "files",
			Val: runTestsParam{
				tests: []string{
					"meta.LocalFiles",
				},
				wantResults: []tastrun.TestResult{
					{Name: "meta.LocalFiles"},
				},
				wantFiles: map[string]string{
					"tests/meta.LocalFiles/local_files_internal.txt":          "This is an internal data file.\n",
					"tests/meta.LocalFiles/local_files_external.txt":          "This is an external data file.\n",
					"fixtures/metaDataFilesFixture/fixture_data_internal.txt": "This is an internal data file.\n",
					"fixtures/metaDataFilesFixture/fixture_data_external.txt": "This is an external data file.\n",
				},
			},
			ExtraAttr: []string{"group:mainline", "group:meta"},
		}, {
			Name: "files_remote",
			Val: runTestsParam{
				tests: []string{
					"meta.RemoteFiles",
				},
				wantResults: []tastrun.TestResult{
					{Name: "meta.RemoteFiles"},
				},
				wantFiles: map[string]string{
					"tests/meta.RemoteFiles/remote_files_internal.txt": "This is an internal data file for remote tests.\n",
					"tests/meta.RemoteFiles/remote_files_external.txt": "This is an external data file for remote tests.\n",
				},
			},
			ExtraAttr: []string{"group:mainline", "informational"},
		}, {
			Name: "panic",
			Val: runTestsParam{
				tests: []string{
					"meta.LocalPanic",
				},
				wantResults: []tastrun.TestResult{
					{Name: "meta.LocalPanic", Errors: []tastrun.TestError{{Reason: "Panic: intentionally panicking"}}},
				},
			},
			ExtraAttr: []string{"group:mainline", "group:meta"},
		}, {
			Name: "vars",
			Val: runTestsParam{
				tests: []string{
					"meta.LocalVars",
					"meta.RemoteVars",
				},
				wantResults: []tastrun.TestResult{
					{Name: "meta.LocalVars"},
					{Name: "meta.RemoteVars"},
				},
				wantFiles: map[string]string{
					"tests/meta.LocalVars/var.txt":  localVarValue,
					"tests/meta.RemoteVars/var.txt": remoteVarValue,
				},
			},
			ExtraAttr: []string{"group:mainline", "group:meta"},
		}},
	})
}

func RunTests(ctx context.Context, s *testing.State) {
	param := s.Param().(runTestsParam)
	resultsDir := filepath.Join(s.OutDir(), "subtest_results")
	flags := []string{
		"-resultsdir=" + resultsDir,
		"-var=meta.LocalVars.var=" + localVarValue,
		"-var=meta.RemoteVars.var=" + remoteVarValue,
	}
	stdout, _, err := tastrun.Exec(ctx, s, "run", flags, param.tests)
	if err != nil {
		lines := strings.Split(strings.TrimSpace(string(stdout)), "\n")
		s.Fatalf("Failed to run tast: %v (last line: %q)", err, lines[len(lines)-1])
	}

	results, err := tastrun.ParseResultsJSON(resultsDir)
	if err != nil {
		s.Fatal("Failed to get results for tests ", param.tests)
	}

	if !reflect.DeepEqual(results, param.wantResults) {
		s.Errorf("Got results %+v; want %+v", results, param.wantResults)
	}

	// Some tests copy their data files to the output directory.
	// These filenames and corresponding contents are hardcoded in the tests.
	for p, v := range param.wantFiles {
		p = filepath.Join(resultsDir, p)
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
