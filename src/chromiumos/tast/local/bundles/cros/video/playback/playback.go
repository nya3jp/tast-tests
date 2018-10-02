// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package playback provides common code for video.Playback* tests.
package playback

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/video/lib/constants"
	"chromiumos/tast/local/bundles/cros/video/lib/cpu"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

type playbackType string

const (
	// Identifier for a map of measured values with/without HW Acceleration
	// storing during a test.
	playbackWithHWAcceleration    playbackType = "playback_with_hw_acceleration"
	playbackWithoutHWAcceleration playbackType = "playback_without_hw_acceleration"

	// Time to sleep during the test properly.
	measurementDuration = 30 * time.Second

	// Timeout to get idle cpu.
	waitIdleCpuTimeout = 30 * time.Second

	// The percentage of cpu usage when it is idle.
	idleCpuUsagePercent = 1.0

	// Description for measured values shown in dashboard.
	droppedFrameDesc        = "video_dropped_frames_"
	droppedFramePercentDesc = "video_dropped_frames_percent_"

	videoElement = "document.getElementsByTagName('video')[0]"
)

// metricsFunc is the type of function to gather metrics during playback.
type metricsFunc = func(context.Context, *chrome.Conn) (map[string]float64, error)

type playbackPerfMetrics struct {
	desc   string
	unit   string
	values map[playbackType]float64
}

// RunTest measures dropped frames and dropped frames percent in playing a video with/without HW Acceleration.
// The measured values are reported to a dashboard. videoDesc is a video description shown on the dashboard.
func RunTest(ctx context.Context, s *testing.State, videoName string, videoDesc string) {
	vl, err := logging.NewVideoLogger(ctx)
	if err != nil {
		s.Fatal("Failed to set values for verbose logging.")
	}
	defer vl.Close(ctx)

	// TODO(crbug.com/890733): Check if video is downloaded correctly.
	// The md5 value of the video must match md5 string included in videoName.

	p := &perf.Values{}

	// Measure the number of dropped frames and the rate of dropped frames.
	ms := []playbackPerfMetrics{
		{desc: droppedFrameDesc, unit: "frames", values: map[playbackType]float64{}},
		{desc: droppedFramePercentDesc, unit: "percent", values: map[playbackType]float64{}},
	}
	if err := measurePlaybackPerf(ctx, s.DataFileSystem(), videoName, getDroppedFrames, ms); err != nil {
		s.Fatal("Failed to collect performance values: ", err)
	}
	testing.ContextLogf(ctx, "Measured metrics (dropped frames and percent): %v", ms)

	for _, m := range ms {
		if len(m.values) == 0 {
			s.Fatalf("No performance result for %s: %v", m.desc, ms)
		}
		savePerfResult(p, m.values, m.desc+videoDesc, m.unit)
	}

	// TODO(crbug.com/890733): Add CPU usage.
	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}

// getDroppedFrames obtains the number of decoded frames and dropped frames by JavaScript,
// and returns the number of dropped frames and the rate of dropped frames.
func getDroppedFrames(ctx context.Context, conn *chrome.Conn) (map[string]float64, error) {

	var decodedFrameCount, droppedFrameCount int64
	if err := conn.Eval(ctx, videoElement+".webkitDecodedFrameCount", &decodedFrameCount); err != nil {
		return nil, errors.Wrap(err, "failed to get # of decoded frames.")
	}
	if err := conn.Eval(ctx, videoElement+".webkitDroppedFrameCount", &droppedFrameCount); err != nil {
		return nil, errors.Wrap(err, "failed to get # of dropped frames.")
	}

	var droppedFramePercent float64
	if decodedFrameCount != 0 {
		droppedFramePercent = 100.0 * float64(droppedFrameCount) / float64(decodedFrameCount)
	} else {
		testing.ContextLogf(ctx, "No frame is decoded and set drop percent to 100.")
		droppedFramePercent = 100.0
	}
	return map[string]float64{
		droppedFrameDesc:        float64(droppedFrameCount),
		droppedFramePercentDesc: droppedFramePercent,
	}, nil
}

