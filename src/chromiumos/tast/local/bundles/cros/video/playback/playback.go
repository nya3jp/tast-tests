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
	"chromiumos/tast/local/faillog"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

const (
	PlaybackWithHWAcceleration    = "playback_with_hw_acceleration"
	PlaybackWithoutHWAcceleration = "playback_without_hw_acceleration"

	MesurementDuration = 30

	DroppedFrameDesc        = "video_dropped_frames_"
	DroppedFramePercentDesc = "video_dropped_frames_percent_"
)

type metricsFunc func(context.Context, *chrome.Conn) map[string]float64

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
		s.Fatal("No performance result for %s: %v", metricsDesc, keyvals)
	}
	return resultByMetrics
}

func RunTest(s *testing.State, video string, videoDesc string) {
	defer common.DisableVideoLogs(common.EnableVideoLogs(s.Context()))
	defer faillog.SaveIfError(s)
	p := &perf.Values{}
	defer p.Save(s.OutDir())
	// DroppedFrames and DroppedFramesPercent
	keyvals := playback(s, video, getDroppedFrames)
	testing.ContextLogf(s.Context(), "Measured keyvals: %v", keyvals)
	uploadPerfResult(p, assortMetrics(s, keyvals, DroppedFrameDesc), DroppedFrameDesc+videoDesc, "frames")
	uploadPerfResult(p, assortMetrics(s, keyvals, DroppedFramePercentDesc), DroppedFramePercentDesc+videoDesc, "percent")
}

func getDroppedFrames(ctx context.Context, conn *chrome.Conn) map[string]float64 {
	time.Sleep(time.Duration(MesurementDuration) * time.Second)
	decodedFrameCount, droppedFrameCount := 0, 0
	if err := conn.Eval(ctx, "document.getElementsByTagName('video')[0].webkitDecodedFrameCount", &decodedFrameCount); err != nil {
		testing.ContextLogf(ctx, "Failed to get # of decoded frames, %v", err)
		return nil
	}
	if err := conn.Eval(ctx, "document.getElementsByTagName('video')[0].webkitDroppedFrameCount", &droppedFrameCount); err != nil {
		testing.ContextLogf(ctx, "Failed to get # of decoded frames, %v", err)
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

func startPlayback(s *testing.State, video string, gatherResultFunc metricsFunc, keyvals *map[string]map[string]float64, disableHWAcc bool) {
	ctx := s.Context()
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

	hd, err := common.NewHistogramDiffer(ctx, cr, common.MediaGVDInitStatus)
	if err != nil {
		s.Fatal("Failed to create HistogramDiffer.")
		return
	}
	conn, err := cr.NewConn(ctx, server.URL+"/"+video)
	defer conn.Close()
	result := gatherResultFunc(ctx, conn)

	histogramDiff := common.PollHistogramGrow(ctx, hd, 10, 1)
	if len(histogramDiff) > 1 {
		s.Fatal("Unexpected Histogram Difference: %v", histogramDiff)
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

func playback(s *testing.State, video string, gatherResultFunc metricsFunc) map[string]map[string]float64 {
	// TODO(hiroh): enforce the idle checks after login.
	keyvals := make(map[string]map[string]float64)
	// Try without disabling HW Acceleration
	startPlayback(s, video, gatherResultFunc, &keyvals, true)
	// Try witout disabling HW Acceleration
	startPlayback(s, video, gatherResultFunc, &keyvals, false)
	return keyvals

}

func uploadPerfResult(p *perf.Values, keyvals map[string]float64, desc string, unit string) {
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
