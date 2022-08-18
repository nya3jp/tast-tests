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
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/graphics"
	mediacpu "chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
)

const (
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

// RTCTestOptions is used to describe the config used to run RTCPeerConnectionPerf.
type RTCTestOptions struct {
	verifyHWDecoding   VerifyHWAcceleratorMode // Whether to verify or not that hardware decoding is used.
	verifyHWEncoding   VerifyHWAcceleratorMode // Whether to verify or not that hardware encoding is used.
	profile            string                  // Codec to try, e.g. VP8, VP9.
	streamWidth        int                     // Width of video to be sent in the peerconnection.
	streamHeight       int                     // Height of video to be sent in the peerconnection.
	videoGridDimension int                     // Dimension of the grid in which to embed the RTCPeerConnection <video>.
	videoGridFile      string                  // Name of the video file to fill up the grid with, if needed.
	// ScalableVideoCodec "scalabilityMode" identifier.
	// https://www.w3.org/TR/webrtc-svc/#scalabilitymodes
	svc string
	// The number of video streams in simulcast.
	simulcasts int
	// The array each element of which is true iff -th smaller resolution should be a hardware encoder implementation.
	simulcastHWEncs []bool
	// If non-empty, the media to send through the RTC connection will be obtained
	// using getDisplayMedia() and the value corresponds to the surface type. If
	// empty, the media to send will be obtained using getUserMedia().
	displayMediaType DisplayMediaType
}

// MakeTestOptions creates RTCTestoptions for profile, width and height and with
// HW Encoding/Decoding enabled.
func MakeTestOptions(profile string, width, height int) RTCTestOptions {
	return RTCTestOptions{
		verifyHWDecoding:   VerifyHWDecoderUsed,
		verifyHWEncoding:   VerifyHWEncoderUsed,
		profile:            profile,
		streamWidth:        width,
		streamHeight:       height,
		videoGridDimension: 1,
		videoGridFile:      "",
		svc:                "",
		displayMediaType:   "",
	}
}

// MakeSWTestOptions creates RTCTestoptions for profile, width and height and
// with HW Encoding/Decoding disabled.
func MakeSWTestOptions(profile string, width, height int) RTCTestOptions {
	return RTCTestOptions{
		verifyHWDecoding:   VerifySWDecoderUsed,
		verifyHWEncoding:   VerifySWEncoderUsed,
		profile:            profile,
		streamWidth:        width,
		streamHeight:       height,
		videoGridDimension: 1,
		videoGridFile:      "",
		svc:                "",
		displayMediaType:   "",
	}
}

// MakeSimulcastTestOptions creates RTCTestOptions for profile, width and height.
// While a hardware decoder is used, if hwEncs[i] is true then a hardware encoder is used for i-th stream in simulcast.
func MakeSimulcastTestOptions(profile string, width, height int, hwEncs []bool) RTCTestOptions {
	verifyHWEncoding := VerifySWEncoderUsed
	if hwEncs[len(hwEncs)-1] {
		verifyHWEncoding = VerifyHWEncoderUsed
	}
	return RTCTestOptions{
		verifyHWDecoding:   VerifyHWDecoderUsed,
		verifyHWEncoding:   verifyHWEncoding,
		profile:            profile,
		streamWidth:        width,
		streamHeight:       height,
		videoGridDimension: 1,
		videoGridFile:      "",
		svc:                "", // L1T3?
		simulcasts:         len(hwEncs),
		simulcastHWEncs:    hwEncs,
		displayMediaType:   "",
	}
}

// MakeSWEncoderTestOptions creates RTCTestoptions for profile, width and height and
// with HW Decoding and SW Encoding.
func MakeSWEncoderTestOptions(profile string, width, height int) RTCTestOptions {
	return RTCTestOptions{
		verifyHWDecoding:   VerifyHWDecoderUsed,
		verifyHWEncoding:   VerifySWEncoderUsed,
		profile:            profile,
		streamWidth:        width,
		streamHeight:       height,
		videoGridDimension: 1,
		videoGridFile:      "",
		svc:                "",
		displayMediaType:   "",
	}
}

// MakeTestOptionsWithSVC creates RTCTestoptions for profile, width and height with HW
// Decoding enabled and with a layer structure as per svc definition. hwEnc specifies
// enabling a hardware encoder.
func MakeTestOptionsWithSVC(profile string, width, height int, svc string, hwEnc bool) RTCTestOptions {
	verifyHWEncoding := VerifySWEncoderUsed
	if hwEnc {
		verifyHWEncoding = VerifyHWEncoderUsed
	}
	return RTCTestOptions{
		verifyHWDecoding:   VerifyHWDecoderUsed,
		verifyHWEncoding:   verifyHWEncoding,
		profile:            profile,
		streamWidth:        width,
		streamHeight:       height,
		videoGridDimension: 1,
		videoGridFile:      "",
		svc:                svc,
		displayMediaType:   "",
	}
}

// MakeTestOptionsWithVideoGrid creates RTCTestoptions for profile, width and height with HW
// Encoding/Decoding enabled and embedding the RTCPeerConnection in a grid of
// videoGridDimension x videoGridDimension videoGridFiles.
func MakeTestOptionsWithVideoGrid(profile string, width, height, videoGridDimension int, videoGridFile string) RTCTestOptions {
	return RTCTestOptions{
		verifyHWDecoding:   VerifyHWDecoderUsed,
		verifyHWEncoding:   VerifyHWEncoderUsed,
		profile:            profile,
		streamWidth:        width,
		streamHeight:       height,
		videoGridDimension: videoGridDimension,
		videoGridFile:      videoGridFile,
		svc:                "",
		displayMediaType:   "",
	}
}

// MakeCaptureTestOptions creates RTCTestoptions for profile, width, height and displayMediaType
// and with HW Encoding/Decoding enabled.
func MakeCaptureTestOptions(profile string, width, height int, displayMediaType DisplayMediaType) RTCTestOptions {
	return RTCTestOptions{
		verifyHWDecoding:   VerifyHWDecoderUsed,
		verifyHWEncoding:   VerifyHWEncoderUsed,
		profile:            profile,
		streamWidth:        width,
		streamHeight:       height,
		videoGridDimension: 1,
		videoGridFile:      "",
		svc:                "",
		displayMediaType:   displayMediaType,
	}
}

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

// waitForPeerConnectionStabilized waits up to maxStreamWarmUp for one of the
// following:
//
// - If displayMediaType is empty (i.e., we're capturing user media), it waits for the
// transmitted resolution to reach streamWidth x streamHeight.
//
// - If displayMediaType is non-empty (i.e., we're capturing display media), it waits for
// the transmitted width to reach streamWidth or for the transmitted height to
// reach streamHeight.
//
// Returns error on failure or timeout.
func waitForPeerConnectionStabilized(ctx context.Context, conn *chrome.Conn, streamWidth, streamHeight int, displayMediaType DisplayMediaType) error {
	testing.ContextLogf(ctx, "Waiting at most %v seconds for tx resolution rampup, target %dx%d", maxStreamWarmUp, streamWidth, streamHeight)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var txm txMeas
		if err := readRTCReport(ctx, conn, localPeerConnection, "outbound-rtp", &txm); err != nil {
			return testing.PollBreak(err)
		}
		// In the case of screen capture (i.e. the source is non-camera), the original
		// source may not be 16:9, so we can't expect the tx stream dimensions to be
		// exactly equal to streamWidth x streamHeight. However, we can expect either the
		// tx stream width or height to match streamWidth or streamHeight respectively
		// because the screen capture will be scaled to match one of the dimensions and
		// keep the original aspect ratio.
		if displayMediaType == "" && (int(txm.FrameHeight) != streamHeight || int(txm.FrameWidth) != streamWidth) {
			return errors.Errorf("still waiting for tx resolution to reach %dx%d, current: %.0fx%.0f",
				streamWidth, streamHeight, txm.FrameWidth, txm.FrameHeight)
		}
		if displayMediaType != "" && int(txm.FrameHeight) != streamHeight && int(txm.FrameWidth) != streamWidth {
			return errors.Errorf("still waiting for tx width to reach %d or tx height to reach %d, current: %.0fx%.0f",
				streamWidth, streamHeight, txm.FrameWidth, txm.FrameHeight)
		}
		return nil
	}, &testing.PollOptions{Timeout: maxStreamWarmUp, Interval: time.Second}); err != nil {
		return errors.Wrap(err, "timeout waiting for tx resolution to stabilize")
	}
	return nil
}

