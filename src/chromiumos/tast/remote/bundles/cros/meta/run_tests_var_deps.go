// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/remote/bundles/cros/meta/tastrun"
	"chromiumos/tast/testing"
)

type runTestsVarDepsParams struct {
	varValue string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     RunTestsVarDeps,
		Desc:     "Verifies that Tast can handle VarDeps",
		Contacts: []string{"oka@chromium.org", "tast-owners@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func RunTestsVarDeps(ctx context.Context, s *testing.State) {
	for _, tc := range []struct {
		name       string
		flags      []string
		wantResult []tastrun.TestResult
	}{
		{
			name:  "skip",
			flags: nil, // variable unprovided
			// Test is skipped without error.
			wantResult: []tastrun.TestResult{
				{
					Name:       "meta.LocalVarDeps",
					SkipReason: "var meta.LocalVarDeps.var not provided",
				},
			},
		},
		{
			name:  "run",
			flags: []string{"-var=meta.LocalVarDeps.var=whatever"},
			// Test runs without error.
			wantResult: []tastrun.TestResult{{
				Name: "meta.LocalVarDeps",
			}},
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			resultsDir := filepath.Join(s.OutDir(), "subtest_results")
			flags := append([]string{
				"-build=false",
				"-resultsdir=" + resultsDir,
			}, tc.flags...)

			stdout, _, err := tastrun.Exec(ctx, s, "run", flags, []string{"meta.LocalVarDeps"})
			if err != nil {
				lines := strings.Split(strings.TrimSpace(string(stdout)), "\n")
				s.Fatalf("Failed to run tast: %v (last line: %q)", err, lines[len(lines)-1])
			}
			results, err := tastrun.ParseResultsJSON(resultsDir)
			if err != nil {
				s.Fatal("Failed to get results: ", err)
			}
			if diff := cmp.Diff(results, tc.wantResult); diff != "" {
				s.Error("Results mismatch (-got +want): ", diff)
			}
		})
	}
}
