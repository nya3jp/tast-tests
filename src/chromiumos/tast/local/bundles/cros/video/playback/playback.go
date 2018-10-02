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

type perfMetrics struct {
	desc   string
	unit   string
	dir    perf.Direction
	values map[playbackType]float64
}

// RunTest measures dropped frames and dropped frames percent in playing a video with/without HW Acceleration.
// The measured values are reported to a dashboard. videoDesc is a video description shown on the dashboard.
func RunTest(ctx context.Context, s *testing.State, videoName string, videoDesc string) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	// TODO(crbug.com/890733): Check if video is downloaded correctly.
	// The md5 value of the video must match md5 string included in videoName.

	// Measure the number of dropped frames and the rate of dropped frames.
	ms := []perfMetrics{
		{desc: droppedFrameDesc, unit: "frames", dir: perf.SmallerIsBetter, values: map[playbackType]float64{}},
		{desc: droppedFramePercentDesc, unit: "percent", dir: perf.SmallerIsBetter, values: map[playbackType]float64{}},
	}
	if err := measurePlaybackPerf(ctx, s.DataFileSystem(), videoName, getDroppedFrames, ms); err != nil {
		s.Fatal("Failed to collect performance values: ", err)
	}
	testing.ContextLogf(ctx, "Measured metrics (dropped frames and percent): %v", ms)

	// TODO(crbug.com/890733): Add CPU usage.
	if err := savePerfResults(ms, videoDesc, s.OutDir()); err != nil {
		s.Fatal("Failed to save perf data: ", ms)
	}
}

// measurePlaybackPerf collects video playback performance by gatherPerfFunc, playing a video with SW decoder and
// also with HW decoder if available.
func measurePlaybackPerf(ctx context.Context, fileSystem http.FileSystem, videoName string, gatherPerfFunc metricsFunc, ms []perfMetrics) error {
	// Try Software playback.
	if err := playback(ctx, fileSystem, videoName, gatherPerfFunc, ms, true); err != nil {
		return err
	}

	// Try without disabling HW Acceleration.
	if err := playback(ctx, fileSystem, videoName, gatherPerfFunc, ms, false); err != nil {
		return err
	}
	return nil
}

// playback plays video one time and measures performance values by executing gatherPerfFunc().
// The measured values are recorded in ms.
// If disableHWAcc is true, Chrome must play video with SW decoder. If false, Chrome play video with HW decoder if it is available.
func playback(ctx context.Context, fileSystem http.FileSystem, videoName string, gatherPerfFunc metricsFunc, ms []perfMetrics, disableHWAcc bool) error {
	chromeArgs := []string{logging.ChromeVmoduleFlag()}
	if disableHWAcc {
		chromeArgs = append(chromeArgs, "--disable-accelerated-video-decode")
	}

	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromeArgs))
	if err != nil {
		return errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	// Wait until cpu is idle state.
	if err := cpu.WaitForIdle(ctx, waitIdleCpuTimeout, idleCpuUsagePercent); err != nil {
		return errors.Wrap(err, "failed to wait idle cpu")
	}

	server := httptest.NewServer(http.FileServer(fileSystem))
	defer server.Close()

	beginHistogram, err := metrics.GetHistogram(ctx, cr, constants.MediaGVDInitStatus)
	if err != nil {
		return errors.Wrap(err, "failed to get begin histogram")
	}
	testing.ContextLogf(ctx, "Initial histograms/%s: %v", constants.MediaGVDInitStatus, beginHistogram.Buckets)

	conn, err := cr.NewConn(ctx, server.URL+"/"+videoName)
	if err != nil {
		return errors.Wrap(err, "failed to open video page")
	}
	defer conn.Close()

	// Play a video repeatedly during measurement.
	if err := conn.Exec(ctx, videoElement+".loop=true"); err != nil {
		return errors.Wrap(err, "failed to settle video looping")
	}

	time.Sleep(measurementDuration)
	mtr, err := gatherPerfFunc(ctx, conn)
	if err != nil {
		return errors.Wrap(err, "failed to gather performance values")
	}

	// Stop video.
	if err := conn.Exec(ctx, videoElement+".pause()"); err != nil {
		return errors.Wrap(err, "failed to stop video")
	}

	return recordMetrics(ctx, mtr, ms, cr, beginHistogram, disableHWAcc)
}

