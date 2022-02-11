// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

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
	quickRetry     = 0 // No retry on failure.
)

var basicTests = []string{
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
	"ui.EverydayMultiTaskingCUJ.plus_ytmusic",
	"ui.GoogleMeetCUJ.plus_large",
	"ui.GoogleMeetCUJ.plus_class",
	"ui.ExtendedDisplayCUJ.plus_video_youtube_web",
)
var premiumTests = append(plusTests,
	"ui.TabSwitchCUJ2.premium_noproxy",
	"ui.VideoCUJ2.premium_youtube_web",
	"ui.VideoCUJ2.premium_youtube_app",
	"ui.GoogleMeetCUJ.premium_large",
	"ui.ExtendedDisplayCUJ.premium_meet_large",
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     RunCUJ,
		Desc:     "Run CUJ tests",
		Contacts: []string{"xliu@cienet.com", "cienet-development@googlegroups.com"},
		Params: []testing.Param{{
			Name: "tabswitchcuj2_basic_noproxy",
			Val: runCUJParam{
				// tests:     []string{"ui.TabSwitchCUJ2.basic_noproxy"},
				tests:     []string{"ui.TabSwitchCUJ2.basic_noproxy"},
				iteration: formalIteration,
				retry:     formalRetry,
			},
			Timeout:   formalIteration * 8 * time.Minute,
			ExtraAttr: []string{},
		}, {
			Name: "quickcheckcuj2_basic_wakeup",
			Val: runCUJParam{
				tests:     []string{"ui.QuickCheckCUJ2.basic_wakeup"},
				iteration: formalIteration,
				retry:     formalRetry,
			},
			Timeout:   formalIteration * 3 * time.Minute,
			ExtraAttr: []string{},
		}, {
			Name: "quickcheckcuj2_basic_unlock",
			Val: runCUJParam{
				tests:     []string{"ui.QuickCheckCUJ2.basic_unlock"},
				iteration: formalIteration,
				retry:     formalRetry,
			},
			Timeout:   formalIteration * 3 * time.Minute,
			ExtraAttr: []string{},
		}, {
			Name: "everydaymultitaskingcuj_basic_ytmusic",
			Val: runCUJParam{
				tests:     []string{"ui.EverydayMultiTaskingCUJ.basic_ytmusic"},
				iteration: formalIteration,
				retry:     formalRetry,
			},
			Timeout:   formalIteration * 10 * time.Minute,
			ExtraAttr: []string{},
		}, {
			Name: "videocuj2_basic_youtube_web",
			Val: runCUJParam{
				tests:     []string{"ui.VideoCUJ2.basic_youtube_web"},
				iteration: formalIteration,
				retry:     formalRetry,
			},
			Timeout:   formalIteration * 10 * time.Minute,
			ExtraAttr: []string{},
		}, {
			Name: "videocuj2_basic_youtube_app",
			Val: runCUJParam{
				tests:     []string{"ui.VideoCUJ2.basic_youtube_app"},
				iteration: formalIteration,
				retry:     formalRetry,
			},
			Timeout:   formalIteration * 10 * time.Minute,
			ExtraAttr: []string{},
		}, {
			Name: "googlemeetcuj_basic_two",
			Val: runCUJParam{
				tests:     []string{"ui.GoogleMeetCUJ.basic_two"},
				iteration: formalIteration,
				retry:     formalRetry,
			},
			Timeout:   formalIteration * 10 * time.Minute,
			ExtraAttr: []string{},
		}, {
			Name: "googlemeetcuj_basic_small",
			Val: runCUJParam{
				tests:     []string{"ui.GoogleMeetCUJ.basic_small"},
				iteration: formalIteration,
				retry:     formalRetry,
			},
			Timeout:   formalIteration * 10 * time.Minute,
			ExtraAttr: []string{},
		}, {
			Name: "googlemeetcuj_basic_large",
			Val: runCUJParam{
				tests:     []string{"ui.GoogleMeetCUJ.basic_large"},
				iteration: formalIteration,
				retry:     formalRetry,
			},
			Timeout:   formalIteration * 10 * time.Minute,
			ExtraAttr: []string{},
		}, {
			Name: "googlemeetcuj_basic_class",
			Val: runCUJParam{
				tests:     []string{"ui.GoogleMeetCUJ.basic_class"},
				iteration: formalIteration,
				retry:     formalRetry,
			},
			Timeout:   formalIteration * 10 * time.Minute,
			ExtraAttr: []string{},
		}, {
			Name: "tabswitchcuj2_plus_noproxy",
			Val: runCUJParam{
				tests:     []string{"ui.TabSwitchCUJ2.plus_noproxy"},
				iteration: formalIteration,
				retry:     formalRetry,
			},
			Timeout:   formalIteration * 20 * time.Minute,
			ExtraAttr: []string{},
		}, {
			Name: "everydaymultitaskingcuj_plus_ytmusic",
			Val: runCUJParam{
				tests:     []string{"ui.EverydayMultiTaskingCUJ.plus_ytmusic"},
				iteration: formalIteration,
				retry:     formalRetry,
			},
			Timeout:   formalIteration * 10 * time.Minute,
			ExtraAttr: []string{},
		}, {
			Name: "googlemeetcuj_plus_large",
			Val: runCUJParam{
				tests:     []string{"ui.GoogleMeetCUJ.plus_large"},
				iteration: formalIteration,
				retry:     formalRetry,
			},
			Timeout:   formalIteration * 15 * time.Minute,
			ExtraAttr: []string{},
		}, {
			Name: "googlemeetcuj_plus_class",
			Val: runCUJParam{
				tests:     []string{"ui.GoogleMeetCUJ.plus_class"},
				iteration: formalIteration,
				retry:     formalRetry,
			},
			Timeout:   formalIteration * 15 * time.Minute,
			ExtraAttr: []string{},
		}, {
			Name: "extendeddisplaycuj_plus_video_youtube_web",
			Val: runCUJParam{
				tests:     []string{"ui.ExtendedDisplayCUJ.plus_video_youtube_web"},
				iteration: formalIteration,
				retry:     formalRetry,
			},
			Timeout:   formalIteration * 10 * time.Minute,
			ExtraAttr: []string{},
		}, {
			Name: "tabswitchcuj2_premium_noproxy",
			Val: runCUJParam{
				tests:     []string{"ui.TabSwitchCUJ2.premium_noproxy"},
				iteration: formalIteration,
				retry:     formalRetry,
			},
			Timeout:   formalIteration * 30 * time.Minute,
			ExtraAttr: []string{},
		}, {
			Name: "videocuj2_premium_youtube_web",
			Val: runCUJParam{
				tests:     []string{"ui.VideoCUJ2.premium_youtube_web"},
				iteration: formalIteration,
				retry:     formalRetry,
			},
			Timeout:   formalIteration * 5 * time.Minute,
			ExtraAttr: []string{},
		}, {
			Name: "videocuj2_premium_youtube_app",
			Val: runCUJParam{
				tests:     []string{"ui.VideoCUJ2.premium_youtube_app"},
				iteration: formalIteration,
				retry:     formalRetry,
			},
			Timeout:   formalIteration * 5 * time.Minute,
			ExtraAttr: []string{},
		}, {
			Name: "googlemeetcuj_premium_large",
			Val: runCUJParam{
				tests:     []string{"ui.GoogleMeetCUJ.premium_large"},
				iteration: formalIteration,
				retry:     formalRetry,
			},
			Timeout:   formalIteration * 15 * time.Minute,
			ExtraAttr: []string{},
		}, {
			Name: "extendeddisplaycuj_premium_meet_large",
			Val: runCUJParam{
				tests:     []string{"ui.ExtendedDisplayCUJ.premium_meet_large"},
				iteration: formalIteration,
				retry:     formalRetry,
			},
			Timeout:   formalIteration * 15 * time.Minute,
			ExtraAttr: []string{},
		}, {
			Name: "basic_quick",
			Val: runCUJParam{
				tests:     basicTests,
				iteration: quickIteration,
				retry:     quickRetry,
			},
			Timeout: quickIteration * time.Duration(len(basicTests)) * 10 * time.Minute,
		}, {
			Name: "plus_quick",
			Val: runCUJParam{
				tests:     plusTests,
				iteration: quickIteration,
				retry:     quickRetry,
			},
			Timeout: quickIteration * time.Duration(len(plusTests)) * 12 * time.Minute,
		}, {
			Name: "premium_quick",
			Val: runCUJParam{
				tests:     premiumTests,
				iteration: quickIteration,
				retry:     quickRetry,
			},
			Timeout: quickIteration * time.Duration(len(premiumTests)) * 15 * time.Minute,
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
