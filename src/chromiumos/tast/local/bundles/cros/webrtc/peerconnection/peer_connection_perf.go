// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Provides code for WebRTC's RTCPeerConnection performance tests.

package peerconnection

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
)

const (
	// MediaStream's width and height utilized around this test.
	streamWidth  = 1280
	streamHeight = 720

	// Before taking any measurements, we need to wait for the RTCPeerConnection
	// to ramp up the CPU adaptation; until then, the transmitted resolution may
	// be smaller than the one expected.
	maxStreamWarmUpSeconds = 60
)

// WebRTC Stats collected on transmission side.
type txMeas struct {
	// From https://www.w3.org/TR/webrtc-stats/#dom-rtcoutboundrtpstreamstats-totalencodetime:
	// "Total number of seconds that has been spent encoding the framesEncoded
	// frames of this stream. The average encode time can be calculated by
	// dividing this value with framesEncoded."
	TotalEncodeTime float64 `json:"totalEncodeTime"`
	FramesEncoded   float64 `json:"framesEncoded"`
	// See https://www.w3.org/TR/webrtc-stats/#vststats-dict* for the following.
	FrameWidth      float64 `json:"frameWidth"`
	FrameHeight     float64 `json:"frameHeight"`
	FramesPerSecond float64 `json:"framesPerSecond"`
}

// WebRTC Stats collected on the receiver side.
type rxMeas struct {
	// From https://w3c.github.io/webrtc-stats/#dom-rtcinboundrtpstreamstats-totaldecodetime
	// "Total number of seconds that have been spent decoding the framesDecoded
	// frames of this stream. The average decode time can be calculated by
	// dividing this value with framesDecoded."
	TotalDecodeTime float64 `json:"totalDecodeTime"`
	FramesDecoded   float64 `json:"framesDecoded"`
}

// openInternalsPage opens WebRTC internals page and replaces JS
// addLegacyStats() to intercept WebRTC performance metrics, "googMaxDecodeMs"
// and "googDecodeMs".
func openInternalsPage(ctx context.Context, cr *chrome.Chrome, addStatsJS string) (*chrome.Conn, error) {
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

// MeasureConfig is a set of parameters for the various measure functions to reference.
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

// measureCPU measures CPU usage for a period of time after a short period for stabilization and writes CPU usage to perf.Values.
func measureCPU(ctx context.Context, cr *chrome.Chrome, p *perf.Values, config MeasureConfig) error {
	testing.ContextLogf(ctx, "Sleeping %v to wait for CPU usage to stabilize", config.CPUStabilize)
	if err := testing.Sleep(ctx, config.CPUStabilize); err != nil {
		return err
	}
	testing.ContextLog(ctx, "Measuring CPU usage for ", config.CPUMeasure)
	cpuUsage, err := cpu.MeasureCPUUsage(ctx, config.CPUMeasure)
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
	conn, err := openInternalsPage(ctx, cr, config.AddStatsJS)
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

// waitForPeerConnectionStabilized waits up to maxStreamWarmUpSeconds for the
// transmitted resolution to reach streamWidth x streamHeight, or returns error.
func waitForPeerConnectionStabilized(ctx context.Context, conn *chrome.Conn, parseTxStatsJS string) error {
	testing.ContextLogf(ctx, "Waiting at most %v seconds for tx resolution rampup, target %dx%d", maxStreamWarmUpSeconds, streamWidth, streamHeight)
	var txMeasurement txMeas
	err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := conn.EvalPromise(ctx, parseTxStatsJS, &txMeasurement); err != nil {
			return errors.Wrap(err, "failed to retrieve and/or parse getStats()")
		}
		if txMeasurement.FrameHeight < streamHeight || txMeasurement.FrameWidth < streamWidth {
			return errors.Errorf("Still waiting for tx resolution to reach %dx%d, current: %.0fx%.0f", streamWidth, streamHeight,
				txMeasurement.FrameWidth, txMeasurement.FrameHeight)
		}
		return nil
	}, &testing.PollOptions{Timeout: maxStreamWarmUpSeconds * time.Second, Interval: time.Second})
	if err != nil {
		return errors.Wrap(err, "Timeout waiting for tx resolution to stabilise")
	}
	return nil
}

