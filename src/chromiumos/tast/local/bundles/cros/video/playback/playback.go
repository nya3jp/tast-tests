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
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

type playbackType int
type metricsDesc string
type metricsValue float64
type hwAccelState int

const (
	// Identifier for a map of measured values with/without HW Acceleration
	// storing during a test.
	playbackWithHWAcceleration    playbackType = iota
	playbackWithoutHWAcceleration playbackType = iota

	// Time to sleep while collecting data.
	measurementDuration = 30 * time.Second

	// Timeout to get idle CPU.
	waitIdleCPUTimeout = 30 * time.Second

	// The percentage of CPU usage when it is idle.
	idleCPUUsagePercent = 1.0

	// Description for measured values shown in dashboard.
	// A video description (e.g. h264_1080p) is appended to them.
	droppedFrameDesc        metricsDesc = "video_dropped_frames_"
	droppedFramePercentDesc metricsDesc = "video_dropped_frames_percent_"

	// Video Element in the page to play a video.
	videoElement = "document.getElementsByTagName('video')[0]"

	hwAccelEnabled  hwAccelState = iota
	hwAccelDisabled hwAccelState = iota
)

// metricsFunc is the type of function to gather metrics during playback.
type metricsFunc = func(context.Context, *chrome.Conn) (map[metricsDesc]metricsValue, error)

type perfMetrics struct {
	desc   metricsDesc
	unit   string
	dir    perf.Direction
	values map[playbackType]metricsValue
}

// RunTest measures dropped frames and dropped frames percent in playing a video with/without HW Acceleration.
// The measured values are reported to a dashboard. videoDesc is a video description shown on the dashboard.
func RunTest(ctx context.Context, s *testing.State, videoName, videoDesc string) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	// TODO(crbug.com/890733): Check if video is downloaded correctly.
	// The MD5 value of the video must match MD5 string included in videoName.

	// Measure the number of dropped frames and the rate of dropped frames.
	mts := []perfMetrics{
		{droppedFrameDesc, "frames", perf.SmallerIsBetter, map[playbackType]metricsValue{}},
		{droppedFramePercentDesc, "percent", perf.SmallerIsBetter, map[playbackType]metricsValue{}},
	}
	if err := measurePlaybackPerf(ctx, s.DataFileSystem(), videoName, getDroppedFrames, mts); err != nil {
		s.Fatal("Failed to collect performance values: ", err)
	}
	s.Log("Measured metrics (dropped frames and percent): ", mts)

	// TODO(crbug.com/890733): Add CPU usage.
	if err := savePerfResults(mts, videoDesc, s.OutDir()); err != nil {
		s.Fatal("Failed to save perf data: ", mts)
	}
}

// measurePlaybackPerf collects video playback performance by gatherPerfFunc, playing a video with SW decoder and
// also with HW decoder if available.
func measurePlaybackPerf(ctx context.Context, fileSystem http.FileSystem, videoName string, gatherPerfFunc metricsFunc, mts []perfMetrics) error {
	// Try Software playback.
	if err := play(ctx, fileSystem, videoName, gatherPerfFunc, mts, hwAccelDisabled); err != nil {
		return err
	}

	// Try with a default Chrome. Even in this case, HW Acceleration may not be used, since a device doesn't
	// have a capability to play the video with HW acceleration.
	if err := play(ctx, fileSystem, videoName, gatherPerfFunc, mts, hwAccelEnabled); err != nil {
		return err
	}
	return nil
}

// play plays video one time and measures performance values by executing gatherPerfFunc().
// The measured values are recorded in mts.
// If disableHWAcc is true, Chrome must play video with SW decoder. If false, Chrome play video with HW decoder if it is available.
func play(ctx context.Context, fileSystem http.FileSystem, videoName string, gatherPerfFunc metricsFunc, mts []perfMetrics, hwState hwAccelState) error {
	chromeArgs := []string{logging.ChromeVmoduleFlag()}
	if hwState == hwAccelDisabled {
		chromeArgs = append(chromeArgs, "--disable-accelerated-video-decode")
	}

	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromeArgs))
	if err != nil {
		return errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	// TODO(hiroh): Wait until CPU is idle state.

	server := httptest.NewServer(http.FileServer(fileSystem))
	defer server.Close()

	beginHistogram, err := metrics.GetHistogram(ctx, cr, constants.MediaGVDInitStatus)
	if err != nil {
		return errors.Wrap(err, "failed to get begin histogram")
	}
	testing.ContextLogf(ctx, "Initial %s histograms: %v", constants.MediaGVDInitStatus, beginHistogram.Buckets)

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
	vs, err := gatherPerfFunc(ctx, conn)
	if err != nil {
		return errors.Wrap(err, "failed to gather performance values")
	}

	// Stop video.
	if err := conn.Exec(ctx, videoElement+".pause()"); err != nil {
		return errors.Wrap(err, "failed to stop video")
	}

	return recordMetrics(ctx, vs, mts, cr, beginHistogram, hwState)
}