// measureRTCStats parses the WebRTC Tx and Rx Stats, and stores them into p.
// See https://www.w3.org/TR/webrtc-stats/#stats-dictionaries for more info.
func measureRTCStats(ctx context.Context, conn *chrome.Conn, streamWidth, streamHeight int, displayMediaType DisplayMediaType, p *perf.Values) error {
	if err := waitForPeerConnectionStabilized(ctx, conn, streamWidth, streamHeight, displayMediaType); err != nil {
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

// peerConnectionPerf opens a WebRTC Loopback connection and streams while collecting
// statistics. If videoGridDimension is larger than 1, then the real time <video>
// is plugged into a videoGridDimension x videoGridDimension grid with copies
// of videoURL being played, similar to a mosaic video call.
func peerConnectionPerf(ctx context.Context, cr *chrome.Chrome, loopbackURL, videoURL, outDir string, opts RTCTestOptions, p *perf.Values) error {
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

	if err := conn.Call(ctx, nil, "start", opts.profile, opts.simulcasts, opts.svc, opts.displayMediaType, opts.streamWidth, opts.streamHeight); err != nil {
		return errors.Wrap(err, "establishing connection")
	}

	decImplName, hwDecoderUsed, err := getCodecImplementation(ctx, conn /*decode=*/, true)
	if err != nil {
		return errors.Wrap(err, "failed to get decoder implementation name")
	}
	if opts.verifyHWDecoding == VerifyHWDecoderUsed && !hwDecoderUsed {
		return errors.Errorf("hardware decode accelerator wasn't used, got %s", decImplName)
	}
	if opts.verifyHWDecoding == VerifySWDecoderUsed && hwDecoderUsed {
		return errors.Errorf("software decode wasn't used, got %s", decImplName)
	}

	encImplName, hwEncoderUsed, err := getCodecImplementation(ctx, conn /*decode=*/, false)
	if err != nil {
		return errors.Wrap(err, "failed to get encoder implementation name")
	}
	if opts.simulcasts > 1 {
		if err := checkSimulcastEncImpl(encImplName, opts.simulcastHWEncs); err != nil {
			return err
		}
	} else {
		if opts.verifyHWEncoding == VerifyHWEncoderUsed && !hwEncoderUsed {
			return errors.Errorf("hardware encode accelerator wasn't used, got %s", encImplName)
		}
		if opts.verifyHWEncoding == VerifySWEncoderUsed && hwEncoderUsed {
			return errors.Errorf("software encode wasn't used, got %s", encImplName)
		}
	}

	if opts.videoGridDimension > 1 {
		if err := conn.Call(ctx, nil, "makeVideoGrid", opts.videoGridDimension, videoURL); err != nil {
			return errors.Wrap(err, "javascript error")
		}
	}

	if err := measureRTCStats(shortCtx, conn, opts.streamWidth, opts.streamHeight, opts.displayMediaType, p); err != nil {
		return errors.Wrap(err, "failed to measure")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to test API")
	}

	var gpuErr, cStateErr, cpuErr, batErr error
	var wg sync.WaitGroup
	wg.Add(4)
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
		cpuErr = graphics.MeasureCPUUsageAndPower(ctx, cpuStabilization, cpuMeasuring, p)
	}()
	go func() {
		defer wg.Done()
		batErr = graphics.MeasureSystemPowerConsumption(ctx, tconn, cpuMeasuring, p)
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
	if batErr != nil {
		return errors.Wrap(batErr, "failed to measure system power consumption")
	}

	testing.ContextLogf(ctx, "Metric: %+v", p)
	return nil
}

// RunRTCPeerConnectionPerf starts a Chrome instance (with or without hardware video decoder and encoder),
// opens a WebRTC loopback page and collects performance measures in p.
func RunRTCPeerConnectionPerf(ctx context.Context, cr *chrome.Chrome, fileSystem http.FileSystem, outDir string, opts RTCTestOptions) error {
	// Time reserved for cleanup.
	const cleanupTime = 5 * time.Second

	server := httptest.NewServer(http.FileServer(fileSystem))
	defer server.Close()
	loopbackURL := server.URL + "/" + LoopbackFile

	cleanUpBenchmark, err := mediacpu.SetUpBenchmark(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to set up CPU benchmark")
	}
	defer cleanUpBenchmark(ctx)

	// Leave a bit of time to tear down benchmark mode.
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	var videoGridURL string
	if opts.videoGridDimension > 1 {
		videoGridURL = server.URL + "/" + opts.videoGridFile
	}
	p := perf.NewValues()
	if err := peerConnectionPerf(ctx, cr, loopbackURL, videoGridURL, outDir, opts, p); err != nil {
		return err
	}

	p.Save(outDir)
	return nil
}