// measureRTCStats parses the WebRTC Tx and Rx Stats, and stores them into p.
// See https://www.w3.org/TR/webrtc-stats/#stats-dictionaries for more info.
func measureRTCStats(ctx context.Context, s *testing.State, conn *chrome.Conn, p *perf.Values, config MeasureConfig) error {
	parseStatsJS :=
		`new Promise(function(resolve, reject) {
			const rtcKeys = [%v]
			let result = {}

			%s.getStats(null).then(stats => {
				if (stats == null) {
					reject("getStats() failed");
					return;
				}
				stats.forEach(report => {
					Object.keys(report).forEach(statName => {
						index = rtcKeys.indexOf(statName)
						if (index != -1)
							result[rtcKeys[index]] = report[statName];
					})
				})
				resolve(result);
			});
		})`
	// These keys should coincide in name with the txMeas JSON ones.
	txStats := "'framesPerSecond', 'framesEncoded', 'totalEncodeTime', 'frameWidth', 'frameHeight'"
	parseTxStatsJS := fmt.Sprintf(parseStatsJS, txStats, "localPeerConnection")
	var txMeasurements = []txMeas{}

	// These keys should coincide in name with the rxMeas JSON ones.
	rxStats := "'framesDecoded', 'totalDecodeTime'"
	parseRxStatsJS := fmt.Sprintf(parseStatsJS, rxStats, "remotePeerConnection")
	var rxMeasurements = []rxMeas{}

	if err := waitForPeerConnectionStabilized(ctx, conn, parseTxStatsJS); err != nil {
		return err
	}

	for i := 0; i < config.DecodeTimeSamples; i++ {
		if err := testing.Sleep(ctx, time.Second); err != nil {
			return err
		}

		var txMeasurement txMeas
		if err := conn.EvalPromise(ctx, parseTxStatsJS, &txMeasurement); err != nil {
			return errors.Wrap(err, "failed to retrieve and/or parse getStats()")
		}
		testing.ContextLogf(ctx, "Measurement: %+v", txMeasurement)
		txMeasurements = append(txMeasurements, txMeasurement)

		var rxMeasurement rxMeas
		if err := conn.EvalPromise(ctx, parseRxStatsJS, &rxMeasurement); err != nil {
			return errors.Wrap(err, "failed to retrieve and/or parse getStats()")
		}
		testing.ContextLogf(ctx, "Measurement: %+v", rxMeasurement)
		rxMeasurements = append(rxMeasurements, rxMeasurement)
	}

	framesPerSecond := perf.Metric{
		Name:      config.NamePrefix + "tx.frames_per_second",
		Unit:      "fps",
		Direction: perf.BiggerIsBetter,
		Multiple:  true,
	}
	for _, txMeasurement := range txMeasurements {
		p.Append(framesPerSecond, txMeasurement.FramesPerSecond)
	}

	encodeTime := perf.Metric{
		Name:      config.NamePrefix + "tx.encode_time",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}
	for i := 1; i < len(txMeasurements); i++ {
		averageEncodeTime := (txMeasurements[i].TotalEncodeTime - txMeasurements[i-1].TotalEncodeTime) / (txMeasurements[i].FramesEncoded - txMeasurements[i-1].FramesEncoded) * 1000
		p.Append(encodeTime, averageEncodeTime)
	}

	decodeTime := perf.Metric{
		Name:      config.NamePrefix + "rx.decode_time",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}
	for i := 1; i < len(rxMeasurements); i++ {
		averageDecodeTime := (rxMeasurements[i].TotalDecodeTime - rxMeasurements[i-1].TotalDecodeTime) / (rxMeasurements[i].FramesDecoded - rxMeasurements[i-1].FramesDecoded) * 1000
		p.Append(decodeTime, averageDecodeTime)
	}
	return nil
}

// decodePerf starts a Chrome instance (with or without hardware video decoder),
// opens a WebRTC loopback page that repeatedly plays a loopback video stream.
func decodePerf(ctx context.Context, s *testing.State, profile, loopbackURL string, enableHWAccel bool, p *perf.Values, config MeasureConfig) {
	chromeArgs := webrtc.ChromeArgsWithFakeCameraInput(false)
	if !enableHWAccel {
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

	if err := conn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
		s.Fatal("Timed out waiting for page loading: ", err)
	}

	if err := conn.EvalPromise(ctx, fmt.Sprintf("start(%q, %d, %d)", profile, streamWidth, streamHeight), nil); err != nil {
		s.Fatal("Error establishing connection: ", err)
	}

	prefix := "sw_"
	hwAccelUsed := checkForCodecImplementation(ctx, s, conn, Decoding) == nil
	if enableHWAccel {
		if !hwAccelUsed {
			s.Fatal("Error: HW accelerator wasn't used")
		}
		prefix = "hw_"
	} else {
		if hwAccelUsed {
			s.Fatal("Error: SW accelerator wasn't used")
		}
	}

	// TODO(crbug.com/955957): Remove "tast_" prefix after removing video_WebRtcPerf in autotest.
	config.NamePrefix = "tast_" + prefix
	if err := measureRTCStats(ctx, s, conn, p, config); err != nil {
		s.Fatal("Failed to measure: ", err)
	}
	if err := measureCPU(shortCtx, cr, p, config); err != nil {
		s.Fatal("Failed to measure: ", err)
	}
	if err := measureDecodeTime(shortCtx, cr, p, config); err != nil {
		s.Fatal("Failed to measure: ", err)
	}
	testing.ContextLogf(ctx, "Metric: %+v", p)
}

// RunDecodePerf starts a Chrome instance (with or without hardware video decoder),
// opens a WebRTC loopback page and colects performance measures in p.
func RunDecodePerf(ctx context.Context, s *testing.State, profile string, config MeasureConfig, enableHWAccel bool) {
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
	decodePerf(ctx, s, profile, loopbackURL, enableHWAccel, p, config)

	p.Save(s.OutDir())
}
