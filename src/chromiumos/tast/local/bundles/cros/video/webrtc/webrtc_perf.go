// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

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
	"chromiumos/tast/local/bundles/cros/video/lib/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

// openWebRTCInternalsPage opens WebRTC internals page and replaces JS
// addStats() to intercept WebRTC performance metrics, "googMaxDecodeMs"
// and "googDecodeMs".
func openWebRTCInternalsPage(ctx context.Context, cr *chrome.Chrome, addStatsJs string) (*chrome.Conn, error) {
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
	if err = conn.Exec(ctx, addStatsJs); err != nil {
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
func gatherDecodeTime(ctx context.Context, cr *chrome.Chrome, addStatsJs string, max *int, median *float64) error {
	const (
		gatherDecodeTimeout  = time.Minute * 1
		gatherInterval       = time.Millisecond * 500
		numDecodeTimeSamples = 10
	)
	conn, err := openWebRTCInternalsPage(ctx, cr, addStatsJs)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	// Maximum decode time of the frames of the last 10 seconds.
	var googMaxDecodeMs []int
	// The decode time of the last frame.
	var googDecodeMs []int

	if err := testing.Poll(ctx,
		func(ctx context.Context) error {
			if e := conn.Eval(ctx, "googDecodeMs", &googDecodeMs); e != nil {
				return testing.PollBreak(errors.Wrap(e, "unable to eval googDecodeMs"))
			}
			if len(googDecodeMs) < numDecodeTimeSamples {
				return errors.New("not enough googDecodeMs")
			}
			if e := conn.Eval(ctx, "googMaxDecodeMs", &googMaxDecodeMs); e != nil {
				return testing.PollBreak(errors.Wrap(e, "unable to eval googMaxDecodeMs"))
			}
			return nil
		}, &testing.PollOptions{Interval: gatherInterval, Timeout: gatherDecodeTimeout}); err != nil {
		return err
	}

	if len(googMaxDecodeMs) < numDecodeTimeSamples {
		return errors.Errorf("#googMaxDecodeMs %d < %d", len(googMaxDecodeMs), numDecodeTimeSamples)
	}
	if len(googDecodeMs) < numDecodeTimeSamples {
		return errors.Errorf("#googDecodeMs %d < %d", len(googDecodeMs), numDecodeTimeSamples)
	}
	*max = getMax(googMaxDecodeMs)
	*median = getMedian(googDecodeMs)
	testing.ContextLogf(ctx, "Max decode time list=%v", googMaxDecodeMs)
	testing.ContextLogf(ctx, "Decode time list=%v", googDecodeMs)
	testing.ContextLogf(ctx, "Maximum decode time=%d, median=%f", *max, *median)
	return nil
}

func webRTCPerf(ctx context.Context, s *testing.State, streamFile, loopbackURL string,
	disableHwAccel bool, p *perf.Values) bool {
	chromeArgs := pre.ChromeArgsWithCameraInput(streamFile)
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

	addStatsJs, err := ioutil.ReadFile(s.DataPath(constants.RTCAddStatsJs))
	if err != nil {
		s.Fatalf("Failed to read JS %s for gather decode time: ", s.DataPath(constants.RTCAddStatsJs), err)
	}
	if err := gatherDecodeTime(shortCtx, cr, string(addStatsJs), &maxDecodeTime, &medianDecodeTime); err != nil {
		s.Error("Failed to gather decode time metric: ", err.Error)
	}

	hwAccelUsed, err := histogram.WasHWAccelUsed(shortCtx, cr, rtcInitHistogram, constants.RTCVDInitStatus, int64(constants.RTCVDInitSuccess))
	if disableHwAccel && hwAccelUsed {
		s.Fatal("HW decode should not be used")
	}
	const tastPrefix = "tast_"
	hwPrefix := "sw_"
	if hwAccelUsed {
		hwPrefix = "hw_"
	}
	s.Log("HW accelerator used: ", hwAccelUsed)

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
	loopbackURL := server.URL + "/" + constants.RTCLoopbackPage

	p := perf.NewValues()
	// Try hardware accelerated WebRTC first.
	hwAccelUsed := webRTCPerf(ctx, s, streamFilePath, loopbackURL, false, p)
	if !hwAccelUsed {
		return
	}
	// If previous run is hardware accelerated, run without hardware acceleration again.
	webRTCPerf(ctx, s, streamFilePath, loopbackURL, true, p)
	p.Save(s.OutDir())
}