// playback plays video one time and measures performance values by executing gatherPerfFunc().
// The measured values are recorded in ms.
// If disableHWAcc is true, Chrome must play video with SW decoder. If false, Chrome play video with HW decoder if it is available.
func playback(ctx context.Context, fileSystem http.FileSystem, videoName string, gatherPerfFunc metricsFunc, ms []playbackPerfMetrics, disableHWAcc bool) error {
	chromeArgs := []string{logging.ChromeVmoduleFlag()}
	if disableHWAcc {
		chromeArgs = append(chromeArgs, "--disable-accelerated-video-decode")
	}

	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromeArgs))
	if err != nil {
		return errors.Wrap(err, "failed to connect to Chrome.")
	}
	defer cr.Close(ctx)

	// Wait until cpu is idle state.
	cpu.WaitForIdle(ctx, waitIdleCpuTimeout, idleCpuUsagePercent)
	server := httptest.NewServer(http.FileServer(fileSystem))
	defer server.Close()

	histogramStart, err := metrics.GetHistogram(ctx, cr, constants.MediaGVDInitStatus)
	if err != nil {
		return errors.Wrap(err, "failed to get histogramStart.")
	}
	testing.ContextLogf(ctx, "Initial histograms/%s: %v", constants.MediaGVDInitStatus, histogramStart.Buckets)

	conn, err := cr.NewConn(ctx, server.URL+"/"+videoName)
	if err != nil {
		return errors.Wrap(err, "failed to open video page.")
	}
	defer conn.Close()

	// Play a video repeatedly during measurement.
	err = conn.Exec(ctx, videoElement+".loop=true")
	if err != nil {
		return errors.Wrap(err, "failed to settle video looping.")
	}

	time.Sleep(measurementDuration)
	mtr, err := gatherPerfFunc(ctx, conn)
	if err != nil {
		return errors.Wrap(err, "failed to gather performance values.")
	}

	// Stop video.
	err = conn.Exec(ctx, videoElement+".pause()")
	if err != nil {
		return errors.Wrap(err, "failed to stop video.")
	}

	return recordMetrics(ctx, mtr, ms, cr, histogramStart, disableHWAcc)
}

// recordMetrics records the measured performance values, mtr, in ms.
func recordMetrics(ctx context.Context, mtr map[string]float64, ms []playbackPerfMetrics, cr *chrome.Chrome, histogramStart *metrics.Histogram, disableHWAcc bool) error {
	// There are three valid cases.
	// 1. No histogram is updated. This is the case HW Acceleration is disabled due to Chrome flag, --disable-accelerated-video-decode.
	// 2. Histogram is updated with 15. This is the case Chrome tries to initailize VDA but it fails because the code is not supported on DUT.
	// 3. Histogram is updated with 0. This is the case Chrome sucessfully initializes VDA.

	// err is not nil here if HW Acceleration is disable and then Chrome doesn't try VDA initialization at all.
	// For the case 1, we pass a short time context to WaitForHistogramUpdate to avoid the whole test context (ctx) from reaching deadline.
	histogramDiff, err := metrics.WaitForHistogramUpdate(ctx, cr, constants.MediaGVDInitStatus, histogramStart, 10*time.Second)
	hwAccelerationUsed := false
	if err == nil {
		testing.ContextLogf(ctx, "Diff histograms/%s: %v", constants.MediaGVDInitStatus, histogramDiff.Buckets)
		if len(histogramDiff.Buckets) > 1 {
			return errors.Wrapf(err, "unexpected histogram difference: %v", histogramDiff)
		}
		testing.ContextLog(ctx, "HistogramDiff Buckets: ", histogramDiff.Buckets)

		// If HW acceleration is used, the sole bucket is {0, 1, X}.
		diff := histogramDiff.Buckets[0]
		if diff.Min == constants.MediaGVDBucket && diff.Max == constants.MediaGVDBucket+1 {
			hwAccelerationUsed = true
		}
	}

	if hwAccelerationUsed && disableHWAcc {
		return errors.New("Hardware acceleration used despite being disabled.")
	}
	if !hwAccelerationUsed && !disableHWAcc {
		// Software playback performance is not recorded, unless HW Acceleration is disabled.
		return nil
	}

	pType := playbackWithoutHWAcceleration
	if hwAccelerationUsed {
		pType = playbackWithHWAcceleration
	}
	for desc, value := range mtr {
		for _, m := range ms {
			if m.desc == desc {
				m.values[pType] = value
				break
			}
		}
	}
	return nil
}

// measurePlaybackPerf collects video playback performance by gatherPerfFunc, playing a video with SW decoder and
// also with HW decoder if available.
func measurePlaybackPerf(ctx context.Context, fileSystem http.FileSystem, videoName string, gatherPerfFunc metricsFunc, ms []playbackPerfMetrics) error {
	// Try Software playback.
	// TODO(hiroh): enable this after a histogram issue is resolved.
	// if err := playback(ctx, fileSystem, video, gatherPerfFunc, ms, true); err != nil {
	// 	return err
	// }
	// Try without disabling HW Acceleration.
	if err := playback(ctx, fileSystem, videoName, gatherPerfFunc, ms, false); err != nil {
		return err
	}
	return nil
}

// savePerfResult adds perf results with desc and unit to perf.Values.
func savePerfResult(p *perf.Values, results map[playbackType]float64, desc, unit string) {
	// TODO(hiroh): Remove tastSuffix after removing video_PlaybackPerf in autotest.
	const tastPrefix = "tast_"
	for pType, value := range results {
		perfName := tastPrefix
		if pType == playbackWithHWAcceleration {
			perfName += "hw_" + desc
		} else {
			perfName += "sw_" + desc
		}
		metric := perf.Metric{Name: perfName, Unit: unit, Direction: perf.SmallerIsBetter}
		p.Set(metric, value)
	}
}
