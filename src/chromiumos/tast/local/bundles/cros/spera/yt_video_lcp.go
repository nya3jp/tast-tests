// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package spera

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         YTVideoLCP,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measure LCP of YT web with or without user interactions",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", "arc"},
		Fixture:      "loggedInAndKeepState",
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name: "interaction_3s",
				Val:  3,
			}, {
				Name: "interaction_10s",
				Val:  10,
			}, {
				Name: "interaction_20s",
				Val:  20,
			}, {
				Name: "no_interaction",
				Val:  30,
			},
		},
	})
}

// YTVideoLCP performs the video loading and collect LCP.
func YTVideoLCP(ctx context.Context, s *testing.State) {
	userIteractSecond := s.Param().(int)
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kb.Close()

	testing.ContextLog(ctx, "Start recording actions")
	options := cujrecorder.NewPerformanceCUJOptions()
	recorder, err := cujrecorder.NewRecorder(ctx, cr, nil, nil, options)
	if err != nil {
		s.Fatal("Failed to create the recorder: ", err)
	}
	defer recorder.Close(ctx)
	if err := cuj.AddPerformanceCUJMetrics(tconn, nil, recorder); err != nil {
		s.Fatal("Failed to add metrics to recorder: ", err)
	}

	if err := recorder.Run(ctx, func(ctx context.Context) error {
		duration := 30

		testing.ContextLog(ctx, "Navigate to YT Web")
		_, err = cr.NewConn(ctx, cuj.YoutubeDeveloperKeynoteVideoURL)
		if err != nil {
			s.Fatal("Failed to open new tab: ", err)
		}

		s.Logf("Sleep for %d seconds while the video is playing", userIteractSecond)
		testing.Sleep(ctx, time.Duration(time.Duration(userIteractSecond))*time.Second)
		if userIteractSecond < duration {
			s.Log("User interaction: pause and play with keyboard")
			if err := uiauto.NamedCombine("pause and play with k",
				kb.TypeAction("k"),
				uiauto.Sleep(1*time.Second),
				kb.TypeAction("k"),
			)(ctx); err != nil {
				s.Fatal("Failed to pause and play video: ", err)
			}
		}
		s.Logf("Sleep for %d seconds while the video is playing", duration-userIteractSecond)
		testing.Sleep(ctx, time.Duration(duration-userIteractSecond)*time.Second)

		browser.CloseAllTabs(ctx, tconn)
		return nil
	}); err != nil {
		s.Fatal("Failed to run the recorder: ", err)
	}

	pv := perf.NewValues()

	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to record the performance data: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save the performance data: ", err)
	}
	if err := recorder.SaveHistograms(s.OutDir()); err != nil {
		s.Fatal("Failed to save histogram raw data: ", err)
	}
}
