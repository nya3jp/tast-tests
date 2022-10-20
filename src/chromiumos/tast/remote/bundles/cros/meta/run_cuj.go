// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
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
	// Default iteration and retry numbers for full run and quick run. It can be overriden by the
	// runtime variables.
	fullIteration  = 10
	fullRetry      = 4 // Total 5 times of execution if one test fails.
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
		Desc:     "Run CUJ tests with specified iterations and retries",
		Contacts: []string{"xliu@cienet.com", "cienet-development@googlegroups.com"},
		Vars: []string{
			"variablesfile", // Mandatory. The varsfile that will be used to call the CUJ test.
			"iteration",     // Optional. If given, it overrides the default value.
			"retry",         // Optional. If given, it overrides the default value.
		},
		Params: []testing.Param{{
			Name: "tabswitchcuj2_basic_noproxy",
			Val: runCUJParam{
				tests:     []string{"ui.TabSwitchCUJ2.basic_noproxy"},
				iteration: fullIteration,
				retry:     fullRetry,
			},
			Timeout: fullIteration * 8 * time.Minute,
		}, {
			Name: "quickcheckcuj2_basic_wakeup",
			Val: runCUJParam{
				tests:     []string{"ui.QuickCheckCUJ2.basic_wakeup"},
				iteration: fullIteration,
				retry:     fullRetry,
			},
			Timeout: fullIteration * 3 * time.Minute,
		}, {
			Name: "quickcheckcuj2_basic_unlock",
			Val: runCUJParam{
				tests:     []string{"ui.QuickCheckCUJ2.basic_unlock"},
				iteration: fullIteration,
				retry:     fullRetry,
			},
			Timeout: fullIteration * 3 * time.Minute,
		}, {
			Name: "everydaymultitaskingcuj_basic_ytmusic",
			Val: runCUJParam{
				tests:     []string{"ui.EverydayMultiTaskingCUJ.basic_ytmusic"},
				iteration: fullIteration,
				retry:     fullRetry,
			},
			Timeout: fullIteration * 10 * time.Minute,
		}, {
			Name: "videocuj2_basic_youtube_web",
			Val: runCUJParam{
				tests:     []string{"ui.VideoCUJ2.basic_youtube_web"},
				iteration: fullIteration,
				retry:     fullRetry,
			},
			Timeout: fullIteration * 10 * time.Minute,
		}, {
			Name: "videocuj2_basic_youtube_app",
			Val: runCUJParam{
				tests:     []string{"ui.VideoCUJ2.basic_youtube_app"},
				iteration: fullIteration,
				retry:     fullRetry,
			},
			Timeout: fullIteration * 10 * time.Minute,
		}, {
			Name: "googlemeetcuj_basic_two",
			Val: runCUJParam{
				tests:     []string{"ui.GoogleMeetCUJ.basic_two"},
				iteration: fullIteration,
				retry:     fullRetry,
			},
			Timeout: fullIteration * 10 * time.Minute,
		}, {
			Name: "googlemeetcuj_basic_small",
			Val: runCUJParam{
				tests:     []string{"ui.GoogleMeetCUJ.basic_small"},
				iteration: fullIteration,
				retry:     fullRetry,
			},
			Timeout: fullIteration * 10 * time.Minute,
		}, {
			Name: "googlemeetcuj_basic_large",
			Val: runCUJParam{
				tests:     []string{"ui.GoogleMeetCUJ.basic_large"},
				iteration: fullIteration,
				retry:     fullRetry,
			},
			Timeout: fullIteration * 10 * time.Minute,
		}, {
			Name: "googlemeetcuj_basic_class",
			Val: runCUJParam{
				tests:     []string{"ui.GoogleMeetCUJ.basic_class"},
				iteration: fullIteration,
				retry:     fullRetry,
			},
			Timeout: fullIteration * 10 * time.Minute,
		}, {
			Name: "tabswitchcuj2_plus_noproxy",
			Val: runCUJParam{
				tests:     []string{"ui.TabSwitchCUJ2.plus_noproxy"},
				iteration: fullIteration,
				retry:     fullRetry,
			},
			Timeout: fullIteration * 20 * time.Minute,
		}, {
			Name: "everydaymultitaskingcuj_plus_ytmusic",
			Val: runCUJParam{
				tests:     []string{"ui.EverydayMultiTaskingCUJ.plus_ytmusic"},
				iteration: fullIteration,
				retry:     fullRetry,
			},
			Timeout: fullIteration * 10 * time.Minute,
		}, {
			Name: "googlemeetcuj_plus_large",
			Val: runCUJParam{
				tests:     []string{"ui.GoogleMeetCUJ.plus_large"},
				iteration: fullIteration,
				retry:     fullRetry,
			},
			Timeout: fullIteration * 15 * time.Minute,
		}, {
			Name: "googlemeetcuj_plus_class",
			Val: runCUJParam{
				tests:     []string{"ui.GoogleMeetCUJ.plus_class"},
				iteration: fullIteration,
				retry:     fullRetry,
			},
			Timeout: fullIteration * 15 * time.Minute,
		}, {
			Name: "extendeddisplaycuj_plus_video_youtube_web",
			Val: runCUJParam{
				tests:     []string{"ui.ExtendedDisplayCUJ.plus_video_youtube_web"},
				iteration: fullIteration,
				retry:     fullRetry,
			},
			Timeout: fullIteration * 10 * time.Minute,
		}, {
			Name: "tabswitchcuj2_premium_noproxy",
			Val: runCUJParam{
				tests:     []string{"ui.TabSwitchCUJ2.premium_noproxy"},
				iteration: fullIteration,
				retry:     fullRetry,
			},
			Timeout: fullIteration * 30 * time.Minute,
		}, {
			Name: "videocuj2_premium_youtube_web",
			Val: runCUJParam{
				tests:     []string{"ui.VideoCUJ2.premium_youtube_web"},
				iteration: fullIteration,
				retry:     fullRetry,
			},
			Timeout: fullIteration * 5 * time.Minute,
		}, {
			Name: "videocuj2_premium_youtube_app",
			Val: runCUJParam{
				tests:     []string{"ui.VideoCUJ2.premium_youtube_app"},
				iteration: fullIteration,
				retry:     fullRetry,
			},
			Timeout: fullIteration * 5 * time.Minute,
		}, {
			Name: "googlemeetcuj_premium_large",
			Val: runCUJParam{
				tests:     []string{"ui.GoogleMeetCUJ.premium_large"},
				iteration: fullIteration,
				retry:     fullRetry,
			},
			Timeout: fullIteration * 15 * time.Minute,
		}, {
			Name: "extendeddisplaycuj_premium_meet_large",
			Val: runCUJParam{
				tests:     []string{"ui.ExtendedDisplayCUJ.premium_meet_large"},
				iteration: fullIteration,
				retry:     fullRetry,
			},
			Timeout: fullIteration * 15 * time.Minute,
		}},
	})
}

