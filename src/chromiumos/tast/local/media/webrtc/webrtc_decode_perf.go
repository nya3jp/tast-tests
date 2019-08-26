// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Provides code for video.WebRTCDecodePerf* tests.

package webrtc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/media/constants"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/media/histogram"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
)

// openWebRTCInternalsPage opens WebRTC internals page and replaces JS
// addLegacyStats() to intercept WebRTC performance metrics, "googMaxDecodeMs"
// and "googDecodeMs".
func openWebRTCInternalsPage(ctx context.Context, cr *chrome.Chrome, addStatsJS string) (*chrome.Conn, error) {
	const url = "chrome://webrtc-internals"
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open "+url)
	}
	toClose := conn
	defer func() {
		if toClose != nil {
			toClose.Close()
		}
	}()

	err = conn.WaitForExpr(ctx, "document.readyState === 'complete'")
	if err != nil {
		return nil, err
	}
	// Switch to legacy mode to reflect webrtc-internals change (crbug.com/803014).
	if err = conn.Exec(ctx, "currentGetStatsMethod = OPTION_GETSTATS_LEGACY"); err != nil {
		return nil, err
	}
	if err = conn.Exec(ctx, addStatsJS); err != nil {
		return nil, err
	}

	toClose = nil
	return conn, nil
}

// getMedian returns the median of the given positive duration.
// If the number of inputs is even, it returns the average of the middle two values.
// If input is empty, returns 0.
func getMedian(s []time.Duration) time.Duration {
	size := len(s)
	if size == 0 {
		return time.Duration(0)
	}
	ss := make([]time.Duration, size)
	copy(ss, s)
	sort.Slice(ss, func(i, j int) bool { return ss[i] < ss[j] })
	if size%2 != 0 {
		return ss[size/2]
	}
	return (ss[size/2] + ss[size/2-1]) / 2
}

// getMax returns the maximum of the given positive duration.
// If input is empty, returns 0.
func getMax(s []time.Duration) time.Duration {
	var max time.Duration
	for _, n := range s {
		if n > max {
			max = n
		}
	}
	return max
}

// MeasureConfig is a set of parameters for measureFunc to reference.
type MeasureConfig struct {
	// NamePrefix is used to prepend on Metric.Name.
	NamePrefix string
	// CPUStabilize is a duration for waiting before measuring CPU usage.
	CPUStabilize time.Duration
	// CPUMeasure is a duration for measuring CPU usage.
	CPUMeasure time.Duration
	// DecodeTimeTimeout is a timeout for measuring frame decode time.
	DecodeTimeTimeout time.Duration
	// DecodeTimeSamples specifies number of frame decode time samples to get.
	// Sample rate: 1 sample per second.
	DecodeTimeSamples int
	// AddStatsJS is a JavaScript used to replace WebRTC internals page's addLegacyStats() function.
	AddStatsJS string
}

// Function signature to measure performance and writes result to perf.Values.
// Note that metric's name prefix is given.
type measureFunc func(context.Context, *chrome.Chrome, *perf.Values, MeasureConfig) error

