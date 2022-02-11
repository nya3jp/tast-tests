// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"chromiumos/tast/remote/bundles/cros/meta/tastrun"
	"chromiumos/tast/testing"
)

// runCUJParam is a parameter to the RunCUJ test.
type runCUJParam struct {
	tests     []string
	iteration int
	retry     int
}

const (
	formalIteration = 10
	formalRetry     = 4 // Total 5 times of execution if one test fails.

	quickIteration = 1
	quickRetry     = 1
)

var basicTests = []string{
	"example.Pass",
	"example.Fail",
	"ui.TabSwitchCUJ2.basic_noproxy",
	"ui.QuickCheckCUJ2.basic_wakeup",
	"ui.QuickCheckCUJ2.basic_unlock",
	"ui.EverydayMultiTaskingCUJ.basic_ytmusic",
	"ui.VideoCUJ2.basic_youtube_web",
	"ui.VideoCUJ2.basic_youtube_app",
	"ui.GoogleMeetCUJ.basic_two",
	"ui.GoogleMeetCUJ.basic_small",
	"ui.GoogleMeetCUJ.basic_large",
	"ui.GoogleMeetCUJ.basic_class",
}
var plusTests = append(basicTests,
	"ui.TabSwitchCUJ2.plus_noproxy",
	"ui.VideoCUJ2.plus_youtube_app",
	"ui.EverydayMultiTaskingCUJ.plus_ytmusic",
	"ui.VideoCUJ2.plus_youtube_web",
	"ui.GoogleMeetCUJ.plus_class",
	"ui.GoogleMeetCUJ.plus_large",
)
var premiumTests = append(plusTests,
	"ui.TabSwitchCUJ2.premium_noproxy",
	"ui.GoogleMeetCUJ.premium_large",
	"ui.ExtendedDisplayCUJ.premium_meet_large",
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     RunCUJ,
		Desc:     "Run CUJ tests",
		Contacts: []string{"xliu@cienet.com"},
		Params: []testing.Param{{
			Name: "basic",
			Val: runCUJParam{
				tests:     basicTests,
				iteration: formalIteration,
				retry:     formalRetry,
			},
			ExtraAttr: []string{"group:mainline", "informational"},
		}, {
			Name: "plus",
			Val: runCUJParam{
				tests:     plusTests,
				iteration: formalIteration,
				retry:     formalRetry,
			},
			ExtraAttr: []string{"group:mainline", "informational"},
		}, {
			Name: "premium",
			Val: runCUJParam{
				tests:     premiumTests,
				iteration: formalIteration,
				retry:     formalRetry,
			},
			ExtraAttr: []string{"group:mainline", "informational"},
		}, {
			Name: "basic_quick",
			Val: runCUJParam{
				tests:     basicTests,
				iteration: quickIteration,
				retry:     quickRetry,
			},
			ExtraAttr: []string{"group:mainline", "informational"},
		}, {
			Name: "plus_quick",
			Val: runCUJParam{
				tests:     plusTests,
				iteration: quickIteration,
				retry:     quickRetry,
			},
			ExtraAttr: []string{"group:mainline", "informational"},
		}, {
			Name: "premium_quick",
			Val: runCUJParam{
				tests:     premiumTests,
				iteration: quickIteration,
				retry:     quickRetry,
			},
			ExtraAttr: []string{"group:mainline", "informational"},
		}},
	})
}

func RunCUJ(ctx context.Context, s *testing.State) {
	param := s.Param().(runCUJParam)

	s.Logf("Run %d iterations with %d retries for failed tests. Tests to execute: %v",
		param.iteration, param.retry, param.tests)
	for i := 1; i <= param.iteration; i++ {
		s.Log("Running iteration ", i)
		resultsDir := filepath.Join(s.OutDir(), fmt.Sprintf("subtest_results_%d", i))

		flags := []string{
			"-resultsdir=" + resultsDir,
			fmt.Sprintf("-retries=%d", param.retry),
		}
		for key, value := range s.Vars() {
			flags = append(flags, fmt.Sprintf("-var=%s=%s", key, value))
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
		s.Log("Check test log: ", resultsDir)
		for _, result := range results {
			if len(result.Errors) != 0 {
				s.Fatalf("Failed to execute test %s in iteration %d with test error: %v", result.Name, i, result.Errors)
			}
			if result.SkipReason != "" {
				s.Fatalf("Skipped to execute test %s in iteration %d with reason: %s", result.Name, i, result.SkipReason)
			}
		}

	}
}
