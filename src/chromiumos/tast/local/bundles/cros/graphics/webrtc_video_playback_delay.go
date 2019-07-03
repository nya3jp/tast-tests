// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

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
		Attr:         []string{"group:crosbolt"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"webrtc_video_display_perf_test.html"},
	})
}

func WebRTCVideoPlaybackDelay(ctx context.Context, s *testing.State) {
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	testURL := server.URL + "/" + "webrtc_video_display_perf_test.html"

	chromeArgs := []string{
		"--autoplay-policy=no-user-gesture-required",
		"--disable-rtc-smoothness-algorithm",
		"--use-fake-device-for-media-stream=fps=60",
		"--use-fake-ui-for-media-stream",
	}
	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromeArgs...))
	if err != nil {
		s.Fatal("Failed to create Chrome: ", err)
	}
	defer cr.Close(ctx)

	conn, err := cr.NewConn(ctx, testURL)
	if err != nil {
		s.Fatalf("Failed to open %s: %v", testURL, err)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	// We could consider removing the CPU frequency scaling and thermal throttling
	// to get more consistent results, but then we wouldn't be measuring on the
	// same conditions as a user might encounter.

	histogramName := "Media.VideoFrameSubmitter"
	initHistogram, err := metrics.GetHistogram(ctx, cr, histogramName)
	if err != nil {
		s.Fatal("Failed to get initial histogram: ", err)
	}

	if err = conn.Eval(ctx, "start(1920, 1080)", nil); err != nil {
		s.Fatal("Start failed: ", err)
	}
	if err = conn.WaitForExpr(ctx, "isGetUserMediaFinished == true"); err != nil {
		s.Fatal("Wait for getUserMedia() failed: ", err)
	}

	if err = conn.Eval(ctx, "call()", nil); err != nil {
		s.Fatal("Call failed: ", err)
	}

	if err := testing.Sleep(ctx, 20 * time.Second); err != nil {
		s.Fatal("Error while waiting for playback delay perf collection: ", err)
	}

	laterHistogram, err := metrics.GetHistogram(ctx, cr, histogramName)
	if err != nil {
		s.Fatal("Failed to get later histogram: ", err)
	}

	histogramDiff, err := laterHistogram.Diff(initHistogram)
	if err != nil {
		s.Fatal("Failed diffing histograms: ", err)
	}
	numBuckets := len(histogramDiff.Buckets)
	if numBuckets == 0 {
		s.Fatal("Empty histogram diff")
	}

	var averageSum float64 = 0
	var numSamples int64 = 0
	for _, bucket := range histogramDiff.Buckets {
		averageSum += float64((bucket.Max + bucket.Min) * bucket.Count) / 2.0
		numSamples += bucket.Count
	}

	testing.ContextLogf(ctx, "%s histogram: %v; average: %f",
			histogramName, laterHistogram.String(), averageSum / float64(numSamples))

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