// measureCPU measures CPU usage for a period of time after a short period for stabilization and writes CPU usage to perf.Values.
func measureCPU(ctx context.Context, cr *chrome.Chrome, p *perf.Values, config MeasureConfig) error {
	testing.ContextLogf(ctx, "Sleeping %v to wait for CPU usage to stabilize", config.CPUStabilize)
	if err := testing.Sleep(ctx, config.CPUStabilize); err != nil {
		return err
	}
	testing.ContextLog(ctx, "Measuring CPU usage for ", config.CPUMeasure)
	cpuUsage, err := cpu.MeasureUsage(ctx, config.CPUMeasure)
	if err != nil {
		return err
	}
	testing.ContextLogf(ctx, "CPU usage: %f%%", cpuUsage)
	p.Set(perf.Metric{
		Name:      config.NamePrefix + "video_cpu_usage",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, cpuUsage)
	return nil
}

// measureDecodeTime measures frames' decode time and recent frames' max decode time via
// chrome://webrtc-internals dashboard. It writes largest max recent decode time and median
// decode time to perf.Values.
func measureDecodeTime(ctx context.Context, cr *chrome.Chrome, p *perf.Values, config MeasureConfig) error {
	conn, err := openWebRTCInternalsPage(ctx, cr, config.AddStatsJS)
	if err != nil {
		return errors.Wrap(err, "failed to open WebRTC-internals page")
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	// Current frame's decode time.
	var decodeTimes []time.Duration
	// Maximum observed frame decode time.
	var maxDecodeTimes []time.Duration
	err = testing.Poll(ctx,
		func(ctx context.Context) error {
			var maxTimesMs []int
			if err := conn.Eval(ctx, "googMaxDecodeMs", &maxTimesMs); err != nil {
				return testing.PollBreak(errors.Wrap(err, "unable to eval googMaxDecodeMs"))

			}
			if len(maxTimesMs) < config.DecodeTimeSamples {
				return errors.New("insufficient samples")
			}
			maxDecodeTimes = make([]time.Duration, len(maxTimesMs))
			for i, ms := range maxTimesMs {
				maxDecodeTimes[i] = time.Duration(ms) * time.Millisecond
			}

			var timesMs []int
			if err := conn.Eval(ctx, "googDecodeMs", &timesMs); err != nil {
				return testing.PollBreak(errors.Wrap(err, "unable to eval googDecodeMs"))
			}
			if len(timesMs) < config.DecodeTimeSamples {
				return errors.New("insufficient samples")
			}
			decodeTimes = make([]time.Duration, len(timesMs))
			for i, ms := range timesMs {
				decodeTimes[i] = time.Duration(ms) * time.Millisecond
			}
			return nil
		}, &testing.PollOptions{Interval: time.Second, Timeout: config.DecodeTimeTimeout})
	if err != nil {
		return err
	}
	if len(maxDecodeTimes) < config.DecodeTimeSamples {
		return errors.Errorf("got %d max decode time sample(s); want %d", len(maxDecodeTimes), config.DecodeTimeSamples)
	}
	if len(decodeTimes) < config.DecodeTimeSamples {
		return errors.Errorf("got %d decode time sample(s); want %d", len(decodeTimes), config.DecodeTimeSamples)
	}
	max := getMax(maxDecodeTimes)
	median := getMedian(decodeTimes)
	testing.ContextLog(ctx, "Max decode times: ", maxDecodeTimes)
	testing.ContextLog(ctx, "Decode times: ", decodeTimes)
	testing.ContextLogf(ctx, "Largest max is %v, median is %v", max, median)
	p.Set(perf.Metric{
		Name:      config.NamePrefix + "decode_time.percentile_0.50",
		Unit:      "milliseconds",
		Direction: perf.SmallerIsBetter},
		float64(median)/float64(time.Millisecond))
	p.Set(perf.Metric{
		Name:      config.NamePrefix + "decode_time.max",
		Unit:      "milliseconds",
		Direction: perf.SmallerIsBetter},
		float64(max)/float64(time.Millisecond))
	return nil
}

// measureCPUDecodeTime measures CPU usage and frame decode time.
func measureCPUDecodeTime(ctx context.Context, cr *chrome.Chrome, p *perf.Values, config MeasureConfig) error {
	if err := measureCPU(ctx, cr, p, config); err != nil {
		return err
	}
	return measureDecodeTime(ctx, cr, p, config)
}

// webRTCDecodePerf starts a Chrome instance (with or without hardware video decoder),
// opens an WebRTC loopback page that repeatedly plays a loopback video stream. After setting up,
// it calls measure() to measure performance metrics and stores to perf.Values.
// webRTCDecodePerf returns true if video decode is hardware accelerated; otherwise, returns false.
// Note: though right now it has only one measure function, i.e. measureCPUDecodeTime, being used. It is kept
// as we will add power measure function later on.
func webRTCDecodePerf(ctx context.Context, s *testing.State, streamFile, loopbackURL string, measure measureFunc,
	disableHWAccel bool, p *perf.Values, config MeasureConfig) (hwAccelUsed bool) {
	chromeArgs := webrtc.ChromeArgsWithCameraInput(streamFile, false)
	if disableHWAccel {
		chromeArgs = append(chromeArgs, "--disable-accelerated-video-decode")
	}
	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromeArgs...))
	if err != nil {
		s.Fatal("Failed to create Chrome: ", err)
	}
	defer cr.Close(ctx)

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed waiting for CPU to become idle: ", err)
	}

	rtcInitHistogram, err := metrics.GetHistogram(ctx, cr, constants.RTCVDInitStatus)
	if err != nil {
		s.Fatalf("Failed to get histogram %s: %v", constants.RTCVDInitStatus, err)
	}

	// Reserve one second for closing tab.
	shortCtx, cancel := ctxutil.Shorten(ctx, time.Second)
	defer cancel()

	// The page repeatedly plays a loopback video stream.
	// To stop it, we defer conn.CloseTarget() to close the tab.
	conn, err := cr.NewConn(shortCtx, loopbackURL)
	if err != nil {
		s.Fatalf("Failed to open %s: %v", loopbackURL, err)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	if err := conn.WaitForExpr(shortCtx, "streamReady"); err != nil {
		s.Fatal("Timed out waiting for stream ready: ", err)
	}
	if err := checkError(shortCtx, conn); err != nil {
		s.Fatal("Error sanity check loopback web page: ", err)
	}

	hwAccelUsed, err = histogram.WasHWAccelUsed(shortCtx, cr, rtcInitHistogram, constants.RTCVDInitStatus, int64(constants.RTCVDInitSuccess))
	s.Log("Use hardware video decoder? ", hwAccelUsed)
	if disableHWAccel && hwAccelUsed {
		s.Fatal("Hardware video decoder unexpectedly used")
	}

	prefix := "sw_"
	if hwAccelUsed {
		prefix = "hw_"
	}
	// TODO(crbug.com/955957): Remove "tast_" prefix after removing video_WebRtcPerf in autotest.
	config.NamePrefix = "tast_" + prefix
	if err := measure(shortCtx, cr, p, config); err != nil {
		s.Fatal("Failed to measure: ", err)
	}

	return hwAccelUsed
}

