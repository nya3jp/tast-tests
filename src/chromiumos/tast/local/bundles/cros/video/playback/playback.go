// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package playback provides common code for video.Playback* tests.
package playback

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/local/bundles/cros/video/lib/constants"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

const (
	// Identifier for a map of measured values with/without HW Acceleration
	// storing during a test.
	PlaybackWithHWAcceleration    = "playback_with_hw_acceleration"
	PlaybackWithoutHWAcceleration = "playback_without_hw_acceleration"

	// Time to sleep during the test properly.
	MeasurementDurationSec = 30 * time.Second

	// Description for measured values shown in dashboard.
	DroppedFrameDesc        = "video_dropped_frames_"
	DroppedFramePercentDesc = "video_dropped_frames_percent_"
)

// metricsFunc is the type of function to gather metrics during playback.
type metricsFunc = func(context.Context, *chrome.Conn) (map[string]float64, error)

type keyVal struct {
	Desc   string
	Unit   string
	Values map[string]float64
}

// RunTest measures dropped frames and dropped frames percent in playing a video with/without HW Acceleration.
// The measured values are reported to a dashabord. videoDesc is a video description shown on the dashboard.
func RunTest(ctx context.Context, s *testing.State, videoName string, videoDesc string) {
	vl := logging.NewVideoLogger(ctx)
	defer vl.Close(ctx)
	p := &perf.Values{}

	// Measure the number of dropped frames and the rate of dropped frames.
	kvs := []keyVal{
		keyVal{DroppedFrameDesc, "frames", map[string]float64{}},
		keyVal{DroppedFramePercentDesc, "percent", map[string]float64{}},
	}
	if err := measurePlaybackPerf(ctx, s.DataFileSystem(), videoName, getDroppedFrames, kvs); err != nil {
		s.Fatal("Fail in collecting performance values: ", err)
	}
	testing.ContextLogf(ctx, "Measured keyvals (dropped frames and percent): %v", kvs)

	for _, kv := range kvs {
		if len(kv.Values) == 0 {
			s.Fatalf("No performance result for %s: %v", kv.Desc, kvs)
		}
		savePerfResult(p, kv.Values, kv.Desc+videoDesc, kv.Unit)
	}

	// TODO(crbug.com/890733): Add CPU usage.
	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}

// getDroppedFrames obtains the number of decoded frames and dropped frames by JavaScript,
// and returns the number of dropped frames and the rate of dropped frames.
func getDroppedFrames(ctx context.Context, conn *chrome.Conn) (map[string]float64, error) {
	const videoElement = "document.getElementsByTagName('video')[0]"

	var decodedFrameCount, droppedFrameCount int64
	if err := conn.Eval(ctx, videoElement+".webkitDecodedFrameCount", &decodedFrameCount); err != nil {
		testing.ContextLog(ctx, "Failed to get # of decoded frames: ", err)
		return nil, err
	}
	if err := conn.Eval(ctx, videoElement+".webkitDroppedFrameCount", &droppedFrameCount); err != nil {
		testing.ContextLog(ctx, "Failed to get # of dropped frames: ", err)
		return nil, err
	}

	var droppedFramePercent float64
	if decodedFrameCount != 0 {
		droppedFramePercent = 100.0 * float64(droppedFrameCount) / float64(decodedFrameCount)
	} else {
		testing.ContextLogf(ctx, "No frame is decoded and set drop percent to 100.")
		droppedFramePercent = 100.0
	}
	return map[string]float64{
		DroppedFrameDesc:        float64(droppedFrameCount),
		DroppedFramePercentDesc: droppedFramePercent,
	}, nil
}

// playback plays video one time and measures performance values by executing gatherPerfFunc().
// The measured values are recorded in kvs.
// If disableHWAcc is true, Chrome must play video with SW decoder. If false, Chrome play video with HW decoder if it is available.
func playback(ctx context.Context, fileSystem http.FileSystem, videoName string, gatherPerfFunc metricsFunc, kvs []keyVal, disableHWAcc bool) error {
	chromeArgs := []string{logging.ChromeVmoduleFlag()}
	if disableHWAcc {
		chromeArgs = append(chromeArgs, "--disable-accelerated-video-decode")
	}

	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromeArgs))
	if err != nil {
		testing.ContextLog(ctx, "Failed to connect to Chrome: ", err)
		return err
	}
	defer cr.Close(ctx)

	// TODO(hiroh): enforce the idle checks after login.
	server := httptest.NewServer(http.FileServer(fileSystem))
	defer server.Close()

	histogramStart, err := metrics.GetHistogram(ctx, cr, constants.MediaGVDInitStatus)
	if err != nil {
		testing.ContextLog(ctx, "Failed to get histogramStart: ", err)
		return err
	}
	testing.ContextLogf(ctx, "begin histograms/%s: %v", constants.MediaGVDInitStatus, histogramStart.Buckets)

	conn, err := cr.NewConn(ctx, server.URL+"/"+videoName)
	if err != nil {
		testing.ContextLog(ctx, "Failed to open video page: ", err)
		return err
	}
	defer conn.Close()

	time.Sleep(MeasurementDurationSec)
	mtr, err := gatherPerfFunc(ctx, conn)
	if err != nil {
		testing.ContextLog(ctx, "Failed to gather performance values: ", err)
		return err
	}

	return recordMetrics(ctx, mtr, kvs, cr, histogramStart, disableHWAcc)
}

