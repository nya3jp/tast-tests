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
	// VDA - legacy VideoDecodeAccelerator-based accelerated video decoder.
	VDA DecoderType = iota
	// VD - VideoDecoder-based accelerated video decoder.
	VD
	// Software - Any software-based video decoder (e.g. ffmpeg, libvpx).
	Software
	// LibGAV1 - an alternative software library used to play AV1 video.
	LibGAV1
)

const (
	// Time to sleep while collecting data.
	// The time to wait just after stating to play video so that CPU usage gets stable.
	stabilizationDuration = 5 * time.Second
	// The time to wait after CPU is stable so as to measure solid metric values.
	measurementDuration = 25 * time.Second

	// Video Element in the page to play a video.
	videoElement = "document.getElementsByTagName('video')[0]"
)

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

	testing.ContextLog(ctx, "Measuring performance")
	if err = measurePerformance(ctx, s.DataFileSystem(), s.DataPath("chrome_media_internals_utils.js"), videoName, decoderType, enableHWAccel, s.OutDir()); err != nil {
		s.Fatal("Failed to collect CPU usage and dropped frames: ", err)
	}
}

// measurePerformance collects video playback performance playing a video with
// either SW or HW decoder. utilsJSPath is a path of
// chrome_media_internals_utils.js
func measurePerformance(ctx context.Context, fileSystem http.FileSystem, utilsJSPath, videoName string,
	decoderType DecoderType, enableHWAccel bool, outDir string) error {
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
		return errors.Wrap(err, "failed to set video loop")
	}

	// TODO(mcasas): Move measurement collection to after verifying that the decoder
	// used is the intended one. It'll need to wait for video started.
	p := perf.NewValues()
	if err = measureCPUUsage(ctx, conn, p); err != nil {
		return errors.Wrap(err, "failed to measure CPU usage")
	}

	if err := measureDroppedFrames(ctx, conn, p); err != nil {
		return errors.Wrap(err, "failed to get dropped frames and percentage")
	}

	usesPlatformVideoDecoder, err := decode.URLUsesPlatformVideoDecoder(ctx, chromeMediaInternalsConn, url)
	if err != nil {
		return errors.Wrap(err, "failed to parse chrome:media-internals: ")
	}
	if enableHWAccel {
		if !usesPlatformVideoDecoder {
			return errors.New("hardware decoding accelerator was expected but wasn't used")
		}
	} else {
		if usesPlatformVideoDecoder {
			return errors.New("software decoding was expected but wasn't used")
		}
	}

	decoderName, err := decode.URLVideoDecoderName(ctx, chromeMediaInternalsConn, url)
	if err != nil {
		return errors.Wrap(err, "failed to parse chrome:media-internals: ")
	}
	testing.ContextLog(ctx, "decoderName: ", decoderName)
	if decoderType == LibGAV1 && decoderName != "Gav1VideoDecoder" {
		return errors.Errorf("Expect Gav1VideoDecoder, but used Decoder is %s", decoderName)
	}

	if err := conn.Exec(ctx, videoElement+".pause()"); err != nil {
		return errors.Wrap(err, "failed to stop video")
	}

	p.Save(outDir)
	return nil
}

// measureCPUUsage obtains CPU usage and power consumption if supported.
func measureCPUUsage(ctx context.Context, conn *chrome.Conn, p *perf.Values) error {
	testing.ContextLogf(ctx, "Sleeping %v to wait for CPU usage to stabilize", stabilizationDuration)
	if err := testing.Sleep(ctx, stabilizationDuration); err != nil {
		return err
	}
	testing.ContextLog(ctx, "Measuring CPU usage for ", measurementDuration)
	measurements, err := cpu.MeasureUsage(ctx, measurementDuration)
	if err != nil {
		return errors.Wrap(err, "failed to measure CPU usage and power consumption")
	}

	cpuUsage := measurements["cpu"]
	testing.ContextLogf(ctx, "CPU usage: %f%%", cpuUsage)
	p.Set(perf.Metric{
		Name:      "cpu_usage",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, cpuUsage)

	if power, ok := measurements["power"]; ok {
		testing.ContextLogf(ctx, "Avg pkg power usage: %fW", power)
		p.Set(perf.Metric{
			Name:      "pkg_power_usage",
			Unit:      "W",
			Direction: perf.SmallerIsBetter,
		}, power)
	}
	return nil
}

// measureDroppedFrames obtains the number of decoded and dropped frames.
func measureDroppedFrames(ctx context.Context, conn *chrome.Conn, p *perf.Values) error {
	var decodedFrameCount, droppedFrameCount int64
	if err := conn.Eval(ctx, videoElement+".webkitDecodedFrameCount", &decodedFrameCount); err != nil {
		return errors.Wrap(err, "failed to get number of decoded frames")
	}
	if err := conn.Eval(ctx, videoElement+".webkitDroppedFrameCount", &droppedFrameCount); err != nil {
		return errors.Wrap(err, "failed to get number of dropped frames")
	}

	var droppedFramePercent float64
	if decodedFrameCount != 0 {
		droppedFramePercent = 100.0 * float64(droppedFrameCount) / float64(decodedFrameCount)
	} else {
		testing.ContextLog(ctx, "No decoded frames; setting dropped percent to 100")
		droppedFramePercent = 100.0
	}

	p.Set(perf.Metric{
		Name:      "dropped_frames",
		Unit:      "frames",
		Direction: perf.SmallerIsBetter,
	}, float64(droppedFrameCount))
	p.Set(perf.Metric{
		Name:      "dropped_frames_percent",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, droppedFramePercent)

	return nil
}
