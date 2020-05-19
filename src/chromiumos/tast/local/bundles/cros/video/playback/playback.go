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

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/testing"
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
	// LibGAV1 is the video decoder type to play AV1 video with Gav1VideoDecoder.
	LibGAV1
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
	cpuUsageDesc            metricDesc = "cpu_usage"
	powerConsumptionDesc    metricDesc = "power_consumption"
	droppedFrameDesc        metricDesc = "dropped_frames"
	droppedFramePercentDesc metricDesc = "dropped_frames_percent"

	// Video Element in the page to play a video.
	videoElement = "document.getElementsByTagName('video')[0]"
)

type collectedPerfData map[metricDesc]metricValue
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

// RunTest measures a number of performance metrics while playing a video with
// or without HW Acceleration as per enableHWAccel. decoderType specifies
// whether to run the tests against the VDA or VD based video decoder
// implementations.
func RunTest(ctx context.Context, s *testing.State, videoName string, decoderType DecoderType, enableHWAccel bool) {
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
	testing.ContextLog(ctx, "Measuring performance")
	if perfData, err = measurePerformance(ctx, s.DataFileSystem(), s.DataPath("chrome_media_internals_utils.js"), videoName, decoderType, enableHWAccel); err != nil {
		s.Fatal("Failed to collect CPU usage and dropped frames: ", err)
	}
	testing.ContextLog(ctx, "Measurements: ", perfData)

	if err := savePerfResults(ctx, perfData, s.OutDir()); err != nil {
		s.Fatal("Failed to save perf data: ", err)
	}
}

// measurePerformance collects video playback performance playing a video with
// either SW or HW decoder. utilsJSPath is a path of
// chrome_media_internals_utils.js
func measurePerformance(ctx context.Context, fileSystem http.FileSystem, utilsJSPath, videoName string,
	decoderType DecoderType, enableHWAccel bool) (perfData collectedPerfData, err error) {
	var chromeArgs []string
	if !enableHWAccel {
		chromeArgs = append(chromeArgs, "--disable-accelerated-video-decode")
	}

	// TODO(b/141652665): Currently the ChromeosVideoDecoder feature is enabled
	// on x% of devices depending on the branch, so we need to use both enable
	// and disable flags to guarantee correct behavior. Once the feature is
	// always enabled we can remove the "--enable-features" flag here.
	// TODO(crbug.com/1065434): Use precondition.
	switch decoderType {
	case VD:
		chromeArgs = append(chromeArgs, "--enable-features=ChromeosVideoDecoder")
	case VDA:
		chromeArgs = append(chromeArgs, "--disable-features=ChromeosVideoDecoder")
	case LibGAV1:
		chromeArgs = append(chromeArgs, "--enable-features=Gav1VideoDecoder")
	}

	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromeArgs...))
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	// Wait until CPU is idle enough. CPU usage can be high immediately after login for various reasons (e.g. animated images on the lock screen).
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return nil, err
	}

	chromeMediaInternalsConn, err := decode.OpenChromeMediaInternalsPageAndInjectJS(ctx, cr, utilsJSPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open chrome://media-internals")
	}
	defer chromeMediaInternalsConn.Close()
	defer chromeMediaInternalsConn.CloseTarget(ctx)

	server := httptest.NewServer(http.FileServer(fileSystem))
	defer server.Close()

	url := server.URL + "/" + videoName
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open video page")
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	// Wait until video element is loaded.
	if err := conn.WaitForExpr(ctx, "document.getElementsByTagName('video').length > 0"); err != nil {
		return nil, errors.Wrap(err, "failed to wait for video element loading")
	}

	// Play a video repeatedly during measurement.
	if err := conn.Exec(ctx, videoElement+".loop=true"); err != nil {
		return nil, errors.Wrap(err, "failed to settle video looping")
	}

	if perfData, err = measureCPUUsage(ctx, conn); err != nil {
		return nil, errors.Wrap(err, "failed to measure CPU usage")
	}

	vsFrameCount, err := getDroppedFrameCount(ctx, conn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get dropped frames and percentage")
	}
	for k, v := range vsFrameCount {
		perfData[k] = v
	}

	usesPlatformVideoDecoder, err := decode.URLUsesPlatformVideoDecoder(ctx, chromeMediaInternalsConn, url)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse chrome:media-internals: ")
	}
	if enableHWAccel {
		if !usesPlatformVideoDecoder {
			return nil, errors.New("hardware decoding accelerator was expected but wasn't used")
		}
	} else {
		if usesPlatformVideoDecoder {
			return nil, errors.New("software decoding was expected but wasn't used")
		}
	}

	decoderName, err := decode.URLVideoDecoderName(ctx, chromeMediaInternalsConn, url)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse chrome:media-internals: ")
	}
	testing.ContextLog(ctx, "decoderName: ", decoderName)
	if decoderType == LibGAV1 && decoderName != "Gav1VideoDecoder" {
		return nil, errors.Errorf("Expect Gav1VideoDecoder, but used Decoder is %s", decoderName)
	}

	if err := conn.Exec(ctx, videoElement+".pause()"); err != nil {
		return nil, errors.Wrap(err, "failed to stop video")
	}

	return perfData, nil
}

// savePerfResults saves performance results in outDir.
func savePerfResults(ctx context.Context, perfData collectedPerfData, outDir string) error {
	p := perf.NewValues()
	for _, m := range metricDefs {
		val, found := perfData[m.desc]
		perfName := string(m.desc)
		if !found && m.desc != powerConsumptionDesc {
			return errors.Errorf("no performance result for %s: %v", perfName, perfData)
		}
		p.Set(perf.Metric{Name: perfName, Unit: m.unit, Direction: m.dir}, float64(val))
	}
	return p.Save(outDir)
}

// measureCPUUsage obtains CPU usage and power consumption if supported.
func measureCPUUsage(ctx context.Context, conn *chrome.Conn) (map[metricDesc]metricValue, error) {
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