// recordMetrics records the measured performance values, mtr, in mts.
func recordMetrics(ctx context.Context, vs map[metricsDesc]metricsValue, mts []perfMetrics, cr *chrome.Chrome, beginHistogram *metrics.Histogram, hwState hwAccelState) error {
	hwAccelerationUsed, err := isHWAccelUsed(ctx, cr, beginHistogram)
	if err != nil {
		return errors.Wrap(err, "failed to check for hardware acceleration")
	}
	if hwAccelerationUsed && hwState == hwAccelDisabled {
		return errors.Errorf("hardware acceleration used despite being disabled")
	}
	if !hwAccelerationUsed && hwState == hwAccelEnabled {
		// Software playback performance is not recorded, unless HW Acceleration is disabled.
		return nil
	}

	pType := playbackWithoutHWAcceleration
	if hwAccelerationUsed {
		pType = playbackWithHWAcceleration
	}
	for desc, value := range vs {
		for _, m := range mts {
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
	if err != nil {
		// This is the first case; no histogram is updated.
		return false, nil
	}

	testing.ContextLogf(ctx, "Diff %s histograms: %v", constants.MediaGVDInitStatus, histogramDiff.Buckets)
	if len(histogramDiff.Buckets) > 1 {
		return false, errors.Wrapf(err, "unexpected histogram difference: %v", histogramDiff)
	}

	// If HW acceleration is used, the sole bucket is {0, 1, X}.
	diff := histogramDiff.Buckets[0]
	hwAccelUsed := diff.Min == constants.MediaGVDInitSuccess && diff.Max == constants.MediaGVDInitSuccess+1
	return hwAccelUsed, nil
}

// savePerfResults saves performance results in outDir.
func savePerfResults(mts []perfMetrics, videoDesc, outDir string) error {
	// TODO(hiroh): Remove tastSuffix after removing video_PlaybackPerf in autotest.
	p := &perf.Values{}
	const tastPrefix = "tast_"

	for _, m := range mts {
		if len(m.values) == 0 {
			return errors.Errorf("no performance result for %s: %v", m.desc, mts)
		}
		for pType, value := range m.values {
			perfName := tastPrefix
			if pType == playbackWithHWAcceleration {
				perfName += "hw_" + string(m.desc)
			} else {
				perfName += "sw_" + string(m.desc)
			}
			p.Set(perf.Metric{Name: perfName, Unit: m.unit, Direction: m.dir}, float64(value))
		}
	}
	return p.Save(outDir)
}

// getDroppedFrames obtains the number of decoded frames and dropped frames by JavaScript,
// and returns the number of dropped frames and the rate of dropped frames.
func getDroppedFrames(ctx context.Context, conn *chrome.Conn) (map[metricsDesc]metricsValue, error) {

	var decodedFrameCount, droppedFrameCount int64
	if err := conn.Eval(ctx, videoElement+".webkitDecodedFrameCount", &decodedFrameCount); err != nil {
		return nil, errors.Wrap(err, "failed to get number of decoded frames")
	}
	if err := conn.Eval(ctx, videoElement+".webkitDroppedFrameCount", &droppedFrameCount); err != nil {
		return nil, errors.Wrap(err, "failed to get number of dropped frames")
	}

	var droppedFramePercent float64
	if decodedFrameCount != 0 {
		droppedFramePercent = 100.0 * float64(droppedFrameCount) / float64(decodedFrameCount)
	} else {
		testing.ContextLogf(ctx, "No decoed frames; setting dropped percent to 100")
		droppedFramePercent = 100.0
	}
	return map[metricsDesc]metricsValue{
		droppedFrameDesc:        metricsValue(droppedFrameCount),
		droppedFramePercentDesc: metricsValue(droppedFramePercent),
	}, nil
}
