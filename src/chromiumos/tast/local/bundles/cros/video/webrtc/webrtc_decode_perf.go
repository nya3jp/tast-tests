// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Provides code for video.WebRTCDecodePerf* tests.

package webrtc

import (
	"context"
	"io/ioutil"
	"math"
	"net/http"
	"net/http/httptest"
	"sort"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/video/lib/constants"
	"chromiumos/tast/local/bundles/cros/video/lib/histogram"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

// openWebRTCInternalsPage opens WebRTC internals page and replaces JS
// addStats() to intercept WebRTC performance metrics, "googMaxDecodeMs"
// and "googDecodeMs".
func openWebRTCInternalsPage(ctx context.Context, cr *chrome.Chrome, addStatsJS string) (*chrome.Conn, error) {
	const url = "chrome://webrtc-internals"
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open "+url)
	}
	err = conn.WaitForExpr(ctx, "document.readyState === 'complete'")
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err = conn.Exec(ctx, addStatsJS); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func getMedian(s []int) float64 {
	sort.Ints(s)
	size := len(s)
	if size%2 != 0 {
		return float64(s[size/2])
	}
	return float64(s[size/2]+s[size/2-1]) / 2
}

func getMax(s []int) int {
	max := math.MinInt32
	for _, n := range s {
		if n > max {
			max = n
		}
	}
	return max
}

// Gathers maximum decode time and median decode time.
func gatherDecodeTime(ctx context.Context, cr *chrome.Chrome, addStatsJS string, max *int, median *float64) error {
	const (
		gatherDecodeTimeout  = time.Minute * 1
		gatherInterval       = time.Millisecond * 500
		numDecodeTimeSamples = 10
	)
	conn, err := openWebRTCInternalsPage(ctx, cr, addStatsJS)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	// Current frame's decode time (in millisecond).
	var decodeTimes []int
	// Maximum observed frame decode time (in millisecond).
	var maxDecodeTimes []int

	if err := testing.Poll(ctx,
		func(ctx context.Context) error {
			if e := conn.Eval(ctx, "googDecodeMs", &decodeTimes); e != nil {
				return testing.PollBreak(errors.Wrap(e, "unable to eval googDecodeMs"))
			}
			if len(decodeTimes) < numDecodeTimeSamples {
				return errors.New("not enough decodeTimes")
			}
			if e := conn.Eval(ctx, "googMaxDecodeMs", &maxDecodeTimes); e != nil {
				return testing.PollBreak(errors.Wrap(e, "unable to eval googMaxDecodeMs"))
			}
			return nil
		}, &testing.PollOptions{Interval: gatherInterval, Timeout: gatherDecodeTimeout}); err != nil {
		return err
	}

	if len(maxDecodeTimes) < numDecodeTimeSamples {
		return errors.Errorf("#maxDecodeTimes %d < %d", len(maxDecodeTimes), numDecodeTimeSamples)
	}
	if len(decodeTimes) < numDecodeTimeSamples {
		return errors.Errorf("#decodeTimes %d < %d", len(decodeTimes), numDecodeTimeSamples)
	}
	*max = getMax(maxDecodeTimes)
	*median = getMedian(decodeTimes)
	testing.ContextLogf(ctx, "Max decode time list=%v", maxDecodeTimes)
	testing.ContextLogf(ctx, "Decode time list=%v", decodeTimes)
	testing.ContextLogf(ctx, "Maximum decode time=%d, median=%f", *max, *median)
	return nil
}

func webRTCPerf(ctx context.Context, s *testing.State, streamFile, loopbackURL string,
	disableHwAccel bool, p *perf.Values) bool {
	chromeArgs := chromeArgsWithCameraInput(streamFile)
	if disableHwAccel {
		chromeArgs = append(chromeArgs, "--disable-accelerated-video-decode")
	}
	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromeArgs...))
	if err != nil {
		s.Fatal("Failed to create Chrome: ", err)
	}
	defer cr.Close(ctx)

	shortCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	rtcInitHistogram, err := metrics.GetHistogram(shortCtx, cr, constants.RTCVDInitStatus)
	if err != nil {
		s.Fatalf("Failed to get histogram %s: %v", constants.RTCVDInitStatus, err)
	}

	conn, err := cr.NewConn(shortCtx, loopbackURL)
	if err != nil {
		s.Fatalf("Failed to open %s: %v", loopbackURL, err)
	}
	defer conn.Close()
	// Close the tab to stop loopback after test.
	defer conn.CloseTarget(shortCtx)

	if err := conn.WaitForExpr(shortCtx, "streamReady"); err != nil {
		s.Fatal("Timed out waiting for stream ready: ", err)
	}

	if err := checkError(shortCtx, conn); err != nil {
		s.Fatal("Error sanity check loopback web page: ", err)
	}

	var maxDecodeTime int
	var medianDecodeTime float64

	addStatsJSPath := s.DataPath(AddStatsJSFile)
	addStatsJS, err := ioutil.ReadFile(addStatsJSPath)
	if err != nil {
		s.Fatalf("Failed to read JS %s for gather decode time: ", addStatsJSPath, err)
	}
	if err := gatherDecodeTime(shortCtx, cr, string(addStatsJS), &maxDecodeTime, &medianDecodeTime); err != nil {
		s.Error("Failed to gather decode time metric: ", err.Error)
	}

	hwAccelUsed, err := histogram.WasHWAccelUsed(shortCtx, cr, rtcInitHistogram, constants.RTCVDInitStatus, int64(constants.RTCVDInitSuccess))
	s.Log("HW accelerator used: ", hwAccelUsed)
	if disableHwAccel && hwAccelUsed {
		s.Fatal("HW decode should not be used")
	}

	hwPrefix := "sw_"
	if hwAccelUsed {
		hwPrefix = "hw_"
	}

	// TODO(crbug.com/918362): Remove "tast_" prefix after removing video_WebRtcPerf in autotest.
	const tastPrefix = "tast_"
	p.Set(perf.Metric{
		Name:      tastPrefix + hwPrefix + "decode_time.max",
		Unit:      "milliseconds",
		Direction: perf.SmallerIsBetter},
		float64(maxDecodeTime))
	p.Set(perf.Metric{
		Name:      tastPrefix + hwPrefix + "decode_time.percentile_0.50",
		Unit:      "milliseconds",
		Direction: perf.SmallerIsBetter},
		medianDecodeTime)

	return hwAccelUsed
}

// RunWebRTCPerf opens a WebRTC loopback page and communicates WebRTC in a fake way.
// The capture stream on WebRTC is specified from streamName.
func RunWebRTCPerf(ctx context.Context, s *testing.State, streamName string) {
	streamFilePath := s.DataPath(streamName)
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	loopbackURL := server.URL + "/" + LoopbackPage

	p := perf.NewValues()
	// Try hardware accelerated WebRTC first.
	// If it is hardware accelerated, run without hardware acceleration again.
	hwAccelUsed := webRTCPerf(ctx, s, streamFilePath, loopbackURL, false, p)
	if hwAccelUsed {
		webRTCPerf(ctx, s, streamFilePath, loopbackURL, true, p)
	}
	p.Save(s.OutDir())
}