// recordMetrics records the measured performance values, mtr, in ms.
func recordMetrics(ctx context.Context, mtr map[string]float64, ms []perfMetrics, cr *chrome.Chrome, beginHistogram *metrics.Histogram, disableHWAcc bool) error {
	hwAccelerationUsed, err := isHWAccelUsed(ctx, cr, beginHistogram)
	if err != nil {
		return errors.Wrap(err, "hw acceleration")
	}
	if hwAccelerationUsed && disableHWAcc {
		return errors.Errorf("hardware acceleration used despite being disabled")
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

// isHWAccelUsed returns whether a video in cr plays with HW acceleration.
func isHWAccelUsed(ctx context.Context, cr *chrome.Chrome, beginHistogram *metrics.Histogram) (bool, error) {
	// There are three valid cases.
	// 1. No histogram is updated. This is the case HW Acceleration is disabled due to Chrome flag, --disable-accelerated-video-decode.
	// 2. Histogram is updated with 15. This is the case Chrome tries to initailize VDA but it fails because the code is not supported on DUT.
	// 3. Histogram is updated with 0. This is the case Chrome sucessfully initializes VDA.

	// err is not nil here if HW Acceleration is disable and then Chrome doesn't try VDA initialization at all.
	// For the case 1, we pass a short time context to WaitForHistogramUpdate to avoid the whole test context (ctx) from reaching deadline.
	histogramDiff, err := metrics.WaitForHistogramUpdate(ctx, cr, constants.MediaGVDInitStatus, beginHistogram, 10*time.Second)
	hwAccelUsed := false
	if err == nil {
		testing.ContextLogf(ctx, "Diff histograms/%s: %v", constants.MediaGVDInitStatus, histogramDiff.Buckets)
		if len(histogramDiff.Buckets) > 1 {
			return false, errors.Wrapf(err, "unexpected histogram difference: %v", histogramDiff)
		}
		testing.ContextLog(ctx, "HistogramDiff Buckets: ", histogramDiff.Buckets)

		// If HW acceleration is used, the sole bucket is {0, 1, X}.
		diff := histogramDiff.Buckets[0]
		if diff.Min == constants.MediaGVDBucket && diff.Max == constants.MediaGVDBucket+1 {
			hwAccelUsed = true
		}
	}
	return hwAccelUsed, nil
}

// savePerfResults saves performance results to "results-chart.json" in outDir
func savePerfResults(ms []perfMetrics, videoDesc, outDir string) error {
	// TODO(hiroh): Remove tastSuffix after removing video_PlaybackPerf in autotest.
	p := &perf.Values{}
	const tastPrefix = "tast_"

	for _, m := range ms {
		if len(m.values) == 0 {
			return errors.Errorf("no performance result for %s: %v", m.desc, ms)
		}
		for pType, value := range m.values {
			perfName := tastPrefix
			if pType == playbackWithHWAcceleration {
				perfName += "hw_" + m.desc
			} else {
				perfName += "sw_" + m.desc
			}
			p.Set(perf.Metric{Name: perfName, Unit: m.unit, Direction: m.dir}, value)
		}
	}
	if err := p.Save(outDir); err != nil {
		return errors.Wrap(err, "failed to save perf data")
	}
	return nil
}

// getDroppedFrames obtains the number of decoded frames and dropped frames by JavaScript,
// and returns the number of dropped frames and the rate of dropped frames.
func getDroppedFrames(ctx context.Context, conn *chrome.Conn) (map[string]float64, error) {

	var decodedFrameCount, droppedFrameCount int64
	if err := conn.Eval(ctx, videoElement+".webkitDecodedFrameCount", &decodedFrameCount); err != nil {
		return nil, errors.Wrap(err, "failed to get # of decoded frames")
	}
	if err := conn.Eval(ctx, videoElement+".webkitDroppedFrameCount", &droppedFrameCount); err != nil {
		return nil, errors.Wrap(err, "failed to get # of dropped frames")
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
