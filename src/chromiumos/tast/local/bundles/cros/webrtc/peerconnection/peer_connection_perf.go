// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Provides code for WebRTC's RTCPeerConnection performance tests.

package peerconnection

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/media/cpu"
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
	// Time to measure GPU usage counters.
	gpuMeasuring = 10 * time.Second
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
func measureCPU(ctx context.Context, p *perf.Values) error {
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

// waitForPeerConnectionStabilized waits up to maxStreamWarmUp for the
// transmitted resolution to reach streamWidth x streamHeight, or returns error.
func waitForPeerConnectionStabilized(ctx context.Context, conn *chrome.Conn) error {
	testing.ContextLogf(ctx, "Waiting at most %v seconds for tx resolution rampup, target %dx%d", maxStreamWarmUp, streamWidth, streamHeight)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var txm txMeas
		if err := readRTCReport(ctx, conn, localPeerConnection, "outbound-rtp", &txm); err != nil {
			return testing.PollBreak(err)
		}
		if txm.FrameHeight != streamHeight || txm.FrameWidth != streamWidth {
			return errors.Errorf("still waiting for tx resolution to reach %dx%d, current: %.0fx%.0f", streamWidth, streamHeight, txm.FrameWidth, txm.FrameHeight)
		}
		return nil
	}, &testing.PollOptions{Timeout: maxStreamWarmUp, Interval: time.Second}); err != nil {
		return errors.Wrap(err, "timeout waiting for tx resolution to stabilize")
	}
	return nil
}

