// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package playback provides common code for video.PlayBack* tests.
package playback

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/local/bundles/cros/video/common"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

const (
	// Identifier for measured values with/without HW Acceleration.
	PlaybackWithHWAcceleration    = "playback_with_hw_acceleration"
	PlaybackWithoutHWAcceleration = "playback_without_hw_acceleration"

	// Time to sleep during the test properly.
	StarbilizationDuration = 10
	MesurementDuration     = 30

	// Description for measured values shown in dashboard.
	DroppedFrameDesc        = "video_dropped_frames_"
	DroppedFramePercentDesc = "video_dropped_frames_percent_"
)

// metricsFunc is the type of function to gather metrics during playback.
type metricsFunc func(*testing.State, context.Context, *chrome.Conn) map[string]float64

// RunTest measures dropped frames and dropped frames percent in playing video with/without HW Acceleration.
// The measured values reports dashabord. videoDesc is a video description shown on the dashboard.
func RunTest(ctx context.Context, s *testing.State, video string, videoDesc string) {
	defer common.DisableVideoLogs(common.EnableVideoLogs(s))
	p := &perf.Values{}

	defer p.Save(s.OutDir())
	// DroppedFrames and DroppedFramesPercent
	keyvals := playback(s, ctx, video, getDroppedFrames)
	testing.ContextLogf(ctx, "Measured keyvals (Dropped frames and percent): %v", keyvals)
	savePerfResult(p, assortMetrics(s, keyvals, DroppedFrameDesc), DroppedFrameDesc+videoDesc, "frames")
	savePerfResult(p, assortMetrics(s, keyvals, DroppedFramePercentDesc), DroppedFramePercentDesc+videoDesc, "percent")

	// TODO(crbug.com/890733): Add CPU usage.
}

// assortMetrics extracts metricsDesc's values from keyvals.
// Example:
//  keyvals = {PlaybackWithHWAcceleration   : {DroppedFramesDesc:  0, DroppedFramesDescPercent: 0},
//             PlaybackWithoutHWAcceleration: {DroppedFramesDesc: 10, DroppedFramesDescPercent: 3}}
//  metricsDesc: DroppedFramesDescPercent
//  Returns: {PlaybackWithHWAcceleration: 0, PlaybackWithoutHWAcceleration:3}
func assortMetrics(s *testing.State, keyvals map[string]map[string]float64, metricsDesc string) map[string]float64 {
	resultByMetrics := make(map[string]float64)
	for playback, metrics := range keyvals {
		for k, v := range metrics {
			if k == metricsDesc {
				resultByMetrics[playback] = v
			}
		}
	}
	// There should be software playback performance.
	if len(resultByMetrics) == 0 {
		s.Fatalf("No performance result for %s: %v", metricsDesc, keyvals)
	}
	return resultByMetrics
}

// getDroppedFrames obtains the number of decoded frames and dropped frames by JavaScript,
// and returns the number of dropped frames and dropped frames rate.
func getDroppedFrames(s *testing.State, ctx context.Context, conn *chrome.Conn) map[string]float64 {
	time.Sleep(time.Duration(MesurementDuration) * time.Second)
	decodedFrameCount, droppedFrameCount := 0, 0
	if err := conn.Eval(ctx, "document.getElementsByTagName('video')[0].webkitDecodedFrameCount", &decodedFrameCount); err != nil {
		s.Error("Failed to get # of decoded frames: ", err)
		return nil
	}
	if err := conn.Eval(ctx, "document.getElementsByTagName('video')[0].webkitDroppedFrameCount", &droppedFrameCount); err != nil {
		s.Error("Failed to get # of dropped frames: ", err)
		return nil
	}
	droppedFramePercent := 0.0
	if decodedFrameCount != 0 {
		droppedFramePercent = 100.0 * float64(droppedFrameCount) / float64(decodedFrameCount)
	} else {
		testing.ContextLogf(ctx, "No frame is decoded. Set drop percent to 100.")
		droppedFramePercent = 100.0
	}
	return map[string]float64{
		DroppedFrameDesc:        float64(droppedFrameCount),
		DroppedFramePercentDesc: droppedFramePercent,
	}
}

// startPlayback plays video one time and measures performance values by executing gatherResultFunc().
// The values are added keyvals.
// If disableHWAcc is true, Chrome must play video with SW decoder. If false, Chrome play video with HW decoder if it is available.
func startPlayback(s *testing.State, ctx context.Context, video string, gatherResultFunc metricsFunc, keyvals *map[string]map[string]float64, disableHWAcc bool) {
	chromeArgs := []string{common.ChromeVmoduleFlag()}
	if disableHWAcc {
		chromeArgs = append(chromeArgs, "--disable-accelerated-video-decode")
	}

	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromeArgs))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
		return
	}
	defer cr.Close(ctx)
	// TODO(hiroh): enforce the idle checks after login.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	hd, err := common.NewHistogramDiffer(s, ctx, cr, common.MediaGVDInitStatus)
	if err != nil {
		s.Fatal("Failed to create HistogramDiffer: ", err)
		return
	}

	conn, err := cr.NewConn(ctx, server.URL+"/"+video)
	if err != nil {
		s.Fatal("Failed to open video page: ", err)
		return
	}

	defer conn.Close()
	result := gatherResultFunc(s, ctx, conn)
	if result == nil {
		s.Fatal("Failed to gather results.")
		return
	}

	histogramDiff := common.PollHistogramGrow(s, ctx, hd, 10, 1)
	if len(histogramDiff) > 1 {
		s.Fatal("Unexpected Histogram Difference: ", histogramDiff)
		return
	}
	if _, found := histogramDiff[common.MediaGVDBucket]; found {
		if disableHWAcc {
			s.Fatal("Video Decode Acceleration should not be working.")
			return
		}
		(*keyvals)[PlaybackWithHWAcceleration] = result
	} else if disableHWAcc {
		// Software playback performance is ignored, unless HW Acceleration is disabled.
		(*keyvals)[PlaybackWithoutHWAcceleration] = result
	}
}

// playback plays video with SW decoder and also with HW decoder if available.
func playback(s *testing.State, ctx context.Context, video string, gatherResultFunc metricsFunc) map[string]map[string]float64 {
	keyvals := make(map[string]map[string]float64)
	// Try Software playback.
	startPlayback(s, ctx, video, gatherResultFunc, &keyvals, true)
	// Try witout disabling HW Acceleration
	startPlayback(s, ctx, video, gatherResultFunc, &keyvals, false)
	return keyvals
}

// savePerfResult adds keyvals with descrition and unit to perf.Values.
func savePerfResult(p *perf.Values, keyvals map[string]float64, desc string, unit string) {
	// TODO(hiroh): Remove tastSuffix after removing video_PlaybackPerf in autotest.
	const tastSuffix = "tast_"
	for playback, value := range keyvals {
		perfName := tastSuffix
		if playback == PlaybackWithHWAcceleration {
			perfName += "hw_" + desc
		} else {
			perfName += "sw_" + desc
		}
		metric := perf.Metric{Name: perfName, Unit: unit, Direction: perf.SmallerIsBetter}
		p.Set(metric, value)
	}
}