// recordMetrics records the measured performance values, mtr, in kvs.
func recordMetrics(ctx context.Context, mtr map[string]float64, kvs []keyVal, cr *chrome.Chrome, histogramStart *metrics.Histogram, disableHWAcc bool) error {
	// There are three valid cases.
	// 1. No histogram is updated. This is the case HW Acceleration is disabled due to Chrome flag, --disable-accelerated-video-decode.
	// 2. Histogram is updated with 15. This is the case Chrome tries to initailize VDA but it fails because the code is not supported on DUT.
	// 3. Histogram is updated with 0. This is the case Chrome sucessfully initializes VDA.

	// err is not nil here if HW Acceleration is disable and then Chrome doesn't try VDA initialization at all.
	// For the case 1, we pass a short time context to WaitForHistogramUpdate to avoid the whole test context (ctx) from reaching deadline.
	shortTimeContext := context.Background()
	shortTimeContext, _ = context.WithTimeout(shortTimeContext, 12*time.Second)
	histogramDiff, err := metrics.WaitForHistogramUpdate(ctx, cr, constants.MediaGVDInitStatus, histogramStart, 10*time.Second)
	hwAccelerationUsed := false
	if err == nil {
		testing.ContextLogf(ctx, "diff histograms/%s: %v", constants.MediaGVDInitStatus, histogramDiff.Buckets)
		if len(histogramDiff.Buckets) > 1 {
			testing.ContextLog(ctx, "Unexpected histogram difference: ", histogramDiff)
			return fmt.Errorf("Unexpected histogram difference: %v", histogramDiff)
		}
		testing.ContextLog(ctx, "histogramDiff Buckets: ", histogramDiff.Buckets)

		// If HW acceleration is used, the sole bucket is {0, 1, X}.
		diff := histogramDiff.Buckets[0]
		if diff.Min == constants.MediaGVDBucket && diff.Max == constants.MediaGVDBucket+1 {
			hwAccelerationUsed = true
		}
	}

	if hwAccelerationUsed && disableHWAcc {
		testing.ContextLog(ctx, "Hardware acceleration used despite being disabled.")
		return errors.New("Hardware acceleration used despite being disabled.")
	}
	if !hwAccelerationUsed && !disableHWAcc {
		// Software playback performance is not recorded, unless HW Acceleration is disabled.
		return nil
	}

	playbackType := PlaybackWithoutHWAcceleration
	if hwAccelerationUsed {
		playbackType = PlaybackWithHWAcceleration
	}
	for desc, value := range mtr {
		for _, kv := range kvs {
			if kv.Desc == desc {
				kv.Values[playbackType] = value
				break
			}
		}
	}
	return nil
}

// measurePlaybackPerf collects video playback performance by gatherPerfFunc, playing a video with SW decoder and
// also with HW decoder if available.
func measurePlaybackPerf(ctx context.Context, fileSystem http.FileSystem, videoName string, gatherPerfFunc metricsFunc, kvs []keyVal) error {
	// Try Software playback.
	// TODO(hiroh): enable this after a histogram issue is resolved.
	// if err := playback(ctx, fileSystem, video, gatherPerfFunc, kvs, true); err != nil {
	// 	return err
	// }
	// Try without disabling HW Acceleration.
	if err := playback(ctx, fileSystem, videoName, gatherPerfFunc, kvs, false); err != nil {
		return err
	}
	return nil
}

// savePerfResult adds perf results with desc and unit to perf.Values.
func savePerfResult(p *perf.Values, results map[string]float64, desc, unit string) {
	// TODO(hiroh): Remove tastSuffix after removing video_PlaybackPerf in autotest.
	const tastPrefix = "tast_"
	for playbackType, value := range results {
		perfName := tastPrefix
		if playbackType == PlaybackWithHWAcceleration {
			perfName += "hw_" + desc
		} else {
			perfName += "sw_" + desc
		}
		metric := perf.Metric{Name: perfName, Unit: unit, Direction: perf.SmallerIsBetter}
		p.Set(metric, value)
	}
}