// measureRTCStats parses the WebRTC Tx and Rx Stats, and stores them into p.
// See https://www.w3.org/TR/webrtc-stats/#stats-dictionaries for more info.
func measureRTCStats(ctx context.Context, conn *chrome.Conn, p *perf.Values) error {
	if err := waitForPeerConnectionStabilized(ctx, conn); err != nil {
		return err
	}

	var txMeasurements []txMeas
	var rxMeasurements []rxMeas
	for i := 0; i < timeSamples; i++ {
		if err := testing.Sleep(ctx, time.Second); err != nil {
			return err
		}

		var txm txMeas
		if err := readRTCReport(ctx, conn, localPeerConnection, "outbound-rtp", &txm); err != nil {
			return errors.Wrap(err, "failed to retrieve and/or parse getStats()")
		}
		testing.ContextLogf(ctx, "Measurement: %+v", txm)
		txMeasurements = append(txMeasurements, txm)

		var rxm rxMeas
		if err := readRTCReport(ctx, conn, remotePeerConnection, "inbound-rtp", &rxm); err != nil {
			return errors.Wrap(err, "failed to retrieve and/or parse getStats()")
		}
		testing.ContextLogf(ctx, "Measurement: %+v", rxm)
		rxMeasurements = append(rxMeasurements, rxm)

		if rxm.FramesDecoded == 0 {
			// Wait until the first frame is decoded before analyzind its contents.
			// Slow devices might take a substantial amount of time: b/158848650.
			continue
		}

		var isBlackFrame bool
		if err := conn.Call(ctx, &isBlackFrame, "isBlackVideoFrame", streamWidth/8, streamHeight/8); err != nil {
			return errors.Wrap(err, "isBlackVideoFrame() JS failed")
		}
		if isBlackFrame {
			return errors.New("last displayed frame is black")
		}

		var isFrozenFrame bool
		if err := conn.Call(ctx, &isFrozenFrame, "isFrozenVideoFrame", streamWidth/8, streamHeight/8); err != nil {
			return errors.Wrap(err, "isFrozenVideoFrameJS() JS failed")
		}
		if isFrozenFrame {
			return errors.New("last displayed frame is frozen")
		}
	}

	framesPerSecond := perf.Metric{
		Name:      "tx.frames_per_second",
		Unit:      "fps",
		Direction: perf.BiggerIsBetter,
		Multiple:  true,
	}
	for _, txMeasurement := range txMeasurements {
		p.Append(framesPerSecond, txMeasurement.FramesPerSecond)
	}

	encodeTime := perf.Metric{
		Name:      "tx.encode_time",
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
		Name:      "rx.decode_time",
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

	framesEncoded := txMeasurements[len(txMeasurements)-1].FramesEncoded
	if framesEncoded > 0.0 {
		droppedFrames := 100 * (framesEncoded - rxMeasurements[len(rxMeasurements)-1].FramesDecoded) / framesEncoded
		testing.ContextLogf(ctx, "Dropped frame ratio: %f%%", droppedFrames)
		p.Set(perf.Metric{
			Name:      "dropped_frames",
			Unit:      "percent",
			Direction: perf.SmallerIsBetter,
		}, droppedFrames)
	}

	return nil
}

// decodePerf opens a WebRTC Loopback connection and streams while collecting
// statistics. If videoGridDimension is larger than 1, then the real time <video>
// is plugged into a videoGridDimension x videoGridDimension grid with copies
// of videoURL being played, similar to a mosaic video call.
func decodePerf(ctx context.Context, cr *chrome.Chrome, profile, loopbackURL string, enableHWAccel bool, videoGridDimension int, videoURL, outDir string, p *perf.Values) error {
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return errors.Wrap(err, "failed waiting for CPU to become idle")
	}

	// Reserve one second for closing tab.
	shortCtx, cancel := ctxutil.Shorten(ctx, time.Second)
	defer cancel()

	// The page repeatedly plays a loopback video stream.
	// To stop it, we defer conn.CloseTarget() to close the tab.
	conn, err := cr.NewConn(shortCtx, loopbackURL)
	if err != nil {
		return errors.Wrapf(err, "failed to open %s", loopbackURL)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	if err := conn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
		return errors.Wrap(err, "timed out waiting for page loading")
	}

	if videoGridDimension > 1 {
		if err := conn.Call(ctx, nil, "makeVideoGrid", videoGridDimension, videoURL); err != nil {
			return errors.Wrap(err, "javascript error")
		}
	}

	if err := conn.Call(ctx, nil, "start", profile, false, "", streamWidth, streamHeight); err != nil {
		return errors.Wrap(err, "establishing connection")
	}

	hwAccelUsed := checkForCodecImplementation(ctx, conn, VerifyHWDecoderUsed, false /*isSimulcast*/) == nil
	if enableHWAccel {
		if !hwAccelUsed {
			return errors.New("hardware encoding accelerator wasn't used")
		}
	} else {
		if hwAccelUsed {
			return errors.New("software encoding wasn't used")
		}
	}

	if err := measureRTCStats(shortCtx, conn, p); err != nil {
		return errors.Wrap(err, "failed to measure")
	}

	var gpuErr, cStateErr, cpuErr error
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		gpuErr = graphics.MeasureGPUCounters(ctx, gpuMeasuring, p)
	}()
	go func() {
		defer wg.Done()
		cStateErr = graphics.MeasurePackageCStateCounters(ctx, gpuMeasuring, p)
	}()
	go func() {
		defer wg.Done()
		cpuErr = measureCPU(ctx, p)
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

	testing.ContextLogf(ctx, "Metric: %+v", p)
	return nil
}

// RunDecodePerf starts a Chrome instance (with or without hardware video decoder),
// opens a WebRTC loopback page and collects performance measures in p.
func RunDecodePerf(ctx context.Context, cr *chrome.Chrome, fileSystem http.FileSystem, outDir, profile string, enableHWAccel bool, videoGridDimension int, videoGridFilename string) error {
	// Time reserved for cleanup.
	const cleanupTime = 5 * time.Second

	server := httptest.NewServer(http.FileServer(fileSystem))
	defer server.Close()
	loopbackURL := server.URL + "/" + LoopbackFile

	cleanUpBenchmark, err := cpu.SetUpBenchmark(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to set up CPU benchmark")
	}
	defer cleanUpBenchmark(ctx)

	// Leave a bit of time to tear down benchmark mode.
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	var videoGridURL string
	if videoGridDimension > 1 {
		videoGridURL = server.URL + "/" + videoGridFilename
	}
	p := perf.NewValues()
	if err := decodePerf(ctx, cr, profile, loopbackURL, enableHWAccel, videoGridDimension, videoGridURL, outDir, p); err != nil {
		return err
	}

	p.Save(outDir)
	return nil
}
