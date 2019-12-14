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
	maxStreamWarmUp = 60 * time.Second

	// Max time to wait before measuring CPU usage.
	cpuStabilization = 10 * time.Second
	// Time to measure CPU usage.
	cpuMeasuring = 30 * time.Second

	// timeSamples specifies number of frame decode time samples to get.
	timeSamples = 10
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

// measureCPU measures CPU usage for a period of time after a short period for stabilization and writes CPU usage to perf.Values.
func measureCPU(ctx context.Context, cr *chrome.Chrome, prefix string, p *perf.Values) error {
	testing.ContextLogf(ctx, "Sleeping %v to wait for CPU usage to stabilize", cpuStabilization)
	if err := testing.Sleep(ctx, cpuStabilization); err != nil {
		return err
	}
	testing.ContextLog(ctx, "Measuring CPU and Power usage for ", cpuMeasuring)
	measurements, err := cpu.MeasureUsage(ctx, cpuMeasuring)
	if err != nil {
		return err
	}
	cpuUsage := measurements["cpu"]
	testing.ContextLogf(ctx, "CPU usage: %f%%", cpuUsage)
	p.Set(perf.Metric{
		Name:      prefix + "cpu_usage",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, cpuUsage)

	if power, ok := measurements["power"]; ok {
		testing.ContextLogf(ctx, "Avg pkg power usage: %fW", power)
		p.Set(perf.Metric{
			Name:      prefix + "pkg_power_usage",
			Unit:      "W",
			Direction: perf.SmallerIsBetter,
		}, power)
	}

	return nil
}

// waitForPeerConnectionStabilized waits up to maxStreamWarmUp for the
// transmitted resolution to reach streamWidth x streamHeight, or returns error.
func waitForPeerConnectionStabilized(ctx context.Context, conn *chrome.Conn, parseTxStatsJS string) error {
	testing.ContextLogf(ctx, "Waiting at most %v seconds for tx resolution rampup, target %dx%d", maxStreamWarmUp, streamWidth, streamHeight)
	var txMeasurement txMeas
	err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := conn.EvalPromise(ctx, parseTxStatsJS, &txMeasurement); err != nil {
			return testing.PollBreak(err)
		}
		if txMeasurement.FrameHeight != streamHeight || txMeasurement.FrameWidth != streamWidth {
			return errors.Errorf("still waiting for tx resolution to reach %dx%d, current: %.0fx%.0f", streamWidth, streamHeight, txMeasurement.FrameWidth, txMeasurement.FrameHeight)
		}
		return nil
	}, &testing.PollOptions{Timeout: maxStreamWarmUp, Interval: time.Second})
	if err != nil {
		return errors.Wrap(err, "timeout waiting for tx resolution to stabilise")
	}
	return nil
}

// measureRTCStats parses the WebRTC Tx and Rx Stats, and stores them into p.
// See https://www.w3.org/TR/webrtc-stats/#stats-dictionaries for more info.
func measureRTCStats(ctx context.Context, s *testing.State, conn *chrome.Conn, prefix string, p *perf.Values) error {
	parseStatsJS :=
		`new Promise(function(resolve, reject) {
			const rtcKeys = [%v];
			const result = {};

			%s.getStats(null).then(stats => {
				if (stats === null) {
					reject("getStats() failed");
					return;
				}
				for (const stat of stats) {
					const report = stat[1]
					for (const statName in report) {
						const index = rtcKeys.indexOf(statName);
						if (index != -1) {
							result[rtcKeys[index]] = report[statName];
						}
					}
				}
				resolve(result);
			});
		})`
	// These keys should coincide in name with the txMeas JSON ones.
	txStats := "'framesPerSecond', 'framesEncoded', 'totalEncodeTime', 'frameWidth', 'frameHeight'"
	parseTxStatsJS := fmt.Sprintf(parseStatsJS, txStats, "localPeerConnection")
	var txMeasurements []txMeas

	// These keys should coincide in name with the rxMeas JSON ones.
	rxStats := "'framesDecoded', 'totalDecodeTime'"
	parseRxStatsJS := fmt.Sprintf(parseStatsJS, rxStats, "remotePeerConnection")
	var rxMeasurements []rxMeas

	if err := waitForPeerConnectionStabilized(ctx, conn, parseTxStatsJS); err != nil {
		return err
	}

	for i := 0; i < timeSamples; i++ {
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
		Name:      prefix + "tx.frames_per_second",
		Unit:      "fps",
		Direction: perf.BiggerIsBetter,
		Multiple:  true,
	}
	for _, txMeasurement := range txMeasurements {
		p.Append(framesPerSecond, txMeasurement.FramesPerSecond)
	}

	encodeTime := perf.Metric{
		Name:      prefix + "tx.encode_time",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}
	for i := 1; i < len(txMeasurements); i++ {
		if txMeasurements[i].FramesEncoded == txMeasurements[i-1].FramesEncoded {
			continue
		}
		averageEncodeTime := (txMeasurements[i].TotalEncodeTime - txMeasurements[i-1].TotalEncodeTime) / (txMeasurements[i].FramesEncoded - txMeasurements[i-1].FramesEncoded) * 1000
		p.Append(encodeTime, averageEncodeTime)
	}

	decodeTime := perf.Metric{
		Name:      prefix + "rx.decode_time",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}
	for i := 1; i < len(rxMeasurements); i++ {
		if rxMeasurements[i].FramesDecoded == rxMeasurements[i-1].FramesDecoded {
			continue
		}
		averageDecodeTime := (rxMeasurements[i].TotalDecodeTime - rxMeasurements[i-1].TotalDecodeTime) / (rxMeasurements[i].FramesDecoded - rxMeasurements[i-1].FramesDecoded) * 1000
		p.Append(decodeTime, averageDecodeTime)
	}
	return nil
}

// decodePerf starts a Chrome instance (with or without hardware video decoder),
// opens a WebRTC loopback page that repeatedly plays a loopback video stream.
func decodePerf(ctx context.Context, s *testing.State, profile, loopbackURL string, enableHWAccel bool, p *perf.Values) {
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

	prefix := "sw."
	hwAccelUsed := checkForCodecImplementation(ctx, s, conn, Decoding) == nil
	if enableHWAccel {
		if !hwAccelUsed {
			s.Fatal("Error: HW accelerator wasn't used")
		}
		prefix = "hw."
	} else {
		if hwAccelUsed {
			s.Fatal("Error: SW accelerator wasn't used")
		}
	}

	if err := measureRTCStats(shortCtx, s, conn, prefix, p); err != nil {
		s.Fatal("Failed to measure: ", err)
	}
	if err := measureCPU(shortCtx, cr, prefix, p); err != nil {
		s.Fatal("Failed to measure: ", err)
	}
	testing.ContextLogf(ctx, "Metric: %+v", p)
}

// RunDecodePerf starts a Chrome instance (with or without hardware video decoder),
// opens a WebRTC loopback page and collects performance measures in p.
func RunDecodePerf(ctx context.Context, s *testing.State, profile string, enableHWAccel bool) {
	// Time reserved for cleanup.
	const cleanupTime = 5 * time.Second

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	loopbackURL := server.URL + "/" + LoopbackFile

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
	decodePerf(ctx, s, profile, loopbackURL, enableHWAccel, p)

	p.Save(s.OutDir())
}
