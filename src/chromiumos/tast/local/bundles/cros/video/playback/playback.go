// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package playback provides common code for video.Playback* tests.
package playback

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/media/devtools"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/testing"
)

// DecoderType represents the different video decoder types.
type DecoderType int

const (
	// Hardware means hardware-accelerated video decoding.
	Hardware DecoderType = iota
	// Software - Any software-based video decoder (e.g. ffmpeg, libvpx).
	Software
	// LibGAV1 is a subtype of the Software above, using an alternative library
	// to play AV1 video for experimentation purposes.
	// TODO(crbug.com/1047051): remove this flag when the experiment is over, and
	// turn DecoderType into a boolean to represent hardware or software decoding.
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
// or without hardware acceleration as per decoderType.
func RunTest(ctx context.Context, s *testing.State, cs ash.ConnSource, cr *chrome.Chrome, videoName string, decoderType DecoderType) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	if err := crastestclient.Mute(ctx); err != nil {
		s.Fatal("Failed to mute device: ", err)
	}
	defer crastestclient.Unmute(ctx)

	testing.ContextLog(ctx, "Measuring performance")
	if err = measurePerformance(ctx, cs, cr, s.DataFileSystem(), videoName, decoderType, s.OutDir()); err != nil {
		s.Fatal("Failed to collect CPU usage and dropped frames: ", err)
	}
}

// measurePerformance collects video playback performance playing a video with
// either SW or HW decoder.
func measurePerformance(ctx context.Context, cs ash.ConnSource, cr *chrome.Chrome, fileSystem http.FileSystem, videoName string,
	decoderType DecoderType, outDir string) error {
	// Wait until CPU is idle enough. CPU usage can be high immediately after login for various reasons (e.g. animated images on the lock screen).
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return err
	}

	server := httptest.NewServer(http.FileServer(fileSystem))
	defer server.Close()

	url := server.URL + "/video.html"
	conn, err := cs.NewConn(ctx, url)
	if err != nil {
		return errors.Wrap(err, "failed to open video page")
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	observer, err := conn.GetMediaPropertiesChangedObserver(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to retrieve DevTools Media messages")
	}

	// Wait until video element is loaded.
	if err := conn.WaitForExpr(ctx, "document.getElementsByTagName('video').length > 0"); err != nil {
		return errors.Wrap(err, "failed to wait for video element loading")
	}

	// TODO(b/183044442): before playing and measuring, we should probably ensure
	// that the UI is in a known state.
	if err := conn.Call(ctx, nil, "playRepeatedly", videoName); err != nil {
		return errors.Wrap(err, "failed to start video")
	}

	// Wait until videoElement has advanced so that chrome:media-internals has
	// time to fill in their fields.
	if err := conn.WaitForExpr(ctx, videoElement+".currentTime > 1"); err != nil {
		return errors.Wrap(err, "failed waiting for video to advance playback")
	}

	isPlatform, decoderName, err := devtools.GetVideoDecoder(ctx, observer, url)
	if err != nil {
		return errors.Wrap(err, "failed to parse Media DevTools: ")
	}
	if decoderType == Hardware && !isPlatform {
		return errors.New("hardware decoding accelerator was expected but wasn't used")
	}
	if decoderType == Software && isPlatform {
		return errors.New("software decoding was expected but wasn't used")
	}
	testing.ContextLog(ctx, "decoderName: ", decoderName)
	if decoderType == LibGAV1 && decoderName != "Gav1VideoDecoder" {
		return errors.Errorf("Expect Gav1VideoDecoder, but used Decoder is %s", decoderName)
	}

	p := perf.NewValues()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to test API")
	}
	const decodeHistogram = "Media.MojoVideoDecoder.Decode"
	initDecodeHistogram, err := metrics.GetHistogram(ctx, tconn, decodeHistogram)
	if err != nil {
		return errors.Wrap(err, "failed to get initial histogram")
	}
	const platformdecodeHistogram = "Media.PlatformVideoDecoding.Decode"
	initPlatformdecodeHistogram, err := metrics.GetHistogram(ctx, tconn, platformdecodeHistogram)
	if err != nil {
		return errors.Wrap(err, "failed to get initial histogram")
	}

	var gpuErr, cStateErr, cpuErr error
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		gpuErr = graphics.MeasureGPUCounters(ctx, measurementDuration, p)
	}()
	go func() {
		defer wg.Done()
		cStateErr = graphics.MeasurePackageCStateCounters(ctx, measurementDuration, p)
	}()
	go func() {
		defer wg.Done()
		cpuErr = measureCPUUsage(ctx, p)
	}()
	wg.Wait()
	if gpuErr != nil {
		return errors.Wrap(gpuErr, "failed to measure GPU counters")
	}
	if cStateErr != nil {
		return errors.Wrap(cStateErr, "failed to measure Package C-State residency")
	}
	if cpuErr != nil {
		return errors.Wrap(cpuErr, "failed to measure CPU/Package power")
	}

	if err := graphics.UpdatePerfMetricFromHistogram(ctx, tconn, decodeHistogram, initDecodeHistogram, p, "video_decode_delay"); err != nil {
		return errors.Wrap(err, "failed to calculate Decode perf metric")
	}
	if err := graphics.UpdatePerfMetricFromHistogram(ctx, tconn, platformdecodeHistogram, initPlatformdecodeHistogram, p, "platform_video_decode_delay"); err != nil {
		return errors.Wrap(err, "failed to calculate Platform Decode perf metric")
	}

	if err := sampleDroppedFrames(ctx, conn, p); err != nil {
		return errors.Wrap(err, "failed to get dropped frames and percentage")
	}

	if err := conn.Eval(ctx, videoElement+".pause()", nil); err != nil {
		return errors.Wrap(err, "failed to stop video")
	}

	p.Save(outDir)
	return nil
}

// measureCPUUsage obtains CPU usage and power consumption if supported.
func measureCPUUsage(ctx context.Context, p *perf.Values) error {
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

// sampleDroppedFrames obtains the number of decoded and dropped frames.
func sampleDroppedFrames(ctx context.Context, conn *chrome.Conn, p *perf.Values) error {
	var decodedFrameCount, droppedFrameCount int64
	if err := conn.Eval(ctx, videoElement+".getVideoPlaybackQuality().totalVideoFrames", &decodedFrameCount); err != nil {
		return errors.Wrap(err, "failed to get number of decoded frames")
	}
	if err := conn.Eval(ctx, videoElement+".getVideoPlaybackQuality().droppedVideoFrames", &droppedFrameCount); err != nil {
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