// RunWebRTCDecodePerf starts a Chrome instance (with or without hardware video decoder),
// opens an WebRTC loopback page that repeatedly plays a loopback video stream
// to measure CPU usage and frame decode time and stores them to perf.
func RunWebRTCDecodePerf(ctx context.Context, s *testing.State, streamName string, config MeasureConfig) {
	// Time reserved for cleanup.
	const cleanupTime = 5 * time.Second

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	loopbackURL := server.URL + "/" + webrtc.LoopbackPage

	s.Log("Setting up for CPU benchmarking")
	cleanUpBenchmark, err := cpu.SetUpBenchmark(ctx)
	if err != nil {
		s.Fatal("Failed to set up CPU benchmark mode: ", err)
	}
	defer cleanUpBenchmark(ctx)

	// Leave a bit of time to tear down benchmark mode.
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	p := perf.NewValues()
	// Try hardware accelerated WebRTC first.
	// If it is hardware accelerated, run without hardware acceleration again.
	streamFilePath := s.DataPath(streamName)
	hwAccelUsed := webRTCDecodePerf(ctx, s, streamFilePath, loopbackURL, measureCPUDecodeTime, false, p, config)
	if hwAccelUsed {
		webRTCDecodePerf(ctx, s, streamFilePath, loopbackURL, measureCPUDecodeTime, true, p, config)
	}
	p.Save(s.OutDir())
}
