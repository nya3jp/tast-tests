// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package playback provides common code for video.Playback* tests.
package playback

import (
	"context"
	"fmt"
	"math"
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
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/graphics"
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
func RunTest(ctx context.Context, s *testing.State, cs ash.ConnSource, cr *chrome.Chrome, videoName string, decoderType DecoderType, gridSize int, measureRoughness bool) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	if err := crastestclient.Mute(ctx); err != nil {
		s.Fatal("Failed to mute device: ", err)
	}
	defer crastestclient.Unmute(ctx)

	s.Log("Starting playback")
	if err = measurePerformance(ctx, cs, cr, s.DataFileSystem(), videoName, decoderType, gridSize, measureRoughness, s.OutDir()); err != nil {
		s.Fatal("Playback test failed: ", err)
	}
}

// measurePerformance collects video playback performance playing a video with
// either SW or HW decoder.
func measurePerformance(ctx context.Context, cs ash.ConnSource, cr *chrome.Chrome, fileSystem http.FileSystem, videoName string,
	decoderType DecoderType, gridSize int, measureRoughness bool, outDir string) error {
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

	// The page is already rendered with 1 video element by default.
	defaultGridSize := 1
	if gridSize > defaultGridSize {
		if err := conn.Call(ctx, nil, "setGridSize", gridSize); err != nil {
			return errors.Wrap(err, "failed to adjust the grid size")
		}
	}

	// Wait until video element(s) are loaded.
	exprn := fmt.Sprintf("document.getElementsByTagName('video').length == %d", int(math.Max(1.0, float64(gridSize*gridSize))))
	if err := conn.WaitForExpr(ctx, exprn); err != nil {
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
		return errors.Wrap(err, "failed to parse Media DevTools")
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

	var roughness float64
	var gpuErr, cStateErr, cpuErr, fdErr, dramErr, batErr, roughnessErr error
	var wg sync.WaitGroup
	wg.Add(6)
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
		cpuErr = graphics.MeasureCPUUsageAndPower(ctx, stabilizationDuration, measurementDuration, p)
	}()
	go func() {
		defer wg.Done()
		fdErr = graphics.MeasureFdCount(ctx, measurementDuration, p)
	}()
	go func() {
		defer wg.Done()
		dramErr = graphics.MeasureDRAMBandwidth(ctx, measurementDuration, p)
	}()
	go func() {
		defer wg.Done()
		batErr = graphics.MeasureSystemPowerConsumption(ctx, tconn, measurementDuration, p)
	}()
	if measureRoughness {
		wg.Add(1)

		go func() {
			defer wg.Done()
			// If the video sequence is not long enough, roughness won't be provided by
			// Media Devtools and this call will timeout.
			roughness, roughnessErr = devtools.GetVideoPlaybackRoughness(ctx, observer, url)
		}()
	}
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
	if fdErr != nil {
		return errors.Wrap(fdErr, "failed to measure open FD count")
	}
	if dramErr != nil {
		return errors.Wrap(dramErr, "failed to measure DRAM bandwidth consumption")
	}
	if batErr != nil {
		return errors.Wrap(batErr, "failed to measure system power consumption")
	}
	if roughnessErr != nil {
		return errors.Wrap(roughnessErr, "failed to measure playback roughness")
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

	if measureRoughness {
		p.Set(perf.Metric{
			Name:      "roughness",
			Unit:      "percent",
			Direction: perf.SmallerIsBetter,
		}, float64(roughness))
	}

	if err := conn.Eval(ctx, videoElement+".pause()", nil); err != nil {
		return errors.Wrap(err, "failed to stop video")
	}

	p.Save(outDir)
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

	testing.ContextLogf(ctx, "Dropped frames: %d (%f%%)", droppedFrameCount, droppedFramePercent)

	return nil
}