func RunCUJ(ctx context.Context, s *testing.State) {
	param := s.Param().(runCUJParam)
	iteration := param.iteration
	retry := param.retry
	varsfile := ""

	if strVar, ok := s.Var("variablesfile"); ok {
		varsfile = strVar
	}

	// Override default iteration and retry if runtime variables are provided.
	if strVar, ok := s.Var("iteration"); ok {
		if intVar, err := strconv.Atoi(strVar); err == nil {
			iteration = intVar
		} else {
			s.Fatalf("Failed to parse the runtime variable \"iteration\": want an integer, got %q", strVar)
		}
	}
	if strVar, ok := s.Var("retry"); ok {
		if intVar, err := strconv.Atoi(strVar); err == nil {
			retry = intVar
		} else {
			s.Fatalf("Failed to parse the runtime variable \"retry\": want an integer, got %q", strVar)
		}
	}

	s.Logf("Run %d iterations with %d retries for failed tests. Tests to execute: %v",
		iteration, retry, param.tests)
	for i := 1; i <= iteration; i++ {
		s.Logf("Running iteration %d of %d", i, iteration)
		resultsDir := filepath.Join(s.OutDir(), fmt.Sprintf("subtest_results_%d", i))

		flags := []string{
			fmt.Sprintf("-retries=%d", retry),
		}
		if varsfile != "" {
			flags = append(flags, fmt.Sprintf("-varsfile=%s", varsfile))
		}

		skippedTests := tastrun.RunAndEvaluate(ctx, s, flags, param.tests, resultsDir, tastrun.SkipPolicyAllowSkipping)
		if len(skippedTests) == len(param.tests) {
			s.Errorf("All %d test(s) in iteration %d have been skipped", len(param.tests), i)
		}
	}
}
