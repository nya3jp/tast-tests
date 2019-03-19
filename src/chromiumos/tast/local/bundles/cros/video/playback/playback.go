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
	"chromiumos/tast/local/bundles/cros/video/lib/audio"
	"chromiumos/tast/local/bundles/cros/video/lib/constants"
	"chromiumos/tast/local/bundles/cros/video/lib/cpu"
	"chromiumos/tast/local/bundles/cros/video/lib/histogram"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

// Identifier for a map of measured values with/without HW Acceleration stored during a test.
type playbackType int

const (
	playbackWithHWAccel playbackType = iota
	playbackWithoutHWAccel
)

type hwAccelState int

const (
	hwAccelDisabled hwAccelState = iota
	hwAccelEnabled
)

type metricDesc string
type metricValue float64

const (
	// Time to sleep while collecting data.
	// The time to wait just after stating to play video so that CPU usage gets stable.
	stabilizationDuration = 5 * time.Second
	// The time to wait after CPU is stable so as to measure solid metric values.
	measurementDuration = 25 * time.Second

	// The maximum time we will wait for the CPU to become idle.
	waitIdleCPUTimeout = 30 * time.Second

	// The CPU is considered idle when average usage is below this threshold.
	// TODO(hiroh): Decrease this threshold if it is hard to get CPU usage below 30% on some devices.
	idleCPUUsagePercent = 30.0

	// Description for measured values shown in dashboard.
	// A video description (e.g. h264_1080p) is appended to them.
	cpuUsageDesc            metricDesc = "video_cpu_usage_"
	droppedFrameDesc        metricDesc = "video_dropped_frames_"
	droppedFramePercentDesc metricDesc = "video_dropped_frames_percent_"

	// Video Element in the page to play a video.
	videoElement = "document.getElementsByTagName('video')[0]"
)

type collectedPerfData map[playbackType]map[metricDesc]metricValue
type metricDef struct {
	desc metricDesc
	unit string
	dir  perf.Direction
}

// metricDefs is a list of metric measured in this test.
var metricDefs = []metricDef{
	{cpuUsageDesc, "percent", perf.SmallerIsBetter},
	{droppedFrameDesc, "frames", perf.SmallerIsBetter},
	{droppedFramePercentDesc, "percent", perf.SmallerIsBetter},
}

// RunTest measures dropped frames, dropped frames percentage and CPU usage percentage in playing a video with/without HW Acceleration.
// The measured values are reported to a dashboard. videoDesc is a video description shown on the dashboard.
func RunTest(ctx context.Context, s *testing.State, videoName, videoDesc string) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	if err := audio.Mute(ctx); err != nil {
		s.Fatal("Failed to mute device: ", err)
	}
	defer audio.Unmute(ctx)

	perfData := collectedPerfData{}
	s.Log("Measuring CPU usage, the number of dropped frames and percent")
	if err := measurePerformance(ctx, s.DataFileSystem(), videoName, perfData); err != nil {
		s.Fatal("Failed to collect CPU usage and dropped frames: ", err)
	}
	s.Log("Measured CPU usage, dropped frames and percent: ", perfData)

	if err := savePerfResults(ctx, perfData, videoDesc, s.OutDir()); err != nil {
		s.Fatal("Failed to save perf data: ", perfData)
	}
}

// measurePerformance collects video playback performance playing a video with SW decoder and
// also with HW decoder if available.
func measurePerformance(ctx context.Context, fileSystem http.FileSystem, videoName string, perfData collectedPerfData) error {
	// Try Software playback.
	if err := measureWithConfig(ctx, fileSystem, videoName, perfData, hwAccelDisabled); err != nil {
		return err
	}

	// Try with Chrome's default settings. Even in this case, HW Acceleration may not be used, since a device doesn't
	// have a capability to play the video with HW acceleration.
	if err := measureWithConfig(ctx, fileSystem, videoName, perfData, hwAccelEnabled); err != nil {
		return err
	}
	return nil
}

