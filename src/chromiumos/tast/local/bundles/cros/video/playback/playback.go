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
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/media/logging"
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

// DefaultPerfState specifies whether to record perf metrics of default playback.
type DefaultPerfState int

const (
	// DefaultPerfDisabled disables recording metrics of default playback.
	DefaultPerfDisabled DefaultPerfState = iota
	// DefaultPerfEnabled enables recording metrics of default playback.
	DefaultPerfEnabled
)

// DecoderType represents the different video decoder types.
type DecoderType int

const (
	// VDA is the video decoder type based on the VideoDecodeAccelerator
	// interface. These are set to be deprecrated.
	VDA DecoderType = iota
	// VD is the video decoder type based on the VideoDecoder interface. These
	// will eventually replace the current VDAs.
	VD
)

type metricDesc string
type metricValue float64

const (
	// Time to sleep while collecting data.
	// The time to wait just after stating to play video so that CPU usage gets stable.
	stabilizationDuration = 5 * time.Second
	// The time to wait after CPU is stable so as to measure solid metric values.
	measurementDuration = 25 * time.Second

	// Description for measured values shown in dashboard.
	// A video description (e.g. h264_1080p) is appended to them.
	cpuUsageDesc            metricDesc = "video_cpu_usage"
	powerConsumptionDesc    metricDesc = "video_power_consumption"
	droppedFrameDesc        metricDesc = "video_dropped_frames"
	droppedFramePercentDesc metricDesc = "video_dropped_frames_percent"

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
	{powerConsumptionDesc, "watt", perf.SmallerIsBetter},
	{droppedFrameDesc, "frames", perf.SmallerIsBetter},
	{droppedFramePercentDesc, "percent", perf.SmallerIsBetter},
}

// RunTest measures dropped frames, dropped frames percentage and CPU usage percentage in playing a
// video with/without HW Acceleration. The measured values are reported to a dashboard. If dps is
// DefaultPerfEnabled, an additional set of perf metrics will be recorded for default video playback.
// The default video playback stands for HW-accelerated one if available, otherwise software playback.
// decoderType specifies whether to run the tests against the VDA or VD based video decoder
// implementations.
func RunTest(ctx context.Context, s *testing.State, videoName string, dps DefaultPerfState, decoderType DecoderType) {
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
	s.Log("Measuring performance")
	if err := measurePerformance(ctx, s.DataFileSystem(), s.DataPath("chrome_media_internals_utils.js"), videoName, perfData, decoderType); err != nil {
		s.Fatal("Failed to collect CPU usage and dropped frames: ", err)
	}
	s.Log("Measured CPU usage, number of frames dropped and dropped frame percentage: ", perfData)

	if err := savePerfResults(ctx, perfData, s.OutDir(), dps); err != nil {
		s.Fatal("Failed to save perf data: ", err)
	}
}

// measurePerformance collects video playback performance playing a video with SW decoder and
// also with HW decoder if available.
// utilsJSPath is a path of chrome_media_internals_utils.js
func measurePerformance(ctx context.Context, fileSystem http.FileSystem, utilsJSPath, videoName string,
	perfData collectedPerfData, decoderType DecoderType) error {
	// Try Software playback.
	if err := measureWithConfig(ctx, fileSystem, utilsJSPath, videoName, perfData, hwAccelDisabled, decoderType); err != nil {
		return err
	}

	// Try with Chrome's default settings. Even in this case, HW Acceleration may not be used, since a device doesn't
	// have a capability to play the video with HW acceleration.
	if err := measureWithConfig(ctx, fileSystem, utilsJSPath, videoName, perfData, hwAccelEnabled, decoderType); err != nil {
		return err
	}
	return nil
}

