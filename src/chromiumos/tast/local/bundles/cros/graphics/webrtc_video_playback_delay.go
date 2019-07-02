// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/local/bundles/cros/video/lib/cpu"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WebRTCVideoPlaybackDelay,
		Desc: "Runs a webrtc playback-only connection to get performance numbers",
		Contacts: []string{"mcasas@chromium.org", "chromeos-gfx@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"webrtc_video_display_perf_test.html"},
		Timeout:      5 * time.Minute,
	})
}

func WebRTCVideoPlaybackDelay(ctx context.Context, s *testing.State) {
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	testURL := server.URL + "/" + "webrtc_video_display_perf_test.html"

	s.Log("Logging out")
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Chrome logout failed: ", err)
	}

	s.Log("Setting up for CPU benchmarking")
	shortCtx, cleanupBenchmark, err := cpu.SetUpBenchmark(ctx)
	if err != nil {
		s.Fatal("Failed to set up CPU benchmark mode: ", err)
	}
	defer cleanupBenchmark()

	chromeArgs := []string{
		"--autoplay-policy=no-user-gesture-required",
		"--disable-rtc-smoothness-algorithm",
		"--use-fake-device-for-media-stream=fps=60",
		"--use-fake-ui-for-media-stream",
	}
	cr, err := chrome.New(shortCtx, chrome.ExtraArgs(chromeArgs...))
	if err != nil {
		s.Fatal("Failed to create Chrome: ", err)
	}
	defer cr.Close(shortCtx)

	conn, err := cr.NewConn(shortCtx, testURL)
	if err != nil {
		s.Fatalf("Failed to open %s: %v", testURL, err)
	}
	defer conn.Close()
	defer conn.CloseTarget(shortCtx)

	histogramName := "Media.VideoFrameSubmitter"
	initHistogram, err := metrics.GetHistogram(shortCtx, cr, histogramName)
	if err != nil {
		s.Fatalf("failed to get initial histogram %v", err)
	}

	if err = conn.Eval(shortCtx, "start(1920, 1080)", nil); err != nil {
		s.Fatalf("start failed %v", err)
	}
	s.Log("start worked")
	if err = conn.WaitForExpr(shortCtx, "isGetUserMediaFinished == true"); err != nil {
		s.Fatalf("wait failed %v", err)
	}

	if err = conn.Eval(shortCtx, "call()", nil); err != nil {
		s.Fatalf("call failed %v", err)
	}
	s.Log("call worked")

	s.Log("going to sleep")
	time.Sleep(20 * time.Second)
	s.Log("waking up")


	laterHistogram, err := metrics.GetHistogram(shortCtx, cr, histogramName)
	if err != nil {
		s.Fatalf("failed to get later histogram %v", err)
	}
	testing.ContextLogf(shortCtx, "Later %s histogram: %v", histogramName, laterHistogram.String())

	histogramDiff, err := laterHistogram.Diff(initHistogram)
	if err != nil {
		s.Fatalf("failed diffing histograms %v", err)
	}
	numBuckets := len(histogramDiff.Buckets)
	if numBuckets == 0 {
		s.Fatalf("empty histogram diff")
	}

	var averageSum float64 = 0
	var numSamples int64 = 0
	for _, bucket := range histogramDiff.Buckets {
		averageSum += float64((bucket.Max + bucket.Min) * bucket.Count) / 2.0
		numSamples += bucket.Count
	}

	s.Log("finiiiished %v %f", histogramDiff.Buckets, averageSum / float64(numSamples))

	WebRTCVideoPlaybackDelayMetric := perf.Metric{
		Name:      "tast_graphics_webrtc_video_playback_delay",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}
	perfValues := perf.NewValues()

	// Append the central value of the histogram buckets as many times as bucket
	// entries.
	for _, bucket := range histogramDiff.Buckets {
		averageValue := float64(bucket.Max + bucket.Min) / 2.0
		for i := 0; i < int(bucket.Count); i++ {
			perfValues.Append(WebRTCVideoPlaybackDelayMetric, averageValue)
		}
	}

	if err = perfValues.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}