// measureWithConfig plays video one time and measures performance values.
// The measured values are recorded in perfData.
func measureWithConfig(ctx context.Context, fileSystem http.FileSystem, videoName string, perfData collectedPerfData, hwState hwAccelState) error {
	chromeArgs := []string{logging.ChromeVmoduleFlag()}
	if hwState == hwAccelDisabled {
		chromeArgs = append(chromeArgs, "--disable-accelerated-video-decode")
	}

	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromeArgs...))
	if err != nil {
		return errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	// Wait until CPU is idle enough. CPU usage can be high immediately after login for various reasons (e.g. animated images on the lock screen).
	if err := cpu.WaitForIdle(ctx, waitIdleCPUTimeout, idleCPUUsagePercent); err != nil {
		return err
	}

	server := httptest.NewServer(http.FileServer(fileSystem))
	defer server.Close()

	initHistogram, err := metrics.GetHistogram(ctx, cr, constants.MediaGVDInitStatus)
	if err != nil {
		return errors.Wrap(err, "failed to get initial histogram")
	}
	testing.ContextLogf(ctx, "Initial %s histogram: %v", constants.MediaGVDInitStatus, initHistogram.Buckets)

	conn, err := cr.NewConn(ctx, server.URL+"/"+videoName)
	if err != nil {
		return errors.Wrap(err, "failed to open video page")
	}
	defer conn.Close()

	// Wait until video element is loaded.
	if err := conn.WaitForExpr(ctx, "document.getElementsByTagName('video').length > 0"); err != nil {
		return errors.Wrap(err, "failed to wait for video element loading")
	}

	// Play a video repeatedly during measurement.
	if err := conn.Exec(ctx, videoElement+".loop=true"); err != nil {
		return errors.Wrap(err, "failed to settle video looping")
	}

	vs, err := measureCPUUsage(ctx, conn)
	if err != nil {
		return errors.Wrap(err, "failed to measure CPU usage")
	}

	vsFrameCount, err := getDroppedFrameCount(ctx, conn)
	if err != nil {
		return errors.Wrap(err, "failed to get dropped frames and percentage")
	}
	for k, v := range vsFrameCount {
		vs[k] = v
	}

	// Stop video.
	if err := conn.Exec(ctx, videoElement+".pause()"); err != nil {
		return errors.Wrap(err, "failed to stop video")
	}

	return recordMetrics(ctx, vs, perfData, cr, initHistogram, hwState)
}

// recordMetrics records the measured performance values in perfData.
func recordMetrics(ctx context.Context, vs map[metricDesc]metricValue, perfData collectedPerfData, cr *chrome.Chrome, initHistogram *metrics.Histogram, hwState hwAccelState) error {
	hwAccelUsed, err := histogram.WasHWAccelUsed(ctx, cr, initHistogram, constants.MediaGVDInitStatus, int64(constants.MediaGVDInitSuccess))
	if err != nil {
		return errors.Wrap(err, "failed to check for hardware acceleration")
	}
	if hwAccelUsed && hwState == hwAccelDisabled {
		return errors.New("hardware acceleration used despite being disabled")
	}
	if !hwAccelUsed && hwState == hwAccelEnabled {
		// Software playback performance is not recorded, unless HW Acceleration is disabled.
		return nil
	}

	pType := playbackWithoutHWAccel
	if hwAccelUsed {
		pType = playbackWithHWAccel
	}

	if perfData[pType] == nil {
		perfData[pType] = map[metricDesc]metricValue{}
	}
	for desc, value := range vs {
		perfData[pType][desc] = value
	}
	return nil
}

// savePerfResults saves performance results in outDir.
func savePerfResults(ctx context.Context, perfData collectedPerfData, videoDesc, outDir string) error {
	// TODO(hiroh): Remove tastPrefix after removing video_PlaybackPerf in autotest.
	p := perf.NewValues()
	const tastPrefix = "tast_"
	for _, pType := range []playbackType{playbackWithHWAccel, playbackWithoutHWAccel} {
		keyval, found := perfData[pType]
		if !found {
			if pType == playbackWithHWAccel {
				testing.ContextLog(ctx, "No HW playback performance result")
				continue
			} else {
				// SW playback performance results should be collected in any cases.
				return errors.Errorf("no SW playback performance result: %v", perfData)
			}
		}
		perfPrefix := tastPrefix
		if pType == playbackWithHWAccel {
			perfPrefix += "hw_"
		} else {
			perfPrefix += "sw_"
		}
		for _, m := range metricDefs {
			val, found := keyval[m.desc]
			perfName := perfPrefix + string(m.desc) + videoDesc
			if !found {
				return errors.Errorf("no performance result for %s: %v", perfName, perfData)
			}
			p.Set(perf.Metric{Name: perfName, Unit: m.unit, Direction: m.dir}, float64(val))
		}
	}
	return p.Save(outDir)
}

// measureCPUUsage obtains CPU usage percentage.
func measureCPUUsage(ctx context.Context, conn *chrome.Conn) (map[metricDesc]metricValue, error) {
	testing.ContextLogf(ctx, "Sleeping %v to wait for CPU usage to stabilize", stabilizationDuration.Round(time.Second))
	select {
	case <-ctx.Done():
		return nil, errors.Wrap(ctx.Err(), "failed waiting for CPU usage to stabilize")
	case <-time.After(stabilizationDuration):
	}

	testing.ContextLogf(ctx, "Sleeping %v to measure CPU usage while playing video", measurementDuration.Round(time.Second))
	cpuUsage, err := cpu.MeasureUsage(ctx, measurementDuration)
	if err != nil {
		return nil, errors.Wrap(err, "failed to measure CPU usage")
	}

	return map[metricDesc]metricValue{
		cpuUsageDesc: metricValue(cpuUsage),
	}, nil
}

// getDroppedFrameCount obtains the number of decoded frames and dropped frames pecentage.
func getDroppedFrameCount(ctx context.Context, conn *chrome.Conn) (map[metricDesc]metricValue, error) {
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
		testing.ContextLog(ctx, "No decoded frames; setting dropped percent to 100")
		droppedFramePercent = 100.0
	}
	return map[metricDesc]metricValue{
		droppedFrameDesc:        metricValue(droppedFrameCount),
		droppedFramePercentDesc: metricValue(droppedFramePercent),
	}, nil
}