// measureWithConfig plays video one time and measures performance values.
// The measured values are recorded in perfData.
func measureWithConfig(ctx context.Context, fileSystem http.FileSystem, utilsJSPath, videoName string,
	perfData collectedPerfData, hwState hwAccelState, decoderType DecoderType) error {
	var chromeArgs []string
	if hwState == hwAccelDisabled {
		chromeArgs = append(chromeArgs, "--disable-accelerated-video-decode")
	}

	// TODO(b/141652665): Currently the ChromeosVideoDecoder feature is enabled
	// on x% of devices depending on the branch, so we need to use both enable
	// and disable flags to guarantee correct behavior. Once the feature is
	// always enabled we can remove the "--enable-features" flag here.
	if decoderType == VD {
		chromeArgs = append(chromeArgs, "--enable-features=ChromeosVideoDecoder")
	} else {
		chromeArgs = append(chromeArgs, "--disable-features=ChromeosVideoDecoder")
	}

	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromeArgs...))
	if err != nil {
		return errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	// Wait until CPU is idle enough. CPU usage can be high immediately after login for various reasons (e.g. animated images on the lock screen).
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return err
	}

	chromeMediaInternalsConn, err := decode.OpenChromeMediaInternalsPageAndInjectJS(ctx, cr, utilsJSPath)
	if err != nil {
		return errors.Wrap(err, "failed to open chrome://media-internals")
	}
	defer chromeMediaInternalsConn.Close()
	defer chromeMediaInternalsConn.CloseTarget(ctx)

	server := httptest.NewServer(http.FileServer(fileSystem))
	defer server.Close()

	url := server.URL + "/" + videoName
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		return errors.Wrap(err, "failed to open video page")
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	// Wait until video element is loaded.
	if err := conn.WaitForExpr(ctx, "document.getElementsByTagName('video').length > 0"); err != nil {
		return errors.Wrap(err, "failed to wait for video element loading")
	}

	// Play a video repeatedly during measurement.
	if err := conn.Exec(ctx, videoElement+".loop=true"); err != nil {
		return errors.Wrap(err, "failed to settle video looping")
	}

	vs, err := MeasureUsage(ctx, conn)
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

	usesPlatformVideoDecoder, err := decode.URLUsesPlatformVideoDecoder(ctx, chromeMediaInternalsConn, url)
	if err != nil {
		return errors.Wrap(err, "failed to parse chrome:media-internals: ")
	}

	// Stop video.
	if err := conn.Exec(ctx, videoElement+".pause()"); err != nil {
		return errors.Wrap(err, "failed to stop video")
	}

	return recordMetrics(ctx, vs, perfData, cr, usesPlatformVideoDecoder, hwState)
}

// recordMetrics records the measured performance values in perfData.
func recordMetrics(ctx context.Context, vs map[metricDesc]metricValue, perfData collectedPerfData, cr *chrome.Chrome, usesPlatformVideoDecoder bool, hwState hwAccelState) error {
	if usesPlatformVideoDecoder && hwState == hwAccelDisabled {
		return errors.New("hardware acceleration used despite being disabled")
	}
	if !usesPlatformVideoDecoder && hwState == hwAccelEnabled {
		// Software playback performance is not recorded, unless HW Acceleration is disabled.
		return nil
	}

	pType := playbackWithoutHWAccel
	if usesPlatformVideoDecoder {
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
func savePerfResults(ctx context.Context, perfData collectedPerfData, outDir string, dps DefaultPerfState) error {
	p := perf.NewValues()
	defaultPerfRecorded := false
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
		var perfPrefixes []string
		if pType == playbackWithHWAccel {
			perfPrefixes = append(perfPrefixes, "hw_")
		} else {
			perfPrefixes = append(perfPrefixes, "sw_")
		}

		// Default metrics represent perf in default playback, which will be hardware-accelerated playback if it is
		// available on the device; otherwise fallback to software playback.
		if dps == DefaultPerfEnabled && !defaultPerfRecorded {
			perfPrefixes = append(perfPrefixes, "default_")
			defaultPerfRecorded = true
		}
		for _, m := range metricDefs {
			val, found := keyval[m.desc]
			for _, pp := range perfPrefixes {
				perfName := pp + string(m.desc)
				if !found && m.desc != powerConsumptionDesc {
					return errors.Errorf("no performance result for %s: %v", perfName, perfData)
				}
				p.Set(perf.Metric{Name: perfName, Unit: m.unit, Direction: m.dir}, float64(val))
			}
		}
	}
	return p.Save(outDir)
}

// MeasureUsage obtains CPU usage percentage and power consumption if supported.
func MeasureUsage(ctx context.Context, conn *chrome.Conn) (map[metricDesc]metricValue, error) {
	testing.ContextLogf(ctx, "Sleeping %v to wait for CPU usage to stabilize", stabilizationDuration.Round(time.Second))
	if err := testing.Sleep(ctx, stabilizationDuration); err != nil {
		return nil, errors.Wrap(err, "failed waiting for CPU usage to stabilize")
	}

	testing.ContextLogf(ctx, "Sleeping %v to measure CPU usage while playing video", measurementDuration.Round(time.Second))
	measurements, err := cpu.MeasureUsage(ctx, measurementDuration)
	if err != nil {
		return nil, errors.Wrap(err, "failed to measure CPU usage and power consumption")
	}

	// Create metrics map, power is only measured on Intel platforms.
	metrics := map[metricDesc]metricValue{
		cpuUsageDesc: metricValue(measurements["cpu"]),
	}
	if _, ok := measurements["power"]; ok {
		metrics[powerConsumptionDesc] = metricValue(measurements["power"])
	}
	return metrics, nil
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
